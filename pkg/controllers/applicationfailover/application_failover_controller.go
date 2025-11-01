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

package applicationfailover

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
)

// ApplicationFailoverReconciler reconciles Application objects for failover
type ApplicationFailoverReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

// migrateApplication performs the actual migration of an application to a healthy cluster
func (r *ApplicationFailoverReconciler) migrateApplication(app *v1alpha1.Application) error {
	// Implementation of the application migration logic
	// This is where you would implement the actual migration logic
	klog.InfoS("Migrating application", "name", app.Name, "namespace", app.Namespace)

	// Example implementation (replace with actual migration logic):
	// 1. Find a healthy target cluster
	// 2. Update the application's cluster assignment
	// 3. Apply the changes

	return nil // Return nil for successful migration or an error if migration failed
}

// migrateApplicationAndRelated migrates the main application and all related applications
func (r *ApplicationFailoverReconciler) migrateApplicationAndRelated(ctx context.Context, app *v1alpha1.Application) error {
	// Migrate the main application
	err := r.migrateApplication(app)
	if err != nil {
		return fmt.Errorf("failed to migrate main application %s/%s: %w", app.Namespace, app.Name, err)
	}

	// Migrate related applications
	for _, relatedAppName := range app.Spec.RelatedApplications {
		relatedApp, err := r.getApplicationByName(ctx, relatedAppName, app.Namespace)
		if err != nil {
			klog.ErrorS(err, "Failed to get related application", "name", relatedAppName, "namespace", app.Namespace)
			continue
		}
		err = r.migrateApplication(relatedApp)
		if err != nil {
			klog.ErrorS(err, "Failed to migrate related application", "name", relatedAppName, "namespace", app.Namespace)
			continue
		}
		klog.InfoS("Successfully migrated related application", "name", relatedAppName, "namespace", app.Namespace)
	}
	return nil
}

// Reconcile handles the reconciliation logic for Application failover
func (r *ApplicationFailoverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.V(4).InfoS("Reconciling Application for failover", "namespace", req.Namespace, "name", req.Name)

	// Get the Application instance
	var app v1alpha1.Application
	err := r.Client.Get(ctx, req.NamespacedName, &app)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Check if failover is needed
	// This is a placeholder - implement your failover detection logic here
	needsFailover := true

	if needsFailover {
		// Perform failover by migrating the application and its related applications
		err = r.migrateApplicationAndRelated(ctx, &app)
		if err != nil {
			klog.ErrorS(err, "Failed to perform application failover", "name", app.Name, "namespace", app.Namespace)
			return ctrl.Result{}, err
		}

		klog.InfoS("Successfully performed application failover", "name", app.Name, "namespace", app.Namespace)
	}

	return ctrl.Result{}, nil
}

// getApplicationByName retrieves an Application by name in the given namespace
func (r *ApplicationFailoverReconciler) getApplicationByName(ctx context.Context, name, namespace string) (*v1alpha1.Application, error) {
	var app v1alpha1.Application
	err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &app)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *ApplicationFailoverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Application{}).
		Complete(r)
}
