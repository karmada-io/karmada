package mcs

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// MultiClusterServiceControllerName is the controller name that will be used when reporting events.
const MultiClusterServiceControllerName = "multiclusterservice-controller"

// karmadaMCSFinalizer is added to MultiClusterService to ensure downstream resource,
// such as ServiceExport and ServiceImport are deleted before itself is deleted.
const karmadaMCSFinalizer = "karmada.io/karmada-mcs-controller"

// MultiClusterServiceController is to sync MultiClusterService object resource and processed with
// ServiceExport and ServiceImport resources, enable multi-cluster service discovery.
type MultiClusterServiceController struct {
	client.Client
	EventRecorder      record.EventRecorder
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
func (c *MultiClusterServiceController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconcile MultiClusterService %s", req.NamespacedName.String())

	mcs := &networkingv1alpha1.MultiClusterService{}
	if err := c.Client.Get(ctx, req.NamespacedName, mcs); err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	if !mcs.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, c.reconcileWithMCSDelete(ctx, mcs)
	}
	return controllerruntime.Result{}, c.reconcileWithMCSCreateOrUpdate(ctx, mcs)
}

// SetupWithManager creates a controller and register to controller manager.
func (c *MultiClusterServiceController) SetupWithManager(mgr controllerruntime.Manager) error {
	mcsPredicateFn := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			mcs := createEvent.Object.(*networkingv1alpha1.MultiClusterService)
			return mcs.Spec.ContainsCrossClusterExposureType()
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			oldObj := updateEvent.ObjectOld.(*networkingv1alpha1.MultiClusterService)
			newObj := updateEvent.ObjectNew.(*networkingv1alpha1.MultiClusterService)
			return oldObj.Spec.ContainsCrossClusterExposureType() ||
				newObj.Spec.ContainsCrossClusterExposureType()
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			mcs := deleteEvent.Object.(*networkingv1alpha1.MultiClusterService)
			return mcs.Spec.ContainsCrossClusterExposureType()
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.MultiClusterService{}, builder.WithPredicates(mcsPredicateFn)).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions)}).
		Complete(c)
}

func (c *MultiClusterServiceController) reconcileWithMCSCreateOrUpdate(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	// 1. generate ServiceExport and propagate it to cluster
	err := c.ensureServiceExport(ctx, mcs)
	if err != nil {
		return err
	}

	// 2. add own finalizer to MultiClusterService object
	if controllerutil.AddFinalizer(mcs, karmadaMCSFinalizer) {
		err = c.Client.Update(ctx, mcs)
		if err != nil {
			klog.ErrorS(err, "failed to update MultiCluster with finalizer",
				"namespace", mcs.Namespace, "name", mcs.Name, "finalizer", karmadaMCSFinalizer)
			return err
		}
	}

	return nil
}

func (c *MultiClusterServiceController) reconcileWithMCSDelete(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	// 1. remove ServiceExport in karmada control-plane
	err := c.removeServiceExport(ctx, mcs)
	if err != nil {
		return err
	}

	// 2. remove own finalizer on MultiClusterService object
	if controllerutil.RemoveFinalizer(mcs, karmadaMCSFinalizer) {
		err := c.Client.Update(ctx, mcs)
		if err != nil {
			klog.ErrorS(err, "failed to update MultiClusterService with finalizer",
				"namespace", mcs.Namespace, "name", mcs.Name, "finalizer", karmadaMCSFinalizer)
			return err
		}
	}
	return nil
}

