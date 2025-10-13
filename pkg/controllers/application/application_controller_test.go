package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
)

func TestApplicationFailoverWithRelatedApps(t *testing.T) {
	// Setup main application with related applications
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

	apps := map[string]*v1alpha1.Application{
		"default/main-app":      mainApp,
		"default/related-app-1": relatedApp1,
		"default/related-app-2": relatedApp2,
	}

	migrated := map[string]bool{}

	reconciler := &ApplicationReconciler{
		GetFunc: func(ctx context.Context, key types.NamespacedName, obj interface{}) error {
			app, ok := apps[key.Namespace+"/"+key.Name]
			if !ok {
				return errors.NewNotFound(v1alpha1.Resource("applications"), key.Name)
			}
			*(obj.(*v1alpha1.Application)) = *app
			return nil
		},
		MigrateFunc: func(app *v1alpha1.Application) error {
			migrated[app.Name] = true
			return nil
		},
	}

	// Simulate failover
	err := reconciler.handleFailover(context.Background(), mainApp)
	require.NoError(t, err)

	// Check that related applications were migrated
	require.True(t, migrated["related-app-1"])
	require.True(t, migrated["related-app-2"])
}

func TestApplicationFailoverWithMissingRelatedApp(t *testing.T) {
	// Setup main application with a missing related application
	mainApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "main-app",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			RelatedApplications: []string{"existing-app", "missing-app"},
		},
	}
	existingApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-app",
			Namespace: "default",
		},
	}

	apps := map[string]*v1alpha1.Application{
		"default/main-app":     mainApp,
		"default/existing-app": existingApp,
		// missing-app is intentionally not in the map
	}

	migrated := map[string]bool{}

	reconciler := &ApplicationReconciler{
		GetFunc: func(ctx context.Context, key types.NamespacedName, obj interface{}) error {
			app, ok := apps[key.Namespace+"/"+key.Name]
			if !ok {
				return errors.NewNotFound(v1alpha1.Resource("applications"), key.Name)
			}
			*(obj.(*v1alpha1.Application)) = *app
			return nil
		},
		MigrateFunc: func(app *v1alpha1.Application) error {
			migrated[app.Name] = true
			return nil
		},
	}

	// Simulate failover
	err := reconciler.handleFailover(context.Background(), mainApp)
	require.NoError(t, err)

	// Check that only the existing related application was migrated
	require.True(t, migrated["existing-app"])
	require.False(t, migrated["missing-app"])
}
