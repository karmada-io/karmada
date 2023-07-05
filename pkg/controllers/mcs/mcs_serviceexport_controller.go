package mcs

import (
	"context"
	"fmt"
	"strings"

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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
)

const (
	// MultiClusterServiceControllerName MultiClusterServiceController name.
	MultiClusterServiceControllerName = "mcs-serviceexport-controller"

	// karmadaMCSFinalizer is added to MultiClusterService to ensure downstream resource,
	// such as ServiceExport and ServiceImport are deleted before itself is deleted.
	karmadaMCSFinalizer = "karmada.io/karmada-mcs-controller"
)

// MultiClusterServiceController lists/watches the multiClusterService resource,
// generates ServiceExport and propagate it to the source clusters.
// Note: This controller only focus on  multiClusterService of type CrossCluster.
type MultiClusterServiceController struct {
	client.Client
	EventRecorder      record.EventRecorder
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
func (c *MultiClusterServiceController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling MultiClusterService %s.", req.NamespacedName.String())

	mcs := &networkingv1alpha1.MultiClusterService{}
	if err := c.Client.Get(ctx, req.NamespacedName, mcs); err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{Requeue: true}, err
	}

	if err := c.reconcileServiceExport(ctx, mcs); err != nil {
		klog.Errorf("Failed to handle ServiceExport for MultiClusterService(%s), error: %v", req.NamespacedName.String(), err)
		return controllerruntime.Result{}, err
	}
	if !mcs.DeletionTimestamp.IsZero() || !mcs.Spec.HasExposureTypeCrossCluster() {
		return controllerruntime.Result{}, c.removeFinalizer(ctx, mcs)
	}
	return controllerruntime.Result{}, c.ensureFinalizer(ctx, mcs)
}

func (c *MultiClusterServiceController) deriveServiceExportFromMultiClusterService(
	ctx context.Context, mcs *networkingv1alpha1.MultiClusterService, ownSvc *corev1.Service) error {
	oldSvcExport := &mcsv1alpha1.ServiceExport{}
	err := c.Client.Get(ctx, types.NamespacedName{Name: mcs.Name, Namespace: mcs.Namespace}, oldSvcExport)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		newSvcExport := &mcsv1alpha1.ServiceExport{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: mcs.Namespace,
				Name:      mcs.Name,
				Annotations: map[string]string{
					networkingv1alpha1.ResourceKindMultiMultiClusterServiceUsedBy: string(networkingv1alpha1.ExposureTypeCrossCluster),
				},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(ownSvc, corev1.SchemeGroupVersion.WithKind("Service")),
				},
			},
		}
		err = c.Client.Create(ctx, newSvcExport)
		if err != nil {
			msg := fmt.Sprintf("Create derived ServiceExport(%s/%s) failed, Error: %v", newSvcExport.Namespace, newSvcExport.Name, err)
			klog.Errorf(msg)
			c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventSyncDerivedServiceExportFailed, msg)
			return err
		}
		msg := fmt.Sprintf("Sync the serviceExport of MultiClusterService(%s/%s) successful", mcs.Namespace, mcs.Name)
		klog.V(4).Infof(msg)
		c.EventRecorder.Eventf(mcs, corev1.EventTypeNormal, events.EventSyncDerivedServiceExportSucceed, msg)
		return nil
	}
	if addCrossClusterToUsedByAnnotationIfNeed(oldSvcExport) {
		oldSvcExport.OwnerReferences = append(oldSvcExport.OwnerReferences, *metav1.NewControllerRef(ownSvc, corev1.SchemeGroupVersion.WithKind("Service")))
		err = c.Client.Update(ctx, oldSvcExport)
		if err != nil {
			msg := fmt.Sprintf("Update derived ServiceExport(%s/%s) failed, Error: %v", oldSvcExport.Namespace, oldSvcExport.Name, err)
			klog.Errorf(msg)
			c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventSyncDerivedServiceExportFailed, msg)
			return err
		}
		msg := fmt.Sprintf("Sync the serviceExport of MultiClusterService(%s/%s) successful", mcs.Namespace, mcs.Name)
		klog.V(4).Infof(msg)
		c.EventRecorder.Eventf(mcs, corev1.EventTypeNormal, events.EventSyncDerivedServiceExportSucceed, msg)
		return nil
	}
	return nil
}

