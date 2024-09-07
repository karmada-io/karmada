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
)

func TestPredicateFunc(t *testing.T) {
	testCases := []struct {
		name     string
		oldObj   *appsv1.Deployment
		newObj   *appsv1.Deployment
		expected bool
	}{
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
			expected: true,
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
			expected: true,
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
			expected: false,
		},
		{
			name:     "create event",
			oldObj:   nil,
			newObj:   &appsv1.Deployment{},
			expected: false,
		},
		{
			name:     "delete event",
			oldObj:   &appsv1.Deployment{},
			newObj:   nil,
			expected: false,
		},
		{
			name:     "generic event",
			oldObj:   nil,
			newObj:   &appsv1.Deployment{},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result bool
			switch {
			case tc.name == "create event":
				result = predicateFunc.Create(event.CreateEvent{Object: tc.newObj})
			case tc.name == "delete event":
				result = predicateFunc.Delete(event.DeleteEvent{Object: tc.oldObj})
			case tc.name == "generic event":
				result = predicateFunc.Generic(event.GenericEvent{Object: tc.newObj})
			default:
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
					Name:      "deployment-test-deployment",
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
					Name:      "deployment-test-deployment",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
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

			objs := []runtime.Object{}
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
