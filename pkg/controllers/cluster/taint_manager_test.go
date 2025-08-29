/*
Copyright 2024 The Karmada Authors.

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

package cluster

import (
	"context"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
)

func newNoExecuteTaintManager() *NoExecuteTaintManager {
	rbIndexerFunc := func(obj client.Object) []string {
		rb, ok := obj.(*workv1alpha2.ResourceBinding)
		if !ok {
			return nil
		}
		return util.GetBindingClusterNames(&rb.Spec)
	}

	crbIndexerFunc := func(obj client.Object) []string {
		crb, ok := obj.(*workv1alpha2.ClusterResourceBinding)
		if !ok {
			return nil
		}
		return util.GetBindingClusterNames(&crb.Spec)
	}

	mgr := &NoExecuteTaintManager{
		Client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).
			WithIndex(&workv1alpha2.ResourceBinding{}, indexregistry.ResourceBindingIndexByFieldCluster, rbIndexerFunc).
			WithIndex(&workv1alpha2.ClusterResourceBinding{}, indexregistry.ClusterResourceBindingIndexByFieldCluster, crbIndexerFunc).Build(),
		EnableNoExecuteTaintEviction:    true,
		NoExecuteTaintEvictionPurgeMode: "Gracefully",
	}
	bindingEvictionWorkerOptions := util.Options{
		Name:          "binding-eviction",
		KeyFunc:       nil,
		ReconcileFunc: mgr.syncBindingEviction,
	}
	mgr.bindingEvictionWorker = util.NewAsyncWorker(bindingEvictionWorkerOptions)

	clusterBindingEvictionWorkerOptions := util.Options{
		Name:          "cluster-binding-eviction",
		KeyFunc:       nil,
		ReconcileFunc: mgr.syncClusterBindingEviction,
	}
	mgr.clusterBindingEvictionWorker = util.NewAsyncWorker(clusterBindingEvictionWorkerOptions)
	return mgr
}

func TestNoExecuteTaintManager_Reconcile(t *testing.T) {
	tests := []struct {
		name    string
		cluster *clusterv1alpha1.Cluster
		want    reconcile.Result
		wantErr bool
	}{
		{
			name: "no taints",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{},
			},
			want:    reconcile.Result{},
			wantErr: false,
		},
		{
			name: "have taints",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:    "test-taint",
							Value:  "test-value",
							Effect: corev1.TaintEffectNoExecute,
						},
					},
				},
			},
			want:    reconcile.Result{},
			wantErr: false,
		},
		{
			name:    "cluster not found",
			want:    reconcile.Result{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newNoExecuteTaintManager()
			if err := tc.Client.Create(context.Background(), &workv1alpha2.ResourceBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ResourceBinding",
					APIVersion: "work.karmada.io/v1alpha2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "test-cluster",
							Replicas: 1,
						},
					},
				},
			}, &client.CreateOptions{}); err != nil {
				t.Fatalf("failed to create rb, %v", err)
			}

			if err := tc.Client.Create(context.Background(), &workv1alpha2.ClusterResourceBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterResourceBinding",
					APIVersion: "work.karmada.io/v1alpha2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-crb",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "test-cluster",
							Replicas: 1,
						},
					},
				},
			}, &client.CreateOptions{}); err != nil {
				t.Fatalf("failed to create crb, %v", err)
			}

			if tt.cluster != nil {
				if err := tc.Client.Create(context.Background(), tt.cluster, &client.CreateOptions{}); err != nil {
					t.Fatal(err)
					return
				}
			}

			req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-cluster"}}
			got, err := tc.Reconcile(context.Background(), req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NoExecuteTaintManager.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NoExecuteTaintManager.Reconcile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoExecuteTaintManager_syncBindingEviction(t *testing.T) {
	replica := int32(1)
	tests := []struct {
		name    string
		rb      *workv1alpha2.ResourceBinding
		cluster *clusterv1alpha1.Cluster
		wrb     *workv1alpha2.ResourceBinding
		wantErr bool
	}{
		{
			name: "rb without tolerations",
			rb: &workv1alpha2.ResourceBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ResourceBinding",
					APIVersion: "work.karmada.io/v1alpha2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-rb",
					Namespace:   "default",
					Annotations: map[string]string{"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1","member2"]},"clusterTolerations":[{"key":"cluster.karmada.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}],"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted","weightPreference":{"staticWeightList":[{"targetCluster":{"clusterNames":["member1"]},"weight":1},{"targetCluster":{"clusterNames":["member2"]},"weight":1}]}}}`},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "test-cluster",
							Replicas: 1,
						},
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:    "cluster.karmada.io/not-ready",
							Effect: corev1.TaintEffectNoExecute,
						},
					},
				},
			},
			wrb: &workv1alpha2.ResourceBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ResourceBinding",
					APIVersion: "work.karmada.io/v1alpha2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-rb",
					Namespace:   "default",
					Annotations: map[string]string{"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1","member2"]},"clusterTolerations":[{"key":"cluster.karmada.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}],"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted","weightPreference":{"staticWeightList":[{"targetCluster":{"clusterNames":["member1"]},"weight":1},{"targetCluster":{"clusterNames":["member2"]},"weight":1}]}}}`},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster: "test-cluster",
							PurgeMode:   policyv1alpha1.PurgeModeGracefully,
							Replicas:    &replica,
							Reason:      workv1alpha2.EvictionReasonTaintUntolerated,
							Producer:    workv1alpha2.EvictionProducerTaintManager,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "rb with tolerations",
			rb: &workv1alpha2.ResourceBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ResourceBinding",
					APIVersion: "work.karmada.io/v1alpha2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-rb",
					Namespace:   "default",
					Annotations: map[string]string{"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1","member2"]},"clusterTolerations":[{"key":"cluster.karmada.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":30},{"key":"cluster.karmada.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}],"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted","weightPreference":{"staticWeightList":[{"targetCluster":{"clusterNames":["member1"]},"weight":1},{"targetCluster":{"clusterNames":["member2"]},"weight":1}]}}}`},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "test-cluster",
							Replicas: 1,
						},
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:    "cluster.karmada.io/not-ready",
							Effect: corev1.TaintEffectNoExecute,
						},
					},
				},
			},
			wrb: &workv1alpha2.ResourceBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ResourceBinding",
					APIVersion: "work.karmada.io/v1alpha2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-rb",
					Namespace:   "default",
					Annotations: map[string]string{"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1","member2"]},"clusterTolerations":[{"key":"cluster.karmada.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}],"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted","weightPreference":{"staticWeightList":[{"targetCluster":{"clusterNames":["member1"]},"weight":1},{"targetCluster":{"clusterNames":["member2"]},"weight":1}]}}}`},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "test-cluster",
							Replicas: 1,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "rb not exist",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newNoExecuteTaintManager()
			if tt.rb != nil {
				if err := tc.Create(context.Background(), tt.rb, &client.CreateOptions{}); err != nil {
					t.Fatalf("failed to create rb: %v", err)
				}
			}

			if tt.cluster != nil {
				if err := tc.Create(context.Background(), tt.cluster, &client.CreateOptions{}); err != nil {
					t.Fatalf("failed to create cluster: %v", err)
				}
			}

			key := keys.FederatedKey{
				Cluster: "test-cluster",
				ClusterWideKey: keys.ClusterWideKey{
					Kind:      "ResourceBinding",
					Name:      "test-rb",
					Namespace: "default",
				},
			}
			if err := tc.syncBindingEviction(key); (err != nil) != tt.wantErr {
				t.Errorf("NoExecuteTaintManager.syncBindingEviction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wrb != nil {
				gRb := &workv1alpha2.ResourceBinding{}
				if err := tc.Get(context.Background(), types.NamespacedName{Name: tt.wrb.Name, Namespace: tt.wrb.Namespace}, gRb, &client.GetOptions{}); err != nil {
					t.Fatalf("failed to get rb, error %v", err)
				}

				if !reflect.DeepEqual(tt.wrb.Spec, gRb.Spec) {
					t.Errorf("ResourceBinding get %+v, want %+v", gRb.Spec, tt.wrb.Spec)
				}
			}
		})
	}
}

func TestNoExecuteTaintManager_syncClusterBindingEviction(t *testing.T) {
	replica := int32(1)
	tests := []struct {
		name    string
		crb     *workv1alpha2.ClusterResourceBinding
		cluster *clusterv1alpha1.Cluster
		wcrb    *workv1alpha2.ClusterResourceBinding
		wantErr bool
	}{
		{
			name: "crb without tolerations",
			crb: &workv1alpha2.ClusterResourceBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterResourceBinding",
					APIVersion: "work.karmada.io/v1alpha2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-crb",
					Annotations: map[string]string{"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1","member2"]},"clusterTolerations":[{"key":"cluster.karmada.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}],"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted","weightPreference":{"staticWeightList":[{"targetCluster":{"clusterNames":["member1"]},"weight":1},{"targetCluster":{"clusterNames":["member2"]},"weight":1}]}}}`},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "test-cluster",
							Replicas: 1,
						},
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:    "cluster.karmada.io/not-ready",
							Effect: corev1.TaintEffectNoExecute,
						},
					},
				},
			},
			wcrb: &workv1alpha2.ClusterResourceBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterResourceBinding",
					APIVersion: "work.karmada.io/v1alpha2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-crb",
					Annotations: map[string]string{"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1","member2"]},"clusterTolerations":[{"key":"cluster.karmada.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}],"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted","weightPreference":{"staticWeightList":[{"targetCluster":{"clusterNames":["member1"]},"weight":1},{"targetCluster":{"clusterNames":["member2"]},"weight":1}]}}}`},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster: "test-cluster",
							PurgeMode:   policyv1alpha1.PurgeModeGracefully,
							Replicas:    &replica,
							Reason:      workv1alpha2.EvictionReasonTaintUntolerated,
							Producer:    workv1alpha2.EvictionProducerTaintManager,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "crb with tolerations",
			crb: &workv1alpha2.ClusterResourceBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterResourceBinding",
					APIVersion: "work.karmada.io/v1alpha2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-crb",
					Annotations: map[string]string{"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1","member2"]},"clusterTolerations":[{"key":"cluster.karmada.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":30},{"key":"cluster.karmada.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}],"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted","weightPreference":{"staticWeightList":[{"targetCluster":{"clusterNames":["member1"]},"weight":1},{"targetCluster":{"clusterNames":["member2"]},"weight":1}]}}}`},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "test-cluster",
							Replicas: 1,
						},
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:    "cluster.karmada.io/not-ready",
							Effect: corev1.TaintEffectNoExecute,
						},
					},
				},
			},
			wcrb: &workv1alpha2.ClusterResourceBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterResourceBinding",
					APIVersion: "work.karmada.io/v1alpha2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-crb",
					Annotations: map[string]string{"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1","member2"]},"clusterTolerations":[{"key":"cluster.karmada.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}],"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted","weightPreference":{"staticWeightList":[{"targetCluster":{"clusterNames":["member1"]},"weight":1},{"targetCluster":{"clusterNames":["member2"]},"weight":1}]}}}`},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "test-cluster",
							Replicas: 1,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "crb not exist",
			wantErr: false,
		},
		{
			name: "cluster not exist",
			crb: &workv1alpha2.ClusterResourceBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterResourceBinding",
					APIVersion: "work.karmada.io/v1alpha2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-crb",
					Annotations: map[string]string{"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1","member2"]},"clusterTolerations":[{"key":"cluster.karmada.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":30},{"key":"cluster.karmada.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}],"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted","weightPreference":{"staticWeightList":[{"targetCluster":{"clusterNames":["member1"]},"weight":1},{"targetCluster":{"clusterNames":["member2"]},"weight":1}]}}}`},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "test-cluster",
							Replicas: 1,
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newNoExecuteTaintManager()
			if tt.crb != nil {
				if err := tc.Create(context.Background(), tt.crb, &client.CreateOptions{}); err != nil {
					t.Fatalf("failed to create crb: %v", err)
				}
			}

			if tt.cluster != nil {
				if err := tc.Create(context.Background(), tt.cluster, &client.CreateOptions{}); err != nil {
					t.Fatalf("failed to create cluster: %v", err)
				}
			}

			key := keys.FederatedKey{
				Cluster: "test-cluster",
				ClusterWideKey: keys.ClusterWideKey{
					Kind: "ClusterResourceBinding",
					Name: "test-crb",
				},
			}
			if err := tc.syncClusterBindingEviction(key); (err != nil) != tt.wantErr {
				t.Errorf("NoExecuteTaintManager.syncClusterBindingEviction() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wcrb != nil {
				gCRB := &workv1alpha2.ClusterResourceBinding{}
				if err := tc.Get(context.Background(), types.NamespacedName{Name: tt.wcrb.Name}, gCRB, &client.GetOptions{}); err != nil {
					t.Fatalf("failed to get rb, error %v", err)
				}

				if !reflect.DeepEqual(tt.wcrb.Spec, gCRB.Spec) {
					t.Errorf("ResourceBinding get %+v, want %+v", gCRB.Spec, tt.wcrb.Spec)
				}
			}
		})
	}
}

func TestNoExecuteTaintManager_needEviction(t *testing.T) {
	now := time.Now()
	oneSecondAgo := metav1.Time{Time: now.Add(-1 * time.Second)}
	oneSecondLater := metav1.Time{Time: now.Add(1 * time.Second)}

	// Test cases
	tests := []struct {
		name                         string
		clusterName                  string
		annotations                  map[string]string
		cluster                      *clusterv1alpha1.Cluster
		enableNoExecuteTaintEviction bool
		expectedNeedEviction         bool
		expectedTolerationTime       time.Duration
		wantErr                      bool
	}{
		{
			name:        "placement annotation not exist",
			clusterName: "test-cluster",
			annotations: map[string]string{},
			wantErr:     true,
		},
		{
			name:        "placement annotation is invalid JSON",
			clusterName: "test-cluster",
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": "invalid-json",
			},
			wantErr: true,
		},
		{
			name:        "placement is nil",
			clusterName: "test-cluster",
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": "",
			},
			wantErr: true,
		},
		{
			name:        "cluster not found",
			clusterName: "non-existent-cluster",
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]}}`,
			},
			expectedNeedEviction:   false,
			expectedTolerationTime: -1,
			wantErr:                false,
		},
		{
			name:        "no execute taint eviction disabled",
			clusterName: "test-cluster",
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]}}`,
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:    "test-key",
							Effect: corev1.TaintEffectNoExecute,
						},
					},
				},
			},
			enableNoExecuteTaintEviction: false,
			expectedNeedEviction:         false,
			expectedTolerationTime:       -1,
			wantErr:                      false,
		},
		{
			name:        "cluster has no NoExecute taints",
			clusterName: "test-cluster",
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]}}`,
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:    "test-key",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
			enableNoExecuteTaintEviction: true,
			expectedNeedEviction:         false,
			expectedTolerationTime:       -1,
			wantErr:                      false,
		},
		{
			name:        "taint not tolerated - immediate eviction",
			clusterName: "test-cluster",
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[]}`,
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:       "test-key",
							Effect:    corev1.TaintEffectNoExecute,
							TimeAdded: &oneSecondAgo,
						},
					},
				},
			},
			enableNoExecuteTaintEviction: true,
			expectedNeedEviction:         true,
			expectedTolerationTime:       0,
			wantErr:                      false,
		},
		{
			name:        "taint tolerated with zero toleration seconds - immediate eviction",
			clusterName: "test-cluster",
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[{"key":"test-key","operator":"Exists","effect":"NoExecute","tolerationSeconds":0}]}`,
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:       "test-key",
							Effect:    corev1.TaintEffectNoExecute,
							TimeAdded: &oneSecondAgo,
						},
					},
				},
			},
			enableNoExecuteTaintEviction: true,
			expectedNeedEviction:         true,
			expectedTolerationTime:       0,
			wantErr:                      false,
		},
		{
			name:        "taint tolerated with positive toleration seconds - wait for toleration time",
			clusterName: "test-cluster",
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[{"key":"test-key","operator":"Exists","effect":"NoExecute","tolerationSeconds":60}]}`,
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:       "test-key",
							Effect:    corev1.TaintEffectNoExecute,
							TimeAdded: &oneSecondAgo,
						},
					},
				},
			},
			enableNoExecuteTaintEviction: true,
			expectedNeedEviction:         false,
			expectedTolerationTime:       59 * time.Second,
			wantErr:                      false,
		},
		{
			name:        "taint tolerated forever - no eviction",
			clusterName: "test-cluster",
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[{"key":"test-key","operator":"Exists","effect":"NoExecute"}]}`,
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:       "test-key",
							Effect:    corev1.TaintEffectNoExecute,
							TimeAdded: &oneSecondAgo,
						},
					},
				},
			},
			enableNoExecuteTaintEviction: true,
			expectedNeedEviction:         false,
			expectedTolerationTime:       -1,
			wantErr:                      false,
		},
		{
			name:        "multiple taints with mixed tolerations",
			clusterName: "test-cluster",
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[{"key":"tolerated-key","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}]}`,
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:       "tolerated-key",
							Effect:    corev1.TaintEffectNoExecute,
							TimeAdded: &oneSecondLater,
						},
						{
							Key:       "not-tolerated-key",
							Effect:    corev1.TaintEffectNoExecute,
							TimeAdded: &oneSecondAgo,
						},
					},
				},
			},
			enableNoExecuteTaintEviction: true,
			expectedNeedEviction:         true,
			expectedTolerationTime:       0,
			wantErr:                      false,
		},
		{
			name:        "toleration time expired - immediate eviction",
			clusterName: "test-cluster",
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[{"key":"test-key","operator":"Exists","effect":"NoExecute","tolerationSeconds":1}]}`,
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:       "test-key",
							Effect:    corev1.TaintEffectNoExecute,
							TimeAdded: &metav1.Time{Time: now.Add(-2 * time.Second)}, // 2 seconds ago, tolerance is 1 second
						},
					},
				},
			},
			enableNoExecuteTaintEviction: true,
			expectedNeedEviction:         true,
			expectedTolerationTime:       0,
			wantErr:                      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newNoExecuteTaintManager()
			tc.EnableNoExecuteTaintEviction = tt.enableNoExecuteTaintEviction

			// Create cluster if provided
			if tt.cluster != nil {
				if err := tc.Client.Create(context.Background(), tt.cluster); err != nil {
					t.Fatalf("failed to create cluster: %v", err)
				}
			}

			needEviction, tolerationTime, err := tc.needEviction(tt.clusterName, tt.annotations)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("needEviction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If we expect an error, don't check other results
			if tt.wantErr {
				return
			}

			// Check needEviction result
			if needEviction != tt.expectedNeedEviction {
				t.Errorf("needEviction() needEviction = %v, want %v", needEviction, tt.expectedNeedEviction)
			}

			// Check tolerationTime result with tolerance for time-based tests
			if tt.expectedTolerationTime >= 0 && tolerationTime >= 0 {
				// For positive time values, allow 1 second tolerance
				tolerance := 1 * time.Second
				if tolerationTime < tt.expectedTolerationTime-tolerance || tolerationTime > tt.expectedTolerationTime+tolerance {
					t.Errorf("needEviction() tolerationTime = %v, want %v (±%v)", tolerationTime, tt.expectedTolerationTime, tolerance)
				}
			} else {
				// For negative values or zero, check exact match
				if tolerationTime != tt.expectedTolerationTime {
					t.Errorf("needEviction() tolerationTime = %v, want %v", tolerationTime, tt.expectedTolerationTime)
				}
			}
		})
	}
}
