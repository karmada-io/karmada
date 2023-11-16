/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package descheduler

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/estimator/pb"
	estimatorservice "github.com/karmada-io/karmada/pkg/estimator/service"
	fakekarmadaclient "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func buildBinding(name, ns string, target, status []workv1alpha2.TargetCluster) (*workv1alpha2.ResourceBinding, error) {
	bindingStatus := workv1alpha2.ResourceBindingStatus{}
	for _, cluster := range status {
		statusMap := map[string]interface{}{
			util.ReadyReplicasField: cluster.Replicas,
		}
		raw, err := helper.BuildStatusRawExtension(statusMap)
		if err != nil {
			return nil, err
		}
		bindingStatus.AggregatedStatus = append(bindingStatus.AggregatedStatus, workv1alpha2.AggregatedStatusItem{
			ClusterName: cluster.Name,
			Status:      raw,
			Applied:     true,
		})
	}
	return &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: target,
		},
		Status: bindingStatus,
	}, nil
}

func TestDescheduler_worker(t *testing.T) {
	type args struct {
		target        []workv1alpha2.TargetCluster
		status        []workv1alpha2.TargetCluster
		unschedulable []workv1alpha2.TargetCluster
		name          string
		namespace     string
	}
	tests := []struct {
		name         string
		args         args
		wantResponse []workv1alpha2.TargetCluster
		wantErr      bool
	}{
		{
			name: "1 cluster without unschedulable replicas",
			args: args{
				target: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
				},
				status: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
				},
				unschedulable: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 0,
					},
				},
				name:      "foo",
				namespace: "default",
			},
			wantResponse: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 5,
				},
			},
			wantErr: false,
		},
		{
			name: "1 cluster with 1 unschedulable replicas",
			args: args{
				target: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
				},
				status: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 4,
					},
				},
				unschedulable: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 1,
					},
				},
				name:      "foo",
				namespace: "default",
			},
			wantResponse: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 4,
				},
			},
			wantErr: false,
		},
		{
			name: "1 cluster with all unschedulable replicas",
			args: args{
				target: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
				},
				status: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 0,
					},
				},
				unschedulable: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
				},
				name:      "foo",
				namespace: "default",
			},
			wantResponse: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 0,
				},
			},
			wantErr: false,
		},
		{
			name: "1 cluster with 4 ready replicas and 2 unschedulable replicas",
			args: args{
				target: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
				},
				status: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 4,
					},
				},
				unschedulable: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
				},
				name:      "foo",
				namespace: "default",
			},
			wantResponse: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 4,
				},
			},
			wantErr: false,
		},
		{
			name: "1 cluster with 0 ready replicas and 2 unschedulable replicas",
			args: args{
				target: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
				},
				status: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 0,
					},
				},
				unschedulable: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
				},
				name:      "foo",
				namespace: "default",
			},
			wantResponse: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 3,
				},
			},
			wantErr: false,
		},
		{
			name: "1 cluster with 6 ready replicas and 2 unschedulable replicas",
			args: args{
				target: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
				},
				status: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 6,
					},
				},
				unschedulable: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
				},
				name:      "foo",
				namespace: "default",
			},
			wantResponse: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 5,
				},
			},
			wantErr: false,
		},
		{
			name: "2 cluster without unschedulable replicas",
			args: args{
				target: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
					{
						Name:     "member2",
						Replicas: 10,
					},
				},
				status: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
					{
						Name:     "member2",
						Replicas: 10,
					},
				},
				unschedulable: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 0,
					},
					{
						Name:     "member2",
						Replicas: 0,
					},
				},
				name:      "foo",
				namespace: "default",
			},
			wantResponse: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 5,
				},
				{
					Name:     "member2",
					Replicas: 10,
				},
			},
			wantErr: false,
		},
		{
			name: "2 cluster with 1 unschedulable replica",
			args: args{
				target: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
					{
						Name:     "member2",
						Replicas: 10,
					},
				},
				status: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
					{
						Name:     "member2",
						Replicas: 9,
					},
				},
				unschedulable: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 0,
					},
					{
						Name:     "member2",
						Replicas: 1,
					},
				},
				name:      "foo",
				namespace: "default",
			},
			wantResponse: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 5,
				},
				{
					Name:     "member2",
					Replicas: 9,
				},
			},
			wantErr: false,
		},
		{
			name: "2 cluster with unscheable replicas of every cluster",
			args: args{
				target: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
					{
						Name:     "member2",
						Replicas: 10,
					},
				},
				status: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
					{
						Name:     "member2",
						Replicas: 3,
					},
				},
				unschedulable: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 3,
					},
					{
						Name:     "member2",
						Replicas: 7,
					},
				},
				name:      "foo",
				namespace: "default",
			},
			wantResponse: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 2,
				},
				{
					Name:     "member2",
					Replicas: 3,
				},
			},
			wantErr: false,
		},
		{
			name: "2 cluster with 1 cluster status loss",
			args: args{
				target: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 5,
					},
					{
						Name:     "member2",
						Replicas: 10,
					},
				},
				status: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
				},
				unschedulable: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 3,
					},
					{
						Name:     "member2",
						Replicas: 7,
					},
				},
				name:      "foo",
				namespace: "default",
			},
			wantResponse: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 2,
				},
				{
					Name:     "member2",
					Replicas: 3,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel()

			binding, err := buildBinding(tt.args.name, tt.args.namespace, tt.args.target, tt.args.status)
			if err != nil {
				t.Fatalf("build binding error: %v", err)
			}

			karmadaClient := fakekarmadaclient.NewSimpleClientset(binding)
			factory := informerfactory.NewSharedInformerFactory(karmadaClient, 0)

			desched := &Descheduler{
				KarmadaClient:           karmadaClient,
				informerFactory:         factory,
				bindingInformer:         factory.Work().V1alpha2().ResourceBindings().Informer(),
				bindingLister:           factory.Work().V1alpha2().ResourceBindings().Lister(),
				clusterInformer:         factory.Cluster().V1alpha1().Clusters().Informer(),
				clusterLister:           factory.Cluster().V1alpha1().Clusters().Lister(),
				schedulerEstimatorCache: estimatorclient.NewSchedulerEstimatorCache(),
				unschedulableThreshold:  5 * time.Minute,
				eventRecorder:           record.NewFakeRecorder(1024),
			}
			schedulerEstimator := estimatorclient.NewSchedulerEstimator(desched.schedulerEstimatorCache, 5*time.Second)
			estimatorclient.RegisterSchedulerEstimator(schedulerEstimator)

			for _, c := range tt.args.unschedulable {
				cluster := c
				mockClient := &estimatorservice.MockEstimatorClient{}
				mockResultFn := func(
					ctx context.Context,
					in *pb.UnschedulableReplicasRequest,
					opts ...grpc.CallOption,
				) *pb.UnschedulableReplicasResponse {
					return &pb.UnschedulableReplicasResponse{
						UnschedulableReplicas: cluster.Replicas,
					}
				}
				mockClient.On(
					"GetUnschedulableReplicas",
					mock.MatchedBy(func(context.Context) bool { return true }),
					mock.MatchedBy(func(in *pb.UnschedulableReplicasRequest) bool { return in.Cluster == cluster.Name }),
				).Return(mockResultFn, nil)
				desched.schedulerEstimatorCache.AddCluster(cluster.Name, nil, mockClient)
			}

			desched.informerFactory.Start(ctx.Done())
			if !cache.WaitForCacheSync(ctx.Done(), desched.bindingInformer.HasSynced) {
				t.Fatalf("Failed to wait for cache sync")
			}

			key, err := cache.MetaNamespaceKeyFunc(binding)
			if err != nil {
				t.Fatalf("Failed to get key of binding: %v", err)
			}
			if err := desched.worker(key); (err != nil) != tt.wantErr {
				t.Errorf("worker() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			binding, err = desched.KarmadaClient.WorkV1alpha2().ResourceBindings(tt.args.namespace).Get(ctx, tt.args.name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Failed to get binding: %v", err)
				return
			}
			gotResponse := binding.Spec.Clusters
			if !reflect.DeepEqual(gotResponse, tt.wantResponse) {
				t.Errorf("descheduler worker() gotResponse = %v, want %v", gotResponse, tt.wantResponse)
			}
		})
	}
}
