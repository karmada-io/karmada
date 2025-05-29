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
	"fmt"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
)

func newClusterController() *Controller {
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
	client := fake.NewClientBuilder().WithScheme(gclient.NewSchema()).
		WithIndex(&workv1alpha2.ResourceBinding{}, indexregistry.ResourceBindingIndexByFieldCluster, rbIndexerFunc).
		WithIndex(&workv1alpha2.ClusterResourceBinding{}, indexregistry.ClusterResourceBindingIndexByFieldCluster, crbIndexerFunc).
		WithStatusSubresource(&clusterv1alpha1.Cluster{}).Build()
	return &Controller{
		Client:                    client,
		EventRecorder:             record.NewFakeRecorder(1024),
		clusterHealthMap:          newClusterHealthMap(),
		ClusterMonitorGracePeriod: 40 * time.Second,
	}
}

func TestController_Reconcile(t *testing.T) {
	req := controllerruntime.Request{NamespacedName: types.NamespacedName{Name: "test-cluster"}}
	tests := []struct {
		name     string
		cluster  *clusterv1alpha1.Cluster
		ns       *corev1.Namespace
		work     *workv1alpha1.Work
		del      bool
		wCluster *clusterv1alpha1.Cluster
		want     controllerruntime.Result
		wantErr  bool
	}{
		{
			name: "cluster without status",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
			},
			wCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{{
						Key:    clusterv1alpha1.TaintClusterNotReady,
						Effect: corev1.TaintEffectNoSchedule,
					}},
				},
				Status: clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{},
				},
			},
			want:    controllerruntime.Result{},
			wantErr: false,
		},
		{
			name: "cluster with ready condition",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{{
						Key:    clusterv1alpha1.TaintClusterNotReady,
						Effect: corev1.TaintEffectNoSchedule,
					}},
				},
				Status: clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   clusterv1alpha1.ClusterConditionReady,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			wCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
				Spec: clusterv1alpha1.ClusterSpec{Taints: []corev1.Taint{}},
				Status: clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   clusterv1alpha1.ClusterConditionReady,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			want:    controllerruntime.Result{},
			wantErr: false,
		},
		{
			name: "cluster with unknown condition",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{{
						Key:    clusterv1alpha1.TaintClusterNotReady,
						Effect: corev1.TaintEffectNoSchedule,
					}},
				},
				Status: clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   clusterv1alpha1.ClusterConditionReady,
							Status: metav1.ConditionUnknown,
						},
					},
				},
			},
			wCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
				Spec: clusterv1alpha1.ClusterSpec{Taints: []corev1.Taint{
					{
						Key:    clusterv1alpha1.TaintClusterUnreachable,
						Effect: corev1.TaintEffectNoSchedule,
					},
				}},
				Status: clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   clusterv1alpha1.ClusterConditionReady,
							Status: metav1.ConditionUnknown,
						},
					},
				},
			},
			want:    controllerruntime.Result{},
			wantErr: false,
		},
		{
			name: "cluster with false condition",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{{
						Key:    clusterv1alpha1.TaintClusterUnreachable,
						Effect: corev1.TaintEffectNoSchedule,
					}},
				},
				Status: clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   clusterv1alpha1.ClusterConditionReady,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			wCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
				Spec: clusterv1alpha1.ClusterSpec{Taints: []corev1.Taint{
					{
						Key:    clusterv1alpha1.TaintClusterNotReady,
						Effect: corev1.TaintEffectNoSchedule,
					},
				}},
				Status: clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   clusterv1alpha1.ClusterConditionReady,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			want:    controllerruntime.Result{},
			wantErr: false,
		},
		{
			name: "cluster not found",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster-noexist",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
			},
			want:    controllerruntime.Result{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClusterController()
			if tt.cluster != nil {
				if err := c.Create(context.Background(), tt.cluster, &client.CreateOptions{}); err != nil {
					t.Fatalf("failed to create cluster %v", err)
				}
			}

			if tt.ns != nil {
				if err := c.Create(context.Background(), tt.ns, &client.CreateOptions{}); err != nil {
					t.Fatalf("failed to create ns %v", err)
				}
			}

			if tt.work != nil {
				if err := c.Create(context.Background(), tt.work, &client.CreateOptions{}); err != nil {
					t.Fatalf("failed to create work %v", err)
				}
			}

			if tt.del {
				if err := c.Delete(context.Background(), tt.cluster, &client.DeleteOptions{}); err != nil {
					t.Fatalf("failed to delete cluster %v", err)
				}
			}

			got, err := c.Reconcile(context.Background(), req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Controller.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.Reconcile() = %v, want %v", got, tt.want)
				return
			}

			if tt.wCluster != nil {
				cluster := &clusterv1alpha1.Cluster{}
				if err := c.Get(context.Background(), types.NamespacedName{Name: tt.cluster.Name}, cluster, &client.GetOptions{}); err != nil {
					t.Errorf("failed to get cluster %v", err)
					return
				}

				cleanUpCluster(cluster)
				if !reflect.DeepEqual(cluster, tt.wCluster) {
					t.Errorf("Cluster resource reconcile get %v, want %v", *cluster, *tt.wCluster)
				}
			}
		})
	}
}

