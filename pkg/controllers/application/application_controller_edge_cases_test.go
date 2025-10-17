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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
)

// TestEmptyRelatedApplications tests edge case with empty related applications
func TestEmptyRelatedApplications(t *testing.T) {
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{}, // Empty slice
		},
	}

	// Mock functions
	migrated := make(map[string]bool)
	migrateFunc := func(ctx context.Context, app *v1alpha1.Application) error {
		migrated[app.Name] = true
		return nil
	}

	getFunc := func(ctx context.Context, key types.NamespacedName, obj interface{}) error {
		return errors.NewNotFound(v1alpha1.Resource("applications"), key.Name)
	}

	reconciler := &ApplicationReconciler{
		GetFunc:     getFunc,
		MigrateFunc: migrateFunc,
	}

	err := reconciler.handleFailover(context.Background(), mainApp)
	require.NoError(t, err)
	require.True(t, migrated["main-app"])
	require.Len(t, migrated, 1) // Only main app should be migrated
}

// TestMaxRelatedApplications tests edge case with maximum number of related applications
func TestMaxRelatedApplications(t *testing.T) {
	// Create a large number of related applications (1000)
	relatedApps := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		relatedApps[i] = fmt.Sprintf("related-app-%d", i)
	}

	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: relatedApps,
		},
	}

	// Mock functions
	migrated := make(map[string]bool)
	migrateFunc := func(ctx context.Context, app *v1alpha1.Application) error {
		migrated[app.Name] = true
		return nil
	}

	getFunc := func(ctx context.Context, key types.NamespacedName, obj interface{}) error {
		// Simulate that all related apps exist
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
		}
		*(obj.(*v1alpha1.Application)) = *app
		return nil
	}

	reconciler := &ApplicationReconciler{
		GetFunc:     getFunc,
		MigrateFunc: migrateFunc,
	}

	err := reconciler.handleFailover(context.Background(), mainApp)
	require.NoError(t, err)
	require.True(t, migrated["main-app"])
	require.Len(t, migrated, 1001) // Main app + 1000 related apps
}

// TestInvalidApplicationNames tests edge case with invalid application names
func TestInvalidApplicationNames(t *testing.T) {
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{
				"",                    // Empty name
				"invalid@name",        // Invalid characters
				"very-long-name-that-exceeds-kubernetes-limits-and-should-be-invalid", // Too long
				"valid-app",           // Valid name
			},
		},
	}

	// Mock functions
	migrated := make(map[string]bool)
	migrateFunc := func(ctx context.Context, app *v1alpha1.Application) error {
		migrated[app.Name] = true
		return nil
	}

	getFunc := func(ctx context.Context, key types.NamespacedName, obj interface{}) error {
		// Only valid-app should be found
		if key.Name == "valid-app" {
			app := &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
			}
			*(obj.(*v1alpha1.Application)) = *app
			return nil
		}
		return errors.NewNotFound(v1alpha1.Resource("applications"), key.Name)
	}

	reconciler := &ApplicationReconciler{
		GetFunc:     getFunc,
		MigrateFunc: migrateFunc,
	}

	err := reconciler.handleFailover(context.Background(), mainApp)
	require.NoError(t, err)
	require.True(t, migrated["main-app"])
	require.True(t, migrated["valid-app"])
	require.Len(t, migrated, 2) // Only main app and valid related app
}

// TestNamespaceBoundaryConditions tests edge case with namespace boundary conditions
func TestNamespaceBoundaryConditions(t *testing.T) {
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1", "related-app-2"},
		},
	}

	// Mock functions
	migrated := make(map[string]bool)
	migrateFunc := func(ctx context.Context, app *v1alpha1.Application) error {
		migrated[app.Name] = true
		return nil
	}

	getFunc := func(ctx context.Context, key types.NamespacedName, obj interface{}) error {
		// Only return apps in the same namespace
		if key.Namespace == "default" {
			app := &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
			}
			*(obj.(*v1alpha1.Application)) = *app
			return nil
		}
		return errors.NewNotFound(v1alpha1.Resource("applications"), key.Name)
	}

	reconciler := &ApplicationReconciler{
		GetFunc:     getFunc,
		MigrateFunc: migrateFunc,
	}

	err := reconciler.handleFailover(context.Background(), mainApp)
	require.NoError(t, err)
	require.True(t, migrated["main-app"])
	require.True(t, migrated["related-app-1"])
	require.True(t, migrated["related-app-2"])
	require.Len(t, migrated, 3)
}

// TestContextTimeoutScenarios tests edge case with context timeout
func TestContextTimeoutScenarios(t *testing.T) {
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1"},
		},
	}

	// Mock functions
	migrated := make(map[string]bool)
	migrateFunc := func(ctx context.Context, app *v1alpha1.Application) error {
		// Simulate slow migration that might timeout
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			migrated[app.Name] = true
			return nil
		}
	}

	getFunc := func(ctx context.Context, key types.NamespacedName, obj interface{}) error {
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
		}
		*(obj.(*v1alpha1.Application)) = *app
		return nil
	}

	reconciler := &ApplicationReconciler{
		GetFunc:     getFunc,
		MigrateFunc: migrateFunc,
	}

	// Test with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := reconciler.handleFailover(ctx, mainApp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "context deadline exceeded")
}

// TestResourceQuotaExceeded tests edge case with resource quota exceeded
func TestResourceQuotaExceeded(t *testing.T) {
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1"},
		},
	}

	// Mock functions
	migrated := make(map[string]bool)
	migrateFunc := func(ctx context.Context, app *v1alpha1.Application) error {
		if app.Name == "related-app-1" {
			return errors.NewResourceQuotaExceeded("resource quota exceeded")
		}
		migrated[app.Name] = true
		return nil
	}

	getFunc := func(ctx context.Context, key types.NamespacedName, obj interface{}) error {
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
		}
		*(obj.(*v1alpha1.Application)) = *app
		return nil
	}

	reconciler := &ApplicationReconciler{
		GetFunc:     getFunc,
		MigrateFunc: migrateFunc,
	}

	err := reconciler.handleFailover(context.Background(), mainApp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "resource quota exceeded")
	require.True(t, migrated["main-app"]) // Main app should still be migrated
	require.False(t, migrated["related-app-1"]) // Related app should fail
}

// TestConcurrentFailoverOperations tests edge case with concurrent failover operations
func TestConcurrentFailoverOperations(t *testing.T) {
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1"},
		},
	}

	// Mock functions with concurrency control
	migrated := make(map[string]bool)
	var mu sync.Mutex
	migrateFunc := func(ctx context.Context, app *v1alpha1.Application) error {
		mu.Lock()
		defer mu.Unlock()
		migrated[app.Name] = true
		return nil
	}

	getFunc := func(ctx context.Context, key types.NamespacedName, obj interface{}) error {
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
		}
		*(obj.(*v1alpha1.Application)) = *app
		return nil
	}

	reconciler := &ApplicationReconciler{
		GetFunc:     getFunc,
		MigrateFunc: migrateFunc,
	}

	// Run multiple concurrent failover operations
	var wg sync.WaitGroup
	numConcurrent := 10
	errors := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := reconciler.handleFailover(context.Background(), mainApp)
			errors <- err
		}()
	}

	wg.Wait()
	close(errors)

	// Check that all operations completed successfully
	for err := range errors {
		require.NoError(t, err)
	}

	// Check that migration happened (may be multiple times due to concurrency)
	require.True(t, migrated["main-app"])
	require.True(t, migrated["related-app-1"])
}
