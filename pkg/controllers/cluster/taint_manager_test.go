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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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

func TestNoExecuteTaintManager_needEvictionWithCurrentTime(t *testing.T) {
	// Use fixed time for deterministic testing
	fixedTime := time.Date(2025, 9, 23, 12, 0, 0, 0, time.UTC)
	oneSecondAgo := metav1.Time{Time: fixedTime.Add(-1 * time.Second)}
	oneSecondLater := metav1.Time{Time: fixedTime.Add(1 * time.Second)}

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
							TimeAdded: &metav1.Time{Time: fixedTime.Add(-2 * time.Second)}, // 2 seconds ago, tolerance is 1 second
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

			needEviction, tolerationTime, err := tc.needEvictionWithCurrentTime(tt.clusterName, tt.annotations, fixedTime)

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

			// Check tolerationTime result - now we can do exact comparison since we use fixed time
			if tolerationTime != tt.expectedTolerationTime {
				t.Errorf("needEviction() tolerationTime = %v, want %v", tolerationTime, tt.expectedTolerationTime)
			}
		})
	}
}

func Test_getPurgeMode(t *testing.T) {
	tests := []struct {
		name         string
		taintManager *NoExecuteTaintManager
		failover     *policyv1alpha1.FailoverBehavior
		expected     policyv1alpha1.PurgeMode
	}{
		{
			name:         "failover spec is nil, global config is empty, should default to Gracefully",
			taintManager: &NoExecuteTaintManager{NoExecuteTaintEvictionPurgeMode: ""},
			failover:     nil,
			expected:     policyv1alpha1.PurgeModeGracefully,
		},
		{
			name:         "failover spec is nil, global config is Directly",
			taintManager: &NoExecuteTaintManager{NoExecuteTaintEvictionPurgeMode: string(policyv1alpha1.PurgeModeDirectly)},
			failover:     nil,
			expected:     policyv1alpha1.PurgeModeDirectly,
		},
		{
			name:         "failover spec is nil, global config is Gracefully",
			taintManager: &NoExecuteTaintManager{NoExecuteTaintEvictionPurgeMode: string(policyv1alpha1.PurgeModeGracefully)},
			failover:     nil,
			expected:     policyv1alpha1.PurgeModeGracefully,
		},
		{
			name:         "failover spec is present, but cluster is nil, should fallback to global config",
			taintManager: &NoExecuteTaintManager{NoExecuteTaintEvictionPurgeMode: string(policyv1alpha1.PurgeModeDirectly)},
			failover:     &policyv1alpha1.FailoverBehavior{},
			expected:     policyv1alpha1.PurgeModeDirectly,
		},
		{
			name:         "failover spec is present, PurgeMode is empty, should be Gracefully",
			taintManager: &NoExecuteTaintManager{NoExecuteTaintEvictionPurgeMode: string(policyv1alpha1.PurgeModeDirectly)},
			failover: &policyv1alpha1.FailoverBehavior{
				Cluster: &policyv1alpha1.ClusterFailoverBehavior{
					PurgeMode: "",
				},
			},
			expected: policyv1alpha1.PurgeModeGracefully,
		},
		{
			name:         "failover spec is present, PurgeMode is Directly",
			taintManager: &NoExecuteTaintManager{NoExecuteTaintEvictionPurgeMode: string(policyv1alpha1.PurgeModeGracefully)},
			failover: &policyv1alpha1.FailoverBehavior{
				Cluster: &policyv1alpha1.ClusterFailoverBehavior{
					PurgeMode: policyv1alpha1.PurgeModeDirectly,
				},
			},
			expected: policyv1alpha1.PurgeModeDirectly,
		},
		{
			name:         "failover spec is present, PurgeMode is Gracefully",
			taintManager: &NoExecuteTaintManager{NoExecuteTaintEvictionPurgeMode: string(policyv1alpha1.PurgeModeDirectly)},
			failover: &policyv1alpha1.FailoverBehavior{
				Cluster: &policyv1alpha1.ClusterFailoverBehavior{
					PurgeMode: policyv1alpha1.PurgeModeGracefully,
				},
			},
			expected: policyv1alpha1.PurgeModeGracefully,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.taintManager.getPurgeMode(tt.failover); got != tt.expected {
				t.Errorf("getPurgeMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func Test_enqueueBinding(t *testing.T) {
	fixedTime := time.Date(2025, 9, 23, 12, 0, 0, 0, time.UTC)
	oneSecondAgo := metav1.Time{Time: fixedTime.Add(-1 * time.Second)}
	dummyKey := keys.FederatedKey{ClusterWideKey: keys.ClusterWideKey{Name: "test-binding"}}

	tests := []struct {
		name        string
		taints      []corev1.Taint
		annotations map[string]string
		wantAction  string // "skip" (not enqueued), "immediate" (delay=0), "delayed" (delay>0)
	}{
		{
			name:        "no taints - skip",
			taints:      nil,
			annotations: map[string]string{},
			wantAction:  "skip",
		},
		{
			name: "no placement annotation (fail-safe: immediate)",
			taints: []corev1.Taint{
				{Key: "test-key", Effect: corev1.TaintEffectNoExecute, TimeAdded: &oneSecondAgo},
			},
			annotations: map[string]string{},
			wantAction:  "immediate",
		},
		{
			name: "invalid placement JSON (fail-safe: immediate)",
			taints: []corev1.Taint{
				{Key: "test-key", Effect: corev1.TaintEffectNoExecute, TimeAdded: &oneSecondAgo},
			},
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": "invalid-json",
			},
			wantAction: "immediate",
		},
		{
			name: "no tolerations - immediate eviction",
			taints: []corev1.Taint{
				{Key: "test-key", Effect: corev1.TaintEffectNoExecute, TimeAdded: &oneSecondAgo},
			},
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1"]},"clusterTolerations":[]}`,
			},
			wantAction: "immediate",
		},
		{
			name: "finite tolerationSeconds unexpired - delayed eviction",
			taints: []corev1.Taint{
				{Key: "test-key", Effect: corev1.TaintEffectNoExecute, TimeAdded: &oneSecondAgo},
			},
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1"]},"clusterTolerations":[{"key":"test-key","operator":"Exists","effect":"NoExecute","tolerationSeconds":60}]}`,
			},
			wantAction: "delayed",
		},
		{
			name: "infinite toleration (no tolerationSeconds) - skip",
			taints: []corev1.Taint{
				{Key: "test-key", Effect: corev1.TaintEffectNoExecute, TimeAdded: &oneSecondAgo},
			},
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1"]},"clusterTolerations":[{"key":"test-key","operator":"Exists","effect":"NoExecute"}]}`,
			},
			wantAction: "skip",
		},
		{
			name: "multiple taints - one not tolerated - immediate",
			taints: []corev1.Taint{
				{Key: "tolerated-key", Effect: corev1.TaintEffectNoExecute, TimeAdded: &oneSecondAgo},
				{Key: "not-tolerated-key", Effect: corev1.TaintEffectNoExecute, TimeAdded: &oneSecondAgo},
			},
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1"]},"clusterTolerations":[{"key":"tolerated-key","operator":"Exists","effect":"NoExecute"}]}`,
			},
			wantAction: "immediate",
		},
		{
			name: "multiple taints - all tolerated indefinitely - skip",
			taints: []corev1.Taint{
				{Key: "key1", Effect: corev1.TaintEffectNoExecute, TimeAdded: &oneSecondAgo},
				{Key: "key2", Effect: corev1.TaintEffectNoExecute, TimeAdded: &oneSecondAgo},
			},
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1"]},"clusterTolerations":[{"key":"key1","operator":"Exists","effect":"NoExecute"},{"key":"key2","operator":"Exists","effect":"NoExecute"}]}`,
			},
			wantAction: "skip",
		},
		{
			name: "multiple taints - one tolerated with finite seconds - delayed",
			taints: []corev1.Taint{
				{Key: "key1", Effect: corev1.TaintEffectNoExecute, TimeAdded: &oneSecondAgo},
				{Key: "key2", Effect: corev1.TaintEffectNoExecute, TimeAdded: &oneSecondAgo},
			},
			annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["member1"]},"clusterTolerations":[{"key":"key1","operator":"Exists","effect":"NoExecute"},{"key":"key2","operator":"Exists","effect":"NoExecute","tolerationSeconds":120}]}`,
			},
			wantAction: "delayed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type enqueueRecord struct {
				delay time.Duration
			}
			var records []enqueueRecord
			recorder := &recordingWorker{
				addAfterFunc: func(_ any, delay time.Duration) {
					records = append(records, enqueueRecord{delay: delay})
				},
			}

			tc := &NoExecuteTaintManager{
				bindingEvictionWorker: recorder,
			}
			tc.enqueueBinding(tc.bindingEvictionWorker, tt.taints, tt.annotations, dummyKey, fixedTime)

			var gotAction string
			switch {
			case len(records) == 0:
				gotAction = "skip"
			case records[0].delay == 0:
				gotAction = "immediate"
			default:
				gotAction = "delayed"
			}
			if gotAction != tt.wantAction {
				t.Errorf("enqueueBinding() action=%s, want %s (records=%v)", gotAction, tt.wantAction, records)
			}
		})
	}
}

func Test_syncCluster_enqueuesBindingsCorrectly(t *testing.T) {
	oneSecondAgo := metav1.Time{Time: time.Now().Add(-1 * time.Second)}

	cluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: clusterv1alpha1.ClusterSpec{
			Taints: []corev1.Taint{
				{
					Key:       "cluster.karmada.io/maintenance",
					Effect:    corev1.TaintEffectNoExecute,
					TimeAdded: &oneSecondAgo,
				},
			},
		},
	}

	rbInfinite := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "infinite-rb",
			Namespace: "default",
			Annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[{"key":"cluster.karmada.io/maintenance","operator":"Exists","effect":"NoExecute"}]}`,
			},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: []workv1alpha2.TargetCluster{{Name: "test-cluster", Replicas: 1}},
		},
	}

	rbFinite := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "finite-rb",
			Namespace: "default",
			Annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[{"key":"cluster.karmada.io/maintenance","operator":"Exists","effect":"NoExecute","tolerationSeconds":142}]}`,
			},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: []workv1alpha2.TargetCluster{{Name: "test-cluster", Replicas: 1}},
		},
	}

	rbNoToleration := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-toleration-rb",
			Namespace: "default",
			Annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[]}`,
			},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: []workv1alpha2.TargetCluster{{Name: "test-cluster", Replicas: 1}},
		},
	}

	crbNoToleration := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "no-toleration-crb",
			Annotations: map[string]string{
				"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[]}`,
			},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: []workv1alpha2.TargetCluster{{Name: "test-cluster", Replicas: 1}},
		},
	}

	scheme := gclient.NewSchema()
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

	innerClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&workv1alpha2.ResourceBinding{}, indexregistry.ResourceBindingIndexByFieldCluster, rbIndexerFunc).
		WithIndex(&workv1alpha2.ClusterResourceBinding{}, indexregistry.ClusterResourceBindingIndexByFieldCluster, crbIndexerFunc).
		WithObjects(cluster, rbInfinite, rbFinite, rbNoToleration, crbNoToleration).
		Build()
	fakeClient := &gvkSettingClient{Client: innerClient}

	type enqueueRecord struct {
		name  string
		delay time.Duration
	}
	// recordEnqueues builds a worker that records enqueued keys into the given slice.
	recordEnqueues := func(records *[]enqueueRecord) *recordingWorker {
		return &recordingWorker{
			addAfterFunc: func(item any, delay time.Duration) {
				if k, ok := item.(keys.FederatedKey); ok {
					*records = append(*records, enqueueRecord{name: k.Name, delay: delay})
				}
			},
		}
	}
	var bindingRecords, clusterBindingRecords []enqueueRecord

	tc := &NoExecuteTaintManager{
		Client:                       fakeClient,
		EnableNoExecuteTaintEviction: true,
		bindingEvictionWorker:        recordEnqueues(&bindingRecords),
		clusterBindingEvictionWorker: recordEnqueues(&clusterBindingRecords),
	}

	_, err := tc.syncCluster(context.Background(), cluster)
	if err != nil {
		t.Fatalf("syncCluster() error = %v", err)
	}

	tests := []struct {
		bindingName string
		records     *[]enqueueRecord
		wantAction  string // "skip" (not enqueued), "immediate" (delay=0), "delayed" (delay>0)
	}{
		{bindingName: "infinite-rb", records: &bindingRecords, wantAction: "skip"},
		{bindingName: "finite-rb", records: &bindingRecords, wantAction: "delayed"},
		{bindingName: "no-toleration-rb", records: &bindingRecords, wantAction: "immediate"},
		// ClusterResourceBindings must be routed to the clusterBindingEvictionWorker,
		// not the bindingEvictionWorker.
		{bindingName: "no-toleration-crb", records: &clusterBindingRecords, wantAction: "immediate"},
	}
	for _, tt := range tests {
		t.Run(tt.bindingName, func(t *testing.T) {
			gotAction := "skip"
			for _, r := range *tt.records {
				if r.name != tt.bindingName {
					continue
				}
				if r.delay > 0 {
					gotAction = "delayed"
				} else {
					gotAction = "immediate"
				}
				break
			}
			if gotAction != tt.wantAction {
				t.Errorf("binding %s: enqueue action = %s, want %s", tt.bindingName, gotAction, tt.wantAction)
			}
		})
	}

	// No unexpected extra items should be enqueued.
	if len(bindingRecords) != 2 {
		t.Errorf("expected 2 ResourceBindings enqueued to bindingEvictionWorker, got %d", len(bindingRecords))
	}
	if len(clusterBindingRecords) != 1 {
		t.Errorf("expected 1 ClusterResourceBinding enqueued to clusterBindingEvictionWorker, got %d", len(clusterBindingRecords))
	}
}