func TestController_monitorClusterHealth(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *clusterv1alpha1.Cluster
		wCluster *clusterv1alpha1.Cluster
		wantErr  bool
	}{
		{
			name: "cluster without status",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
				Spec: clusterv1alpha1.ClusterSpec{
					SyncMode: clusterv1alpha1.Pull,
				},
			},
			wCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
				Spec: clusterv1alpha1.ClusterSpec{
					SyncMode: clusterv1alpha1.Pull,
					Taints:   []corev1.Taint{},
				},
				Status: clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{{
						Type:    clusterv1alpha1.ClusterConditionReady,
						Status:  metav1.ConditionUnknown,
						Reason:  "ClusterStatusNeverUpdated",
						Message: "Cluster status controller never posted cluster status.",
					}},
				},
			},
			wantErr: false,
		},
		{
			name: "cluster with ready condition",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
				Spec: clusterv1alpha1.ClusterSpec{
					SyncMode: clusterv1alpha1.Pull,
				},
				Status: clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   clusterv1alpha1.ClusterConditionReady,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			wCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:       "test-cluster",
					Finalizers: []string{util.ClusterControllerFinalizer},
				},
				Spec: clusterv1alpha1.ClusterSpec{
					SyncMode: clusterv1alpha1.Pull,
					Taints:   []corev1.Taint{},
				},
				Status: clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   clusterv1alpha1.ClusterConditionReady,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := features.FeatureGate.Set(fmt.Sprintf("%s=%t", features.Failover, true))
			if err != nil {
				t.Fatalf("Failed to enable failover feature gate: %v", err)
			}
			c := newClusterController()
			if tt.cluster != nil {
				if err = c.Create(context.Background(), tt.cluster, &client.CreateOptions{}); err != nil {
					t.Fatalf("failed to create cluster: %v", err)
				}
			}

			if err = c.monitorClusterHealth(context.Background()); (err != nil) != tt.wantErr {
				t.Errorf("Controller.monitorClusterHealth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			cluster := &clusterv1alpha1.Cluster{}
			if err = c.Get(context.Background(), types.NamespacedName{Name: "test-cluster"}, cluster, &client.GetOptions{}); err != nil {
				t.Errorf("failed to get cluster: %v", err)
				return
			}

			cleanUpCluster(cluster)
			if !reflect.DeepEqual(cluster, tt.wCluster) {
				t.Errorf("Cluster resource get %+v, want %+v", *cluster, *tt.wCluster)
				return
			}
		})
	}
}

// cleanUpCluster removes unnecessary fields from Cluster resource for testing purposes.
func cleanUpCluster(c *clusterv1alpha1.Cluster) {
	c.ObjectMeta.ResourceVersion = ""

	taints := []corev1.Taint{}
	for _, taint := range c.Spec.Taints {
		taint.TimeAdded = nil
		taints = append(taints, taint)
	}
	c.Spec.Taints = taints

	cond := []metav1.Condition{}
	for _, condition := range c.Status.Conditions {
		condition.LastTransitionTime = metav1.Time{}
		cond = append(cond, condition)
	}
	c.Status.Conditions = cond
}
