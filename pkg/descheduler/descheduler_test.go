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
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	"github.com/karmada-io/karmada/cmd/descheduler/app/options"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/estimator/pb"
	estimatorservice "github.com/karmada-io/karmada/pkg/estimator/service"
	fakekarmadaclient "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	worklister "github.com/karmada-io/karmada/pkg/generated/listers/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func TestRecordDescheduleResultEventForResourceBinding(t *testing.T) {
	tests := []struct {
		name           string
		rb             *workv1alpha2.ResourceBinding
		message        string
		err            error
		expectedEvents []string
	}{
		{
			name:           "Nil ResourceBinding",
			rb:             nil,
			message:        "Test message",
			err:            nil,
			expectedEvents: []string{},
		},
		{
			name: "Successful descheduling",
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "test-namespace",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
						Namespace:  "test-namespace",
						UID:        types.UID("test-uid"),
					},
				},
			},
			message: "Descheduling succeeded",
			err:     nil,
			expectedEvents: []string{
				"Normal DescheduleBindingSucceed Descheduling succeeded",
				"Normal DescheduleBindingSucceed Descheduling succeeded",
			},
		},
		{
			name: "Failed descheduling",
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "test-namespace",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
						Namespace:  "test-namespace",
						UID:        types.UID("test-uid"),
					},
				},
			},
			message: "Descheduling failed",
			err:     errors.New("descheduling error"),
			expectedEvents: []string{
				"Warning DescheduleBindingFailed descheduling error",
				"Warning DescheduleBindingFailed descheduling error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeRecorder := record.NewFakeRecorder(10)
			d := &Descheduler{
				eventRecorder: fakeRecorder,
			}

			d.recordDescheduleResultEventForResourceBinding(tt.rb, tt.message, tt.err)

			close(fakeRecorder.Events)
			actualEvents := []string{}
			for event := range fakeRecorder.Events {
				actualEvents = append(actualEvents, event)
			}

			assert.Equal(t, tt.expectedEvents, actualEvents, "Recorded events do not match expected events")
		})
	}
}

func TestUpdateCluster(t *testing.T) {
	tests := []struct {
		name        string
		newObj      interface{}
		expectedAdd bool
	}{
		{
			name: "Valid cluster update",
			newObj: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			expectedAdd: true,
		},
		{
			name:        "Invalid object type",
			newObj:      &corev1.Pod{},
			expectedAdd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorker := &mockAsyncWorker{}
			d := &Descheduler{
				schedulerEstimatorWorker: mockWorker,
			}

			if tt.expectedAdd {
				mockWorker.On("Add", mock.AnythingOfType("string")).Return()
			}

			d.updateCluster(nil, tt.newObj)

			if tt.expectedAdd {
				mockWorker.AssertCalled(t, "Add", "test-cluster")
			} else {
				mockWorker.AssertNotCalled(t, "Add", mock.Anything)
			}
		})
	}
}

func TestDeleteCluster(t *testing.T) {
	tests := []struct {
		name        string
		obj         interface{}
		expectedAdd bool
	}{
		{
			name: "Delete Cluster object",
			obj: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			expectedAdd: true,
		},
		{
			name: "Delete DeletedFinalStateUnknown object",
			obj: cache.DeletedFinalStateUnknown{
				Obj: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
				},
			},
			expectedAdd: true,
		},
		{
			name:        "Invalid object type",
			obj:         &corev1.Pod{},
			expectedAdd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorker := &mockAsyncWorker{}
			d := &Descheduler{
				schedulerEstimatorWorker: mockWorker,
			}

			if tt.expectedAdd {
				mockWorker.On("Add", mock.AnythingOfType("string")).Return()
			}

			d.deleteCluster(tt.obj)

			if tt.expectedAdd {
				mockWorker.AssertCalled(t, "Add", "test-cluster")
			} else {
				mockWorker.AssertNotCalled(t, "Add", mock.Anything)
			}
		})
	}
}

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

func TestNewDescheduler(t *testing.T) {
	karmadaClient := fakekarmadaclient.NewSimpleClientset()
	kubeClient := fake.NewSimpleClientset()
	opts := &options.Options{
		UnschedulableThreshold: metav1.Duration{Duration: 5 * time.Minute},
		DeschedulingInterval:   metav1.Duration{Duration: 1 * time.Minute},
		SchedulerEstimatorPort: 8080,
	}

	descheduler := NewDescheduler(karmadaClient, kubeClient, opts)

	assert.NotNil(t, descheduler)
	assert.Equal(t, karmadaClient, descheduler.KarmadaClient)
	assert.Equal(t, kubeClient, descheduler.KubeClient)
	assert.Equal(t, opts.UnschedulableThreshold.Duration, descheduler.unschedulableThreshold)
	assert.Equal(t, opts.DeschedulingInterval.Duration, descheduler.deschedulingInterval)
	assert.NotNil(t, descheduler.schedulerEstimatorCache)
	assert.NotNil(t, descheduler.schedulerEstimatorWorker)
	assert.NotNil(t, descheduler.deschedulerWorker)
}

