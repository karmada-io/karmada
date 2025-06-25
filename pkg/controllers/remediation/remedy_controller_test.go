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

package remediation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	remedyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1"
)

func TestReconcile(t *testing.T) {
	tests := []struct {
		name            string
		existingObjs    []runtime.Object
		inputCluster    *clusterv1alpha1.Cluster
		expectedError   bool
		expectedActions []string
		expectedRequeue bool
	}{
		{
			name:         "Cluster not found",
			existingObjs: []runtime.Object{},
			inputCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "non-existent-cluster",
				},
			},
			expectedError:   false,
			expectedActions: nil,
			expectedRequeue: false,
		},
		{
			name: "Cluster with no matching remedies",
			existingObjs: []runtime.Object{
				&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
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
			},
			inputCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			expectedError:   false,
			expectedActions: nil,
			expectedRequeue: false,
		},
		{
			name: "Cluster being deleted",
			existingObjs: []runtime.Object{
				&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "deleting-cluster",
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
						Finalizers:        []string{"test-finalizer"},
					},
				},
			},
			inputCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "deleting-cluster",
				},
			},
			expectedError:   false,
			expectedActions: nil,
			expectedRequeue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupScheme()
			fakeClient := createFakeClient(scheme, tt.existingObjs...)
			controller := &RemedyController{Client: fakeClient}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{Name: tt.inputCluster.Name},
			}
			result, err := controller.Reconcile(context.Background(), req)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Assert requeue result
			assert.Equal(t, tt.expectedRequeue, result.RequeueAfter > 0, "Unexpected requeue result")

			// Assert remedy actions if expected
			if tt.expectedActions != nil {
				updatedCluster := &clusterv1alpha1.Cluster{}
				err := fakeClient.Get(context.Background(), req.NamespacedName, updatedCluster)
				assert.NoError(t, err, "Failed to get updated cluster")
				assert.Equal(t, tt.expectedActions, updatedCluster.Status.RemedyActions, "Unexpected remedy actions")
			}
		})
	}
}

func TestSetupWithManager(t *testing.T) {
	scheme := setupScheme()
	tests := []struct {
		name            string
		controllerSetup func() *RemedyController
	}{
		{
			name: "setup with valid client",
			controllerSetup: func() *RemedyController {
				return &RemedyController{Client: createFakeClient(scheme)}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, err := setupManager(scheme)
			require.NoError(t, err)

			controller := tt.controllerSetup()
			err = controller.SetupWithManager(mgr)

			assert.NoError(t, err)
		})
	}
}

func TestSetupWatches(t *testing.T) {
	scheme := setupScheme()

	mgr, err := setupManager(scheme)
	require.NoError(t, err)

	controller := &RemedyController{Client: mgr.GetClient()}

	remedyController, err := controllerruntime.NewControllerManagedBy(mgr).
		For(&clusterv1alpha1.Cluster{}).
		Build(controller)
	require.NoError(t, err)

	err = controller.setupWatches(remedyController, mgr)
	assert.NoError(t, err)
}

// Helper Functions

// Helper function to set up the scheme
func setupScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clusterv1alpha1.Install(scheme)
	_ = remedyv1alpha1.Install(scheme)
	return scheme
}

// Helper function to create a fake client
func createFakeClient(scheme *runtime.Scheme, objs ...runtime.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
}

// Helper function to set up the manager
func setupManager(scheme *runtime.Scheme) (controllerruntime.Manager, error) {
	return controllerruntime.NewManager(&rest.Config{}, controllerruntime.Options{Scheme: scheme})
}
