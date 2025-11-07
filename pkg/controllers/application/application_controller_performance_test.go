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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
)

// BenchmarkApplicationFailoverPerformance benchmarks the performance of application failover
func BenchmarkApplicationFailoverPerformance(b *testing.B) {
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1", "related-app-2", "related-app-3"},
		},
	}

	// Mock functions
	migrateFunc := func(app *v1alpha1.Application) error {
		// Simulate migration work
		time.Sleep(1 * time.Millisecond)
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

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := reconciler.handleFailover(context.Background(), mainApp)
			require.NoError(b, err)
		}
	})
}

// BenchmarkRelatedApplicationsMigrationPerformance benchmarks the performance of related applications migration
func BenchmarkRelatedApplicationsMigrationPerformance(b *testing.B) {
	// Create a large number of related applications for performance testing
	relatedApps := make([]string, 100)
	for i := 0; i < 100; i++ {
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
	migrateFunc := func(app *v1alpha1.Application) error {
		// Simulate migration work
		time.Sleep(100 * time.Microsecond)
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

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := reconciler.handleFailover(context.Background(), mainApp)
			require.NoError(b, err)
		}
	})
}

// BenchmarkControllerReconciliationPerformance benchmarks the performance of controller reconciliation
func BenchmarkControllerReconciliationPerformance(b *testing.B) {
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1", "related-app-2"},
		},
	}

	// Mock functions with minimal overhead
	migrateFunc := func(app *v1alpha1.Application) error {
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

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := reconciler.handleFailover(context.Background(), mainApp)
			require.NoError(b, err)
		}
	})
}

// BenchmarkMemoryUsage benchmarks memory usage during failover operations
func BenchmarkMemoryUsage(b *testing.B) {
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"related-app-1", "related-app-2", "related-app-3"},
		},
	}

	// Mock functions
	migrateFunc := func(app *v1alpha1.Application) error {
		// Simulate memory allocation
		_ = make([]byte, 1024) // 1KB allocation
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

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := reconciler.handleFailover(context.Background(), mainApp)
			require.NoError(b, err)
		}
	})
}

// BenchmarkConcurrentFailoverPerformance benchmarks concurrent failover operations
func BenchmarkConcurrentFailoverPerformance(b *testing.B) {
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
	migrateFunc := func(app *v1alpha1.Application) error {
		time.Sleep(1 * time.Millisecond)
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

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := reconciler.handleFailover(context.Background(), mainApp)
			require.NoError(b, err)
		}
	})
}