func TestRun(t *testing.T) {
	karmadaClient := fakekarmadaclient.NewSimpleClientset()
	kubeClient := fake.NewSimpleClientset()
	opts := &options.Options{
		UnschedulableThreshold: metav1.Duration{Duration: 5 * time.Minute},
		DeschedulingInterval:   metav1.Duration{Duration: 1 * time.Minute},
		SchedulerEstimatorPort: 8080,
	}

	descheduler := NewDescheduler(karmadaClient, kubeClient, opts)

	testCluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
	}
	_, err := karmadaClient.ClusterV1alpha1().Clusters().Create(context.TODO(), testCluster, metav1.CreateOptions{})
	assert.NoError(t, err)

	baseCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	descheduler.informerFactory.Start(baseCtx.Done())
	descheduler.informerFactory.WaitForCacheSync(baseCtx.Done())

	ctx, cancel1 := context.WithTimeout(baseCtx, 2*time.Second)
	defer cancel1()
	go descheduler.Run(ctx)

	time.Sleep(500 * time.Millisecond)

	cluster, err := descheduler.clusterLister.Get("test-cluster")
	assert.NoError(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, "test-cluster", cluster.Name)

	<-ctx.Done()
}

func TestDescheduleOnce(t *testing.T) {
	karmadaClient := fakekarmadaclient.NewSimpleClientset()
	kubeClient := fake.NewSimpleClientset()
	opts := &options.Options{
		UnschedulableThreshold: metav1.Duration{Duration: 5 * time.Minute},
		DeschedulingInterval:   metav1.Duration{Duration: 1 * time.Second},
		SchedulerEstimatorPort: 8080,
	}

	descheduler := NewDescheduler(karmadaClient, kubeClient, opts)

	binding1, err := buildBinding("binding1", "default", []workv1alpha2.TargetCluster{{Name: "cluster1", Replicas: 5}}, []workv1alpha2.TargetCluster{{Name: "cluster1", Replicas: 3}})
	assert.NoError(t, err)
	binding2, err := buildBinding("binding2", "default", []workv1alpha2.TargetCluster{{Name: "cluster2", Replicas: 3}}, []workv1alpha2.TargetCluster{{Name: "cluster2", Replicas: 3}})
	assert.NoError(t, err)

	_, err = karmadaClient.WorkV1alpha2().ResourceBindings("default").Create(context.TODO(), binding1, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = karmadaClient.WorkV1alpha2().ResourceBindings("default").Create(context.TODO(), binding2, metav1.CreateOptions{})
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go descheduler.Run(ctx)

	time.Sleep(1 * time.Second)

	descheduler.descheduleOnce()

	bindings, err := descheduler.bindingLister.List(labels.Everything())
	assert.NoError(t, err)
	assert.Len(t, bindings, 2)
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
					_ context.Context,
					_ *pb.UnschedulableReplicasRequest,
					_ ...grpc.CallOption,
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

func TestDescheduler_workerErrors(t *testing.T) {
	tests := []struct {
		name          string
		key           interface{}
		setupMocks    func(*Descheduler)
		expectedError string
	}{
		{
			name:          "Invalid key type",
			key:           123,
			setupMocks:    func(_ *Descheduler) {},
			expectedError: "failed to deschedule as invalid key: 123",
		},
		{
			name:          "Invalid resource key format",
			key:           "invalid/key/format",
			setupMocks:    func(_ *Descheduler) {},
			expectedError: "invalid resource key: invalid/key/format",
		},
		{
			name: "ResourceBinding not found",
			key:  "default/non-existent-binding",
			setupMocks: func(d *Descheduler) {
				d.bindingLister = &mockBindingLister{
					getErr: apierrors.NewNotFound(schema.GroupResource{Resource: "resourcebindings"}, "non-existent-binding"),
				}
			},
			expectedError: "",
		},
		{
			name: "Error getting ResourceBinding",
			key:  "default/error-binding",
			setupMocks: func(d *Descheduler) {
				d.bindingLister = &mockBindingLister{
					getErr: fmt.Errorf("internal error"),
				}
			},
			expectedError: "get ResourceBinding(default/error-binding) error: internal error",
		},
		{
			name: "ResourceBinding being deleted",
			key:  "default/deleted-binding",
			setupMocks: func(d *Descheduler) {
				d.bindingLister = &mockBindingLister{
					binding: &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "deleted-binding",
							Namespace:         "default",
							DeletionTimestamp: &metav1.Time{Time: time.Now()},
						},
					},
				}
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Descheduler{}
			tt.setupMocks(d)

			err := d.worker(tt.key)

			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedError)
			}
		})
	}
}

// Mock Implementations

type mockAsyncWorker struct {
	mock.Mock
}

func (m *mockAsyncWorker) Add(item interface{}) {
	m.Called(item)
}

func (m *mockAsyncWorker) AddAfter(item interface{}, duration time.Duration) {
	m.Called(item, duration)
}

func (m *mockAsyncWorker) Run(_ context.Context, _ int) {}

func (m *mockAsyncWorker) Enqueue(obj interface{}) {
	m.Called(obj)
}

func (m *mockAsyncWorker) EnqueueAfter(obj interface{}, duration time.Duration) {
	m.Called(obj, duration)
}

type mockBindingLister struct {
	binding *workv1alpha2.ResourceBinding
	getErr  error
}

func (m *mockBindingLister) List(_ labels.Selector) (ret []*workv1alpha2.ResourceBinding, err error) {
	return nil, nil
}

func (m *mockBindingLister) ResourceBindings(_ string) worklister.ResourceBindingNamespaceLister {
	return &mockBindingNamespaceLister{
		binding: m.binding,
		getErr:  m.getErr,
	}
}

type mockBindingNamespaceLister struct {
	binding *workv1alpha2.ResourceBinding
	getErr  error
}

func (m *mockBindingNamespaceLister) List(_ labels.Selector) (ret []*workv1alpha2.ResourceBinding, err error) {
	return nil, nil
}

func (m *mockBindingNamespaceLister) Get(_ string) (*workv1alpha2.ResourceBinding, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.binding, nil
}
