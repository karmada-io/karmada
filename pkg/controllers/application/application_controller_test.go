package application

import (
	"context"
	"testing"

	"github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
				return nil
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
	err := reconciler.handleFailover(mainApp)
	require.NoError(t, err)

	// Check that related applications were migrated
	require.True(t, migrated["related-app-1"])
	require.True(t, migrated["related-app-2"])
}