type recordingWorker struct {
	addAfterFunc func(item any, delay time.Duration)
}

func (r *recordingWorker) Add(item any)                           { r.addAfterFunc(item, 0) }
func (r *recordingWorker) AddAfter(item any, delay time.Duration) { r.addAfterFunc(item, delay) }
func (r *recordingWorker) Enqueue(any)                            {}
func (r *recordingWorker) Run(_ context.Context, _ int)           {}

// gvkSettingClient wraps a client.Client and sets GVK on ResourceBinding items
// returned by List, working around the fake client stripping TypeMeta.
type gvkSettingClient struct {
	client.Client
}

func (c *gvkSettingClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if err := c.Client.List(ctx, list, opts...); err != nil {
		return err
	}
	if rbList, ok := list.(*workv1alpha2.ResourceBindingList); ok {
		for i := range rbList.Items {
			rbList.Items[i].SetGroupVersionKind(schema.GroupVersionKind{Group: workv1alpha2.GroupVersion.Group, Version: workv1alpha2.GroupVersion.Version, Kind: "ResourceBinding"})
		}
	}
	if crbList, ok := list.(*workv1alpha2.ClusterResourceBindingList); ok {
		for i := range crbList.Items {
			crbList.Items[i].SetGroupVersionKind(schema.GroupVersionKind{Group: workv1alpha2.GroupVersion.Group, Version: workv1alpha2.GroupVersion.Version, Kind: "ClusterResourceBinding"})
		}
	}
	return nil
}