func (c *MultiClusterServiceController) cleanupDerivedServiceExport(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	derivedSvcExport := &mcsv1alpha1.ServiceExport{}
	derivedSvcExportNamespacedName := types.NamespacedName{
		Namespace: mcs.Namespace,
		Name:      mcs.Name,
	}
	err := c.Client.Get(ctx, derivedSvcExportNamespacedName, derivedSvcExport)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if removeCrossClusterToUsedByAnnotationIfNeed(derivedSvcExport) {
		err = c.Client.Update(ctx, derivedSvcExport)
	}
	used := derivedSvcExport.Annotations[networkingv1alpha1.ResourceKindMultiMultiClusterServiceUsedBy]
	if len(used) == 0 {
		err = c.Client.Delete(ctx, derivedSvcExport)
	}
	if err != nil && !apierrors.IsNotFound(err) {
		klog.ErrorS(err, "failed to delete ServiceExport or remove multiclusterservices.networking.karmada.io/used-by annotation of the ServiceExport",
			"namespacedName", derivedSvcExportNamespacedName.String(), "used-by", used)
		return err
	}
	klog.V(4).InfoS("success to delete ServiceExport or  remove multiclusterservices.networking.karmada.io/used-by annotation of the ServiceExport",
		"namespacedName", derivedSvcExportNamespacedName.String(), "used-by", used)
	return nil
}

func (c *MultiClusterServiceController) reconcileServiceExport(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	if !mcs.Spec.HasExposureTypeCrossCluster() || !mcs.DeletionTimestamp.IsZero() {
		return c.cleanupDerivedServiceExport(ctx, mcs)
	}
	ownerSvc := &corev1.Service{}
	err := c.Client.Get(ctx, types.NamespacedName{Name: mcs.Name, Namespace: mcs.Namespace}, ownerSvc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return c.deriveServiceExportFromMultiClusterService(ctx, mcs, ownerSvc)
}

// SetupWithManager creates a controller and register to controller manager.
func (c *MultiClusterServiceController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.MultiClusterService{}).
		Watches(&corev1.Service{}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(createEvent event.CreateEvent) bool { return true },
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool { return false },
			UpdateFunc: func(updateEvent event.UpdateEvent) bool { return false },
		})).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions),
		}).
		Complete(c)
}

// removeFinalizer remove finalizer from the given MultiClusterService
func (c *MultiClusterServiceController) removeFinalizer(ctx context.Context, obj client.Object) error {
	if !controllerutil.ContainsFinalizer(obj, karmadaMCSFinalizer) {
		return nil
	}
	controllerutil.RemoveFinalizer(obj, karmadaMCSFinalizer)
	return c.Client.Update(ctx, obj)
}

// ensureFinalizer add finalizer from the given MultiClusterService
func (c *MultiClusterServiceController) ensureFinalizer(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	if controllerutil.ContainsFinalizer(mcs, karmadaMCSFinalizer) {
		return nil
	}
	controllerutil.AddFinalizer(mcs, karmadaMCSFinalizer)
	return c.Client.Update(ctx, mcs)
}

func removeCrossClusterToUsedByAnnotationIfNeed(se *mcsv1alpha1.ServiceExport) bool {
	if se.Annotations == nil {
		return false
	}
	used := se.Annotations[networkingv1alpha1.ResourceKindMultiMultiClusterServiceUsedBy]
	usedTypes := strings.Split(used, ",")
	for i, usedType := range usedTypes {
		if usedType == string(networkingv1alpha1.ExposureTypeCrossCluster) {
			se.Annotations[networkingv1alpha1.ResourceKindMultiMultiClusterServiceUsedBy] = strings.Join(append(usedTypes[:i], usedTypes[i+1:]...), ",")
			return true
		}
	}

	return false
}

func addCrossClusterToUsedByAnnotationIfNeed(se *mcsv1alpha1.ServiceExport) bool {
	if se.Annotations == nil {
		se.Annotations = map[string]string{}
	}
	used := se.Annotations[networkingv1alpha1.ResourceKindMultiMultiClusterServiceUsedBy]
	usedTypes := strings.Split(used, ",")
	for _, usedType := range usedTypes {
		if usedType == string(networkingv1alpha1.ExposureTypeCrossCluster) {
			return false
		}
	}
	se.Annotations[networkingv1alpha1.ResourceKindMultiMultiClusterServiceUsedBy] = strings.Trim(strings.Join([]string{string(networkingv1alpha1.ExposureTypeCrossCluster), used}, ","), ",")
	return true
}
