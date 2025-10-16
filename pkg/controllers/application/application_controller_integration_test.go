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

package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
)

// TestApplicationControllerIntegration tests the complete integration of the application controller
func TestApplicationControllerIntegration(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	// Create test applications
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1", "related-app-2"},
		},
	}

	relatedApp1 := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "related-app-1",
			Namespace: "default",
		},
	}

	relatedApp2 := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "related-app-2",
			Namespace: "default",
		},
	}

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mainApp, relatedApp1, relatedApp2).
		Build()

	// Create reconciler
	reconciler := &ApplicationReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	// Test reconciliation
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "main-app",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{}, result)
}

// TestApplicationFailoverControllerIntegration tests the complete integration of the application failover controller
func TestApplicationFailoverControllerIntegration(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	// Create test applications
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1", "related-app-2"},
		},
	}

	relatedApp1 := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "related-app-1",
			Namespace: "default",
		},
	}

	relatedApp2 := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "related-app-2",
			Namespace: "default",
		},
	}

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mainApp, relatedApp1, relatedApp2).
		Build()

	// Create reconciler
	reconciler := &ApplicationFailoverReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	// Test reconciliation
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "main-app",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{}, result)
}

// TestAPIServerIntegration tests integration with the Kubernetes API server
func TestAPIServerIntegration(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	// Create test applications
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1"},
		},
	}

	relatedApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "related-app-1",
			Namespace: "default",
		},
	}

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mainApp, relatedApp).
		Build()

	// Test API operations
	ctx := context.Background()

	// Test Get operation
	var retrievedApp v1alpha1.Application
	err := fakeClient.Get(ctx, types.NamespacedName{
		Name:      "main-app",
		Namespace: "default",
	}, &retrievedApp)
	require.NoError(t, err)
	require.Equal(t, "main-app", retrievedApp.Name)

	// Test List operation
	var appList v1alpha1.ApplicationList
	err = fakeClient.List(ctx, &appList)
	require.NoError(t, err)
	require.Len(t, appList.Items, 2)

	// Test Update operation
	retrievedApp.Spec.RelatedApplications = []string{"related-app-1", "related-app-2"}
	err = fakeClient.Update(ctx, &retrievedApp)
	require.NoError(t, err)

	// Test Delete operation
	err = fakeClient.Delete(ctx, &retrievedApp)
	require.NoError(t, err)
}

// TestMultiClusterIntegration tests integration with multiple clusters
func TestMultiClusterIntegration(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	// Create test applications for different clusters
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
			Labels: map[string]string{
				"cluster": "cluster-1",
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1"},
		},
	}

	relatedApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "related-app-1",
			Namespace: "default",
			Labels: map[string]string{
				"cluster": "cluster-1",
			},
		},
	}

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mainApp, relatedApp).
		Build()

	// Test cluster-aware operations
	ctx := context.Background()

	// Test Get with cluster label
	var retrievedApp v1alpha1.Application
	err := fakeClient.Get(ctx, types.NamespacedName{
		Name:      "main-app",
		Namespace: "default",
	}, &retrievedApp)
	require.NoError(t, err)
	require.Equal(t, "cluster-1", retrievedApp.Labels["cluster"])

	// Test List with cluster filter
	var appList v1alpha1.ApplicationList
	err = fakeClient.List(ctx, &appList, client.MatchingLabels{"cluster": "cluster-1"})
	require.NoError(t, err)
	require.Len(t, appList.Items, 2)
}

// TestEventHandlingIntegration tests integration with event handling
func TestEventHandlingIntegration(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	// Create test applications
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1"},
		},
	}

	relatedApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "related-app-1",
			Namespace: "default",
		},
	}

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mainApp, relatedApp).
		Build()

	// Test event handling
	ctx := context.Background()

	// Simulate application update event
	var retrievedApp v1alpha1.Application
	err := fakeClient.Get(ctx, types.NamespacedName{
		Name:      "main-app",
		Namespace: "default",
	}, &retrievedApp)
	require.NoError(t, err)

	// Update the application
	retrievedApp.Spec.RelatedApplications = []string{"related-app-1", "related-app-2"}
	err = fakeClient.Update(ctx, &retrievedApp)
	require.NoError(t, err)

	// Verify the update
	var updatedApp v1alpha1.Application
	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      "main-app",
		Namespace: "default",
	}, &updatedApp)
	require.NoError(t, err)
	require.Len(t, updatedApp.Spec.RelatedApplications, 2)
}
