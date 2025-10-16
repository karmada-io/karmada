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

// TestKubernetesAPICreation tests Kubernetes API creation operations
func TestKubernetesAPICreation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	ctx := context.Background()

	// Test creating a new Application
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1"},
		},
	}

	err := fakeClient.Create(ctx, app)
	require.NoError(t, err)

	// Verify the application was created
	var retrievedApp v1alpha1.Application
	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      "test-app",
		Namespace: "default",
	}, &retrievedApp)
	require.NoError(t, err)
	require.Equal(t, "test-app", retrievedApp.Name)
	require.Len(t, retrievedApp.Spec.RelatedApplications, 1)
}

// TestKubernetesAPIUpdate tests Kubernetes API update operations
func TestKubernetesAPIUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(app).
		Build()

	ctx := context.Background()

	// Test updating the application
	var retrievedApp v1alpha1.Application
	err := fakeClient.Get(ctx, types.NamespacedName{
		Name:      "test-app",
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
		Name:      "test-app",
		Namespace: "default",
	}, &updatedApp)
	require.NoError(t, err)
	require.Len(t, updatedApp.Spec.RelatedApplications, 2)
}

// TestKubernetesAPIDeletion tests Kubernetes API deletion operations
func TestKubernetesAPIDeletion(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(app).
		Build()

	ctx := context.Background()

	// Test deleting the application
	err := fakeClient.Delete(ctx, app)
	require.NoError(t, err)

	// Verify the application was deleted
	var deletedApp v1alpha1.Application
	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      "test-app",
		Namespace: "default",
	}, &deletedApp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// TestKubernetesAPIList tests Kubernetes API list operations
func TestKubernetesAPIList(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	apps := []client.Object{
		&v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-1",
				Namespace: "default",
			},
			Spec: v1alpha1.ApplicationSpec{
				RelatedApplications: []string{"related-1"},
			},
		},
		&v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-2",
				Namespace: "default",
			},
			Spec: v1alpha1.ApplicationSpec{
				RelatedApplications: []string{"related-2"},
			},
		},
		&v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-3",
				Namespace: "test",
			},
			Spec: v1alpha1.ApplicationSpec{
				RelatedApplications: []string{"related-3"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(apps...).
		Build()

	ctx := context.Background()

	// Test listing all applications
	var appList v1alpha1.ApplicationList
	err := fakeClient.List(ctx, &appList)
	require.NoError(t, err)
	require.Len(t, appList.Items, 3)

	// Test listing applications in specific namespace
	var defaultAppList v1alpha1.ApplicationList
	err = fakeClient.List(ctx, &defaultAppList, client.InNamespace("default"))
	require.NoError(t, err)
	require.Len(t, defaultAppList.Items, 2)

	// Test listing applications with label selector
	var labeledAppList v1alpha1.ApplicationList
	err = fakeClient.List(ctx, &labeledAppList, client.MatchingLabels{"app": "test"})
	require.NoError(t, err)
	require.Len(t, labeledAppList.Items, 0) // No apps with this label
}

// TestKubernetesAPIWatch tests Kubernetes API watch operations
func TestKubernetesAPIWatch(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(app).
		Build()

	ctx := context.Background()

	// Test watching applications
	watcher, err := fakeClient.Watch(ctx, &v1alpha1.ApplicationList{})
	require.NoError(t, err)
	require.NotNil(t, watcher)

	// Close the watcher
	watcher.Stop()
}

// TestKubernetesAPIPatch tests Kubernetes API patch operations
func TestKubernetesAPIPatch(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(app).
		Build()

	ctx := context.Background()

	// Test patching the application
	patch := client.MergeFrom(app.DeepCopy())
	app.Spec.RelatedApplications = []string{"related-app-1", "related-app-2"}
	
	err := fakeClient.Patch(ctx, app, patch)
	require.NoError(t, err)

	// Verify the patch
	var patchedApp v1alpha1.Application
	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      "test-app",
		Namespace: "default",
	}, &patchedApp)
	require.NoError(t, err)
	require.Len(t, patchedApp.Spec.RelatedApplications, 2)
}

// TestKubernetesAPIStatusUpdate tests Kubernetes API status update operations
func TestKubernetesAPIStatusUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(app).
		Build()

	ctx := context.Background()

	// Test status update
	var retrievedApp v1alpha1.Application
	err := fakeClient.Get(ctx, types.NamespacedName{
		Name:      "test-app",
		Namespace: "default",
	}, &retrievedApp)
	require.NoError(t, err)

	// Update status (simulate)
	retrievedApp.Status = v1alpha1.ApplicationStatus{
		Phase: "Running",
	}
	
	err = fakeClient.Status().Update(ctx, &retrievedApp)
	require.NoError(t, err)

	// Verify the status update
	var statusUpdatedApp v1alpha1.Application
	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      "test-app",
		Namespace: "default",
	}, &statusUpdatedApp)
	require.NoError(t, err)
	require.Equal(t, "Running", statusUpdatedApp.Status.Phase)
}