func Test_extractStateForEviction(t *testing.T) {
	statusJSON := []byte(`{
        "metadata": {
            "labels": {
                "foo": "bar"
            }
        }
    }`)

	clusterFailoverBehavior := &policyv1alpha1.ClusterFailoverBehavior{
		StatePreservation: &policyv1alpha1.StatePreservation{
			Rules: []policyv1alpha1.StatePreservationRule{
				{
					AliasLabelName: "foo",
					JSONPath:       "{ .metadata.labels.foo }",
				},
			},
		},
	}

	tests := []struct {
		name             string
		cluster          string
		clusterBehavior  *policyv1alpha1.ClusterFailoverBehavior
		aggregatedStatus []workv1alpha2.AggregatedStatusItem
		expectedState    map[string]string
		expectErr        bool
		expectedErrMsg   string
	}{
		{
			name:            "aggregated status list does not contain the target cluster, should return error",
			cluster:         "member1",
			clusterBehavior: clusterFailoverBehavior,
			aggregatedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "member2", Status: &runtime.RawExtension{Raw: statusJSON}},
			},
			expectedState:  nil,
			expectErr:      true,
			expectedErrMsg: "the application status has not yet been collected from Cluster(member1)",
		},
		{
			name:            "aggregated status exists for target cluster but its raw status is nil, should return error",
			cluster:         "member1",
			clusterBehavior: clusterFailoverBehavior,
			aggregatedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "member1", Status: nil},
			},
			expectedState:  nil,
			expectErr:      true,
			expectedErrMsg: "the application status has not yet been collected from Cluster(member1)",
		},
		{
			name:            "successfully extract state",
			cluster:         "member1",
			clusterBehavior: clusterFailoverBehavior,
			aggregatedStatus: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Status: &runtime.RawExtension{
						Raw: statusJSON,
					},
				},
			},
			expectedState: map[string]string{
				"foo": "bar",
			},
			expectErr:      false,
			expectedErrMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preservedState, err := extractStateForEviction(tt.clusterBehavior, tt.aggregatedStatus, tt.cluster)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErrMsg, err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedState, preservedState)
			}
		})
	}
}