func (c *MultiClusterServiceController) ensureServiceExport(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	svcNamespacedName := types.NamespacedName{Namespace: mcs.Namespace, Name: mcs.Name}

	// 1. make sure serviceExport exist
	svcExport := &mcsv1alpha1.ServiceExport{}
	err := c.Client.Get(ctx, svcNamespacedName, svcExport)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.ErrorS(err, "failed to get serviceExport",
			"namespacedName", svcNamespacedName.String())
		return err
	}

	// 2. if serviceExport not exist, just create it
	if apierrors.IsNotFound(err) {
		svc := &corev1.Service{}
		err = c.Get(ctx, svcNamespacedName, svc)
		if err != nil {
			klog.ErrorS(err, "failed to get target Service",
				"namespacedName", svcNamespacedName.String())
			return err
		}

		svcExport = createServiceExportTemplate(svc)
		err = c.Client.Create(ctx, svcExport)
		if err != nil {
			klog.ErrorS(err, "failed to create serviceExport",
				"namespacedName", svcNamespacedName.String())
			return err
		}
		klog.V(4).InfoS("success to create serviceExport",
			"namespacedName", svcNamespacedName.String())
	} else { // otherwise serviceExport already exist, update it with own finalizer
		if controllerutil.AddFinalizer(svcExport, karmadaMCSFinalizer) {
			err = c.Client.Update(ctx, svcExport)
			if err != nil {
				klog.ErrorS(err, "failed to update ServiceExport with finalizer",
					"namespacedName", svcNamespacedName.String(), "finalizer", karmadaMCSFinalizer)
				return err
			}
			klog.V(4).InfoS("success to update serviceExport with finalizer",
				"namespacedName", svcNamespacedName.String(), "finalizer", karmadaMCSFinalizer)
		}
	}

	// 3. get target service's reference resourceBinding
	refRB := &workv1alpha2.ResourceBinding{}
	rbNamespacedName := types.NamespacedName{
		Namespace: mcs.Namespace,
		Name:      names.GenerateBindingName(util.ServiceKind, mcs.Name),
	}
	err = c.Get(ctx, rbNamespacedName, refRB)
	if err != nil {
		klog.ErrorS(err, "failed to get Service's ref ResourceBinding",
			"namespacedName", rbNamespacedName.String())
		return err
	}

	// 4. make sure the ref resourceBinding's propagateDeps is enabled
	if refRB.Spec.PropagateDeps {
		return nil
	}
	refRB.Spec.PropagateDeps = true
	err = c.Update(ctx, refRB)
	if err != nil {
		klog.ErrorS(err, "failed to update Service's ref ResourceBinding",
			"namespacedName", rbNamespacedName.String())
		return err
	}

	klog.V(4).InfoS("Success to ensure ServiceExport",
		"namespacedName", svcNamespacedName.String())
	return nil
}

func (c *MultiClusterServiceController) removeServiceExport(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	svcNamespacedName := types.NamespacedName{Namespace: mcs.Namespace, Name: mcs.Name}

	// 1. get target ServiceExport
	svcExport := &mcsv1alpha1.ServiceExport{}
	err := c.Client.Get(ctx, svcNamespacedName, svcExport)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.ErrorS(err, "failed to get serviceExport",
			"namespacedName", svcNamespacedName.String())
		return err
	}

	// 2. remove karmadaMCSFinalizer first ant remove it if there is no finalizer
	var deleted bool
	if controllerutil.RemoveFinalizer(svcExport, karmadaMCSFinalizer) {
		if len(svcExport.Finalizers) == 0 {
			err = c.Client.Delete(ctx, svcExport)
			deleted = true
		} else {
			err = c.Client.Update(ctx, svcExport)
		}
		if err != nil {
			klog.ErrorS(err, "failed to delete ServiceExport or remove ServiceExport finalizer",
				"namespacedName", svcNamespacedName.String(), "finalizer", karmadaMCSFinalizer)
			return err
		}
		klog.V(4).InfoS("success to delete ServiceExport or remove ServiceExport finalizer",
			"namespacedName", svcNamespacedName.String(), "finalizer", karmadaMCSFinalizer)
	}

	// 3. get target service's reference resourceBinding and disable resourceBinding propagateDeps
	if deleted {
		refRB := &workv1alpha2.ResourceBinding{}
		rbNamespacedName := types.NamespacedName{
			Namespace: mcs.Namespace,
			Name:      names.GenerateBindingName(util.ServiceKind, mcs.Name),
		}
		err = c.Get(ctx, rbNamespacedName, refRB)
		if err != nil {
			klog.ErrorS(err, "failed to get Service's ref ResourceBinding",
				"namespacedName", rbNamespacedName.String())
			return err
		}

		if !refRB.Spec.PropagateDeps {
			return nil
		}
		refRB.Spec.PropagateDeps = false
		err = c.Update(ctx, refRB)
		if err != nil {
			klog.ErrorS(err, "failed to update Service's ref ResourceBinding",
				"namespacedName", rbNamespacedName.String())
			return err
		}
	}

	klog.V(4).InfoS("success to eliminate the impact on ServiceExport with karmada controller",
		"namespacedName", svcNamespacedName.String())
	return nil
}

func createServiceExportTemplate(svc *corev1.Service) *mcsv1alpha1.ServiceExport {
	return &mcsv1alpha1.ServiceExport{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: svc.Namespace,
			Name:      svc.Name,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(svc, corev1.SchemeGroupVersion.WithKind("Service")),
			},
			Finalizers: []string{karmadaMCSFinalizer},
		},
	}
}
