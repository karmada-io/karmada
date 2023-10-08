package karmada

import (
	"context"
	"reflect"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	operatorscheme "github.com/karmada-io/karmada/operator/pkg/scheme"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName = "karmada-operator-controller"

	// ControllerFinalizerName is the name of the karmada controller finalizer
	ControllerFinalizerName = "operator.karmada.io/finalizer"

	// DisableCascadingDeletionLabel is the label that determine whether to perform cascade deletion
	DisableCascadingDeletionLabel = "operator.karmada.io/disable-cascading-deletion"
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

	// The object is being deleted
	if !karmada.DeletionTimestamp.IsZero() {
		val, ok := karmada.Labels[DisableCascadingDeletionLabel]
		if !ok || val == strconv.FormatBool(false) {
			if err := ctrl.syncKarmada(karmada); err != nil {
				return controllerruntime.Result{}, err
			}
		}

		return ctrl.removeFinalizer(ctx, karmada)
	}

	if err := ctrl.ensureKarmada(ctx, karmada); err != nil {
		return controllerruntime.Result{}, err
	}

	return controllerruntime.Result{}, ctrl.syncKarmada(karmada)
}

func (ctrl *Controller) syncKarmada(karmada *operatorv1alpha1.Karmada) error {
	klog.V(2).InfoS("Reconciling karmada", "name", karmada.Name)
	planner, err := NewPlannerFor(karmada, ctrl.Client, ctrl.Config)
	if err != nil {
		return err
	}

	return planner.Execute()
}

func (ctrl *Controller) removeFinalizer(ctx context.Context, karmada *operatorv1alpha1.Karmada) (controllerruntime.Result, error) {
	return controllerruntime.Result{}, retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		newer := &operatorv1alpha1.Karmada{}
		if err := ctrl.Get(ctx, client.ObjectKeyFromObject(karmada), newer); err != nil {
			return err
		}

		if controllerutil.RemoveFinalizer(newer, ControllerFinalizerName) {
			return ctrl.Update(ctx, newer)
		}

		return nil
	})
}

func (ctrl *Controller) ensureKarmada(ctx context.Context, karmada *operatorv1alpha1.Karmada) error {
	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object. This is equivalent
	// registering our finalizer.
	updated := controllerutil.AddFinalizer(karmada, ControllerFinalizerName)
	if _, isExist := karmada.Labels[DisableCascadingDeletionLabel]; !isExist {
		labelMap := labels.Merge(karmada.GetLabels(), labels.Set{DisableCascadingDeletionLabel: "false"})
		karmada.SetLabels(labelMap)
		updated = true
	}

	older := karmada.DeepCopy()

	// Set the defaults for karmada
	operatorscheme.Scheme.Default(karmada)

	if updated || !reflect.DeepEqual(karmada.Spec, older.Spec) {
		return ctrl.Update(ctx, karmada)
	}

	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (ctrl *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Karmada{},
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: ctrl.onKarmadaUpdate,
			})).
		Complete(ctrl)
}

// onKarmadaUpdate returns a bool which represents whether the karmada events need to entry the queue.
// In order to avoid invalid reconcile, we just focus on these two cases:
// 1. when the karmada cr is deleted.
// 2. when the spec of karmada cr is updated.
func (ctrl *Controller) onKarmadaUpdate(updateEvent event.UpdateEvent) bool {
	newObj := updateEvent.ObjectNew.(*operatorv1alpha1.Karmada)
	oldObj := updateEvent.ObjectOld.(*operatorv1alpha1.Karmada)

	if !newObj.DeletionTimestamp.IsZero() {
		return true
	}

	return !reflect.DeepEqual(newObj.Spec, oldObj.Spec)
}