// newNoExecuteTaintManagerWithClient builds a NoExecuteTaintManager around a
// caller-supplied client.Client so tests can inject behavior (e.g. transient
// optimistic-lock conflicts) via interceptor.Funcs.
func newNoExecuteTaintManagerWithClient(c client.Client) *NoExecuteTaintManager {
	mgr := &NoExecuteTaintManager{
		Client:                          c,
		EnableNoExecuteTaintEviction:    true,
		NoExecuteTaintEvictionPurgeMode: "Gracefully",
	}
	mgr.bindingEvictionWorker = util.NewAsyncWorker(util.Options{
		Name:          "binding-eviction",
		KeyFunc:       nil,
		ReconcileFunc: mgr.syncBindingEviction,
	})
	mgr.clusterBindingEvictionWorker = util.NewAsyncWorker(util.Options{
		Name:          "cluster-binding-eviction",
		KeyFunc:       nil,
		ReconcileFunc: mgr.syncClusterBindingEviction,
	})
	return mgr
}

// conflictOnFirstNUpdates returns an interceptor that injects a Conflict error
// on the first n Update calls for the given GroupResource and lets subsequent
// Update calls pass through untouched. It is used to verify that the
// retry-on-conflict wrapper around the read-modify-write sequence transparently
// recovers from optimistic-lock collisions with peer writers.
func conflictOnFirstNUpdates(gr schema.GroupResource, name string, n int32) interceptor.Funcs {
	var remaining = n
	return interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if atomic.LoadInt32(&remaining) > 0 {
				atomic.AddInt32(&remaining, -1)
				// Bump the cached object's resourceVersion so the next Get inside
				// the retry loop observes a "newer" version, mirroring what
				// happens in production when a peer controller lands a write
				// between our Get and Update.
				if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err == nil {
					return apierrors.NewConflict(gr, name, errOptimisticLock)
				}
				return apierrors.NewConflict(gr, name, errOptimisticLock)
			}
			return c.Update(ctx, obj, opts...)
		},
	}
}

