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

package deploymentreplicassyncer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// predicateTestCase defines the structure for testing predicate functions
type predicateTestCase struct {
	name      string
	oldObj    *appsv1.Deployment
	newObj    *appsv1.Deployment
	expected  bool
	eventType string
}

func TestPredicateFunc(t *testing.T) {
	testCases := []predicateTestCase{
		{
			name: "retain-replicas label added",
			oldObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			newObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{util.RetainReplicasLabel: util.RetainReplicasValue},
				},
			},
			expected:  true,
			eventType: "update",
		},
		{
			name: "replicas changed",
			oldObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{util.RetainReplicasLabel: util.RetainReplicasValue},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 3,
				},
			},
			newObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{util.RetainReplicasLabel: util.RetainReplicasValue},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 4,
				},
			},
			expected:  true,
			eventType: "update",
		},
		{
			name: "no relevant changes",
			oldObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{util.RetainReplicasLabel: util.RetainReplicasValue},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 3,
				},
			},
			newObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{util.RetainReplicasLabel: util.RetainReplicasValue},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 3,
				},
			},
			expected:  false,
			eventType: "update",
		},
		{
			name:      "create event test",
			oldObj:    nil,
			newObj:    &appsv1.Deployment{},
			expected:  false,
			eventType: "create",
		},
		{
			name:      "delete event test",
			oldObj:    &appsv1.Deployment{},
			newObj:    nil,
			expected:  false,
			eventType: "delete",
		},
		{
			name:      "generic event test",
			oldObj:    nil,
			newObj:    &appsv1.Deployment{},
			expected:  false,
			eventType: "generic",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result bool
			// Handle different event types using the eventType field
			switch tc.eventType {
			case "create":
				result = predicateFunc.Create(event.CreateEvent{Object: tc.newObj})
			case "delete":
				result = predicateFunc.Delete(event.DeleteEvent{Object: tc.oldObj})
			case "generic":
				result = predicateFunc.Generic(event.GenericEvent{Object: tc.newObj})
			case "update":
				result = predicateFunc.Update(event.UpdateEvent{
					ObjectOld: tc.oldObj,
					ObjectNew: tc.newObj,
				})
			}
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name             string
		deployment       *appsv1.Deployment
		binding          *workv1alpha2.ResourceBinding
		expectedResult   reconcile.Result
		expectedError    bool
		expectedReplicas *int32
	}{
		{
			name: "successful replicas sync",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
					Labels:    map[string]string{util.RetainReplicasLabel: util.RetainReplicasValue},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 4,
				},
			},
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       names.GenerateBindingName(util.DeploymentKind, "test-deployment"),
					Namespace:  "default",
					Generation: 1,
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Replicas: 3,
					Placement: &policyv1alpha1.Placement{
						ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
							ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
						},
					},
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
						{Name: "cluster2"},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedGeneration: 1,
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status: &runtime.RawExtension{
								Raw: []byte(`{"observedGeneration":1,"replicas":3,"updatedReplicas":3,"readyReplicas":3,"availableReplicas":3}`),
							},
						},
						{
							ClusterName: "cluster2",
							Status: &runtime.RawExtension{
								Raw: []byte(`{"observedGeneration":1,"replicas":1,"updatedReplicas":1,"readyReplicas":1,"availableReplicas":1}`),
							},
						},
					},
				},
			},
			expectedResult:   reconcile.Result{},
			expectedError:    false,
			expectedReplicas: ptr.To[int32](4),
		},
		{
			name: "binding not found",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
					Labels:    map[string]string{util.RetainReplicasLabel: util.RetainReplicasValue},
				},
			},
			expectedResult: reconcile.Result{},
			expectedError:  false,
		},
		{
			name: "non-divided scheduling type",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
					Labels:    map[string]string{util.RetainReplicasLabel: util.RetainReplicasValue},
				},
			},
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.GenerateBindingName(util.DeploymentKind, "test-deployment"),
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{},
				},
			},
			expectedResult: reconcile.Result{},
			expectedError:  false,
		},
		{
			name: "replicas already synced",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
					Labels:    map[string]string{util.RetainReplicasLabel: util.RetainReplicasValue},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 3,
				},
			},
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.GenerateBindingName(util.DeploymentKind, "test-deployment"),
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Replicas: 3,
					Placement: &policyv1alpha1.Placement{
						ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
							ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
						},
					},
				},
			},
			expectedResult: reconcile.Result{},
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = appsv1.AddToScheme(scheme)
			_ = workv1alpha2.Install(scheme)

			var objs []runtime.Object
			if tc.deployment != nil {
				objs = append(objs, tc.deployment)
			}
			if tc.binding != nil {
				objs = append(objs, tc.binding)
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&appsv1.Deployment{}).
				Build()
			r := &DeploymentReplicasSyncer{Client: client}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-deployment",
					Namespace: "default",
				},
			}

			result, err := r.Reconcile(context.TODO(), req)

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedResult, result)

			if tc.expectedReplicas != nil {
				updatedDeployment := &appsv1.Deployment{}
				err := client.Get(context.TODO(), types.NamespacedName{Name: "test-deployment", Namespace: "default"}, updatedDeployment)
				assert.NoError(t, err)
				assert.Equal(t, *tc.expectedReplicas, *updatedDeployment.Spec.Replicas)
			}
		})
	}
}

func TestIsDeploymentStatusCollected(t *testing.T) {
	testCases := []struct {
		name       string
		deployment *appsv1.Deployment
		binding    *workv1alpha2.ResourceBinding
		expected   bool
	}{
		{
			name: "status fully collected",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 3,
				},
			},
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Replicas: 3,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
						{Name: "cluster2"},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedGeneration: 1,
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      &runtime.RawExtension{Raw: []byte(`{"replicas": 2}`)},
						},
						{
							ClusterName: "cluster2",
							Status:      &runtime.RawExtension{Raw: []byte(`{"replicas": 1}`)},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "binding replicas not matching deployment spec",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 3,
				},
			},
			binding: &workv1alpha2.ResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Replicas: 2,
				},
			},
			expected: false,
		},
		{
			name: "scheduler observed generation not matching",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 3,
				},
			},
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 2,
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Replicas: 3,
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedGeneration: 1,
				},
			},
			expected: false,
		},
		{
			name: "incomplete aggregated status",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 3,
				},
			},
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Replicas: 3,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
						{Name: "cluster2"},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedGeneration: 1,
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      &runtime.RawExtension{Raw: []byte(`{"replicas": 2}`)},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "aggregated status is nil",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 3,
				},
			},
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Replicas: 3,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedGeneration: 1,
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      nil,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "invalid status JSON",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 3,
				},
			},
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Replicas: 3,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedGeneration: 1,
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      &runtime.RawExtension{Raw: []byte(`invalid json`)},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "zero replicas in status",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 3,
				},
			},
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Replicas: 3,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedGeneration: 1,
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      &runtime.RawExtension{Raw: []byte(`{"replicas": 0}`)},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "binding status replicas not matching deployment status",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: appsv1.DeploymentStatus{
					Replicas: 3,
				},
			},
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Replicas: 3,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedGeneration: 1,
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      &runtime.RawExtension{Raw: []byte(`{"replicas": 2}`)},
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isDeploymentStatusCollected(tc.deployment, tc.binding)
			assert.Equal(t, tc.expected, result)
		})
	}
}
