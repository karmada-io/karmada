package karmada

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName = "karmada-operator-controller"

	// ControllerFinalizerName is the name of the karmada controller finalizer
	ControllerFinalizerName = "operator.karmada.io/finalizer"
)

// Controller controls the Karmada resource.
type Controller struct {
	client.Client
	Config        *rest.Config
	EventRecorder record.EventRecorder
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (ctrl *Controller) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing karmada", "karmada", req, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing karmada", "karmada", req, "duration", time.Since(startTime))
	}()

	karmada := &operatorv1alpha1.Karmada{}
	if err := ctrl.Get(ctx, req.NamespacedName, karmada); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			klog.V(2).InfoS("Karmada has been deleted", "karmada", req)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if karmada.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(karmada, ControllerFinalizerName) {
			controllerutil.AddFinalizer(karmada, ControllerFinalizerName)
			if err := ctrl.Update(ctx, karmada); err != nil {
				return controllerruntime.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(karmada, ControllerFinalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := ctrl.deleteUnableGCResources(karmada); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return controllerruntime.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(karmada, ControllerFinalizerName)
			if err := ctrl.Update(ctx, karmada); err != nil {
				return controllerruntime.Result{}, err
			}
			// Stop reconciliation as the item is being deleted
			return controllerruntime.Result{}, nil
		}
	}

	klog.V(2).InfoS("Reconciling karmada", "name", req.Name)

	planner, err := NewPlannerFor(karmada, ctrl.Client, ctrl.Config)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	return planner.Execute()
}

func (ctrl *Controller) deleteUnableGCResources(karmada *operatorv1alpha1.Karmada) error {
	klog.InfoS("Deleting unable gc resources", "karmada", klog.KObj(karmada))
	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (ctrl *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&operatorv1alpha1.Karmada{}).Complete(ctrl)
}