// errOptimisticLock matches the wording the apiserver uses for the
// stale-resourceVersion case so log messages remain familiar.
var errOptimisticLock = &errString{"the object has been modified; please apply your changes to the latest version and try again"}

type errString struct{ s string }

func (e *errString) Error() string { return e.s }

func TestNoExecuteTaintManager_syncBindingEviction_retryOnConflict(t *testing.T) {
	replica := int32(1)
	rb := &workv1alpha2.ResourceBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ResourceBinding",
			APIVersion: "work.karmada.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "conflict-rb",
			Namespace:   "default",
			Annotations: map[string]string{"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[{"key":"cluster.karmada.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}],"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted","weightPreference":{"staticWeightList":[{"targetCluster":{"clusterNames":["test-cluster"]},"weight":1}]}}}`},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: []workv1alpha2.TargetCluster{
				{Name: "test-cluster", Replicas: replica},
			},
		},
	}
	cluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
		Spec: clusterv1alpha1.ClusterSpec{
			Taints: []corev1.Taint{
				{Key: "cluster.karmada.io/not-ready", Effect: corev1.TaintEffectNoExecute},
			},
		},
	}

	rbIndexerFunc := func(obj client.Object) []string {
		rb, ok := obj.(*workv1alpha2.ResourceBinding)
		if !ok {
			return nil
		}
		return util.GetBindingClusterNames(&rb.Spec)
	}
	gr := schema.GroupResource{Group: "work.karmada.io", Resource: "resourcebindings"}
	fakeClient := fake.NewClientBuilder().
		WithScheme(gclient.NewSchema()).
		WithObjects(rb, cluster).
		WithIndex(&workv1alpha2.ResourceBinding{}, indexregistry.ResourceBindingIndexByFieldCluster, rbIndexerFunc).
		WithInterceptorFuncs(conflictOnFirstNUpdates(gr, rb.Name, 2)).
		Build()
	tc := newNoExecuteTaintManagerWithClient(fakeClient)

	key := keys.FederatedKey{
		Cluster: "test-cluster",
		ClusterWideKey: keys.ClusterWideKey{
			Kind:      "ResourceBinding",
			Name:      rb.Name,
			Namespace: rb.Namespace,
		},
	}
	if err := tc.syncBindingEviction(key); err != nil {
		t.Fatalf("syncBindingEviction returned error after transient conflicts: %v", err)
	}

	got := &workv1alpha2.ResourceBinding{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Namespace: rb.Namespace, Name: rb.Name}, got); err != nil {
		t.Fatalf("failed to fetch binding after sync: %v", err)
	}
	if len(got.Spec.GracefulEvictionTasks) != 1 {
		t.Fatalf("expected exactly one graceful eviction task to be appended, got %d", len(got.Spec.GracefulEvictionTasks))
	}
	task := got.Spec.GracefulEvictionTasks[0]
	if task.FromCluster != "test-cluster" {
		t.Errorf("expected eviction task FromCluster=%q, got %q", "test-cluster", task.FromCluster)
	}
	if task.Producer != workv1alpha2.EvictionProducerTaintManager {
		t.Errorf("expected eviction task Producer=%q, got %q", workv1alpha2.EvictionProducerTaintManager, task.Producer)
	}
	if task.PurgeMode != policyv1alpha1.PurgeModeGracefully {
		t.Errorf("expected eviction task PurgeMode=%q, got %q", policyv1alpha1.PurgeModeGracefully, task.PurgeMode)
	}
}

func TestNoExecuteTaintManager_syncClusterBindingEviction_retryOnConflict(t *testing.T) {
	replica := int32(1)
	crb := &workv1alpha2.ClusterResourceBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterResourceBinding",
			APIVersion: "work.karmada.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "conflict-crb",
			Annotations: map[string]string{"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[{"key":"cluster.karmada.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}],"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted","weightPreference":{"staticWeightList":[{"targetCluster":{"clusterNames":["test-cluster"]},"weight":1}]}}}`},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: []workv1alpha2.TargetCluster{
				{Name: "test-cluster", Replicas: replica},
			},
		},
	}
	cluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
		Spec: clusterv1alpha1.ClusterSpec{
			Taints: []corev1.Taint{
				{Key: "cluster.karmada.io/not-ready", Effect: corev1.TaintEffectNoExecute},
			},
		},
	}

	crbIndexerFunc := func(obj client.Object) []string {
		crb, ok := obj.(*workv1alpha2.ClusterResourceBinding)
		if !ok {
			return nil
		}
		return util.GetBindingClusterNames(&crb.Spec)
	}
	gr := schema.GroupResource{Group: "work.karmada.io", Resource: "clusterresourcebindings"}
	fakeClient := fake.NewClientBuilder().
		WithScheme(gclient.NewSchema()).
		WithObjects(crb, cluster).
		WithIndex(&workv1alpha2.ClusterResourceBinding{}, indexregistry.ClusterResourceBindingIndexByFieldCluster, crbIndexerFunc).
		WithInterceptorFuncs(conflictOnFirstNUpdates(gr, crb.Name, 2)).
		Build()
	tc := newNoExecuteTaintManagerWithClient(fakeClient)

	key := keys.FederatedKey{
		Cluster: "test-cluster",
		ClusterWideKey: keys.ClusterWideKey{
			Kind: "ClusterResourceBinding",
			Name: crb.Name,
		},
	}
	if err := tc.syncClusterBindingEviction(key); err != nil {
		t.Fatalf("syncClusterBindingEviction returned error after transient conflicts: %v", err)
	}

	got := &workv1alpha2.ClusterResourceBinding{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: crb.Name}, got); err != nil {
		t.Fatalf("failed to fetch cluster binding after sync: %v", err)
	}
	if len(got.Spec.GracefulEvictionTasks) != 1 {
		t.Fatalf("expected exactly one graceful eviction task to be appended, got %d", len(got.Spec.GracefulEvictionTasks))
	}
}

// TestNoExecuteTaintManager_syncBindingEviction_idempotent verifies that
// repeated reconciles for a binding that already has an eviction task for the
// target cluster do not stack duplicate tasks. This guards the safety of the
// retry-on-conflict refetch loop, which calls Spec.GracefulEvictCluster on
// each iteration.
func TestNoExecuteTaintManager_syncBindingEviction_idempotent(t *testing.T) {
	replica := int32(1)
	existingTask := workv1alpha2.GracefulEvictionTask{
		FromCluster: "test-cluster",
		PurgeMode:   policyv1alpha1.PurgeModeGracefully,
		Replicas:    &replica,
		Reason:      workv1alpha2.EvictionReasonTaintUntolerated,
		Producer:    workv1alpha2.EvictionProducerTaintManager,
	}
	rb := &workv1alpha2.ResourceBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ResourceBinding",
			APIVersion: "work.karmada.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "idempotent-rb",
			Namespace:   "default",
			Annotations: map[string]string{"policy.karmada.io/applied-placement": `{"clusterAffinity":{"clusterNames":["test-cluster"]},"clusterTolerations":[{"key":"cluster.karmada.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":30}],"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted","weightPreference":{"staticWeightList":[{"targetCluster":{"clusterNames":["test-cluster"]},"weight":1}]}}}`},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: []workv1alpha2.TargetCluster{
				{Name: "test-cluster", Replicas: replica},
			},
			GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{existingTask},
		},
	}
	cluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
		Spec: clusterv1alpha1.ClusterSpec{
			Taints: []corev1.Taint{
				{Key: "cluster.karmada.io/not-ready", Effect: corev1.TaintEffectNoExecute},
			},
		},
	}

	tc := newNoExecuteTaintManager()
	if err := tc.Create(context.Background(), rb); err != nil {
		t.Fatalf("failed to seed binding: %v", err)
	}
	if err := tc.Create(context.Background(), cluster); err != nil {
		t.Fatalf("failed to seed cluster: %v", err)
	}

	key := keys.FederatedKey{
		Cluster: "test-cluster",
		ClusterWideKey: keys.ClusterWideKey{
			Kind:      "ResourceBinding",
			Name:      rb.Name,
			Namespace: rb.Namespace,
		},
	}
	for i := range 3 {
		if err := tc.syncBindingEviction(key); err != nil {
			t.Fatalf("syncBindingEviction iteration %d returned error: %v", i, err)
		}
	}

	got := &workv1alpha2.ResourceBinding{}
	if err := tc.Get(context.Background(), types.NamespacedName{Namespace: rb.Namespace, Name: rb.Name}, got); err != nil {
		t.Fatalf("failed to refetch binding: %v", err)
	}
	if len(got.Spec.GracefulEvictionTasks) != 1 {
		t.Errorf("expected eviction task to remain a single entry across repeated reconciles, got %d tasks", len(got.Spec.GracefulEvictionTasks))
	}
}
