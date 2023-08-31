package mcs

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
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

	if !mcs.DeletionTimestamp.IsZero() || !mcs.Spec.ContainsCrossClusterExposureType() {
		return controllerruntime.Result{}, c.reconcileWithMCSDelete(ctx, mcs)
	}
	return controllerruntime.Result{}, c.reconcileWithMCSCreateOrUpdate(ctx, mcs)
}

func (c *MultiClusterServiceController) clusterMapFunc() handler.MapFunc {
	return func(ctx context.Context, object client.Object) []reconcile.Request {
		var requests []reconcile.Request

		mcsList := &networkingv1alpha1.MultiClusterServiceList{}
		err := c.List(context.TODO(), mcsList)
		if err != nil {
			klog.ErrorS(err, "failed to list MultiClusterService")
			return nil
		}

		for index := range mcsList.Items {
			mcs := mcsList.Items[index]
			if mcs.Spec.ContainsCrossClusterExposureType() && mcs.Spec.Range == nil {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: mcs.Namespace,
					Name:      mcs.Name,
				}})
			}
		}
		return requests
	}
}

func (c *MultiClusterServiceController) serviceMapFunc() handler.MapFunc {
	return func(ctx context.Context, object client.Object) []reconcile.Request {
		var requests []reconcile.Request

		mcs := &networkingv1alpha1.MultiClusterService{}
		err := c.Get(context.TODO(), types.NamespacedName{Namespace: object.GetNamespace(), Name: object.GetName()}, mcs)
		if err != nil {
			klog.ErrorS(err, "failed to get MultiClusterService object",
				"Namespace", object.GetNamespace(), "Name", object.GetName())
			return nil
		}

		if mcs.Spec.ContainsCrossClusterExposureType() {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: mcs.Namespace,
					Name:      mcs.Name,
				}})
		}
		return requests
	}
}

func (c *MultiClusterServiceController) endpointSliceMapFunc() handler.MapFunc {
	return func(ctx context.Context, object client.Object) []reconcile.Request {
		var requests []reconcile.Request

		svcName, exist := object.GetLabels()[discoveryv1.LabelServiceName]
		if !exist {
			return nil
		}
		mcs := &networkingv1alpha1.MultiClusterService{}
		err := c.Get(context.TODO(), types.NamespacedName{Namespace: object.GetNamespace(), Name: svcName}, mcs)
		if err != nil {
			klog.ErrorS(err, "failed to get MultiClusterService object",
				"Namespace", object.GetNamespace(), "Name", svcName)
			return nil
		}

		if mcs.Spec.ContainsCrossClusterExposureType() {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: mcs.Namespace,
					Name:      mcs.Name,
				}})
		}
		return requests
	}
}

// SetupWithManager creates a controller and register to controller manager.
func (c *MultiClusterServiceController) SetupWithManager(mgr controllerruntime.Manager) error {
	mcsPredicateFn := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			mcs := createEvent.Object.(*networkingv1alpha1.MultiClusterService)
			return mcs.Spec.ContainsCrossClusterExposureType()
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			mcs := deleteEvent.Object.(*networkingv1alpha1.MultiClusterService)
			return mcs.Spec.ContainsCrossClusterExposureType()
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			oldObj := updateEvent.ObjectOld.(*networkingv1alpha1.MultiClusterService)
			newObj := updateEvent.ObjectNew.(*networkingv1alpha1.MultiClusterService)
			return oldObj.Spec.ContainsCrossClusterExposureType() ||
				newObj.Spec.ContainsCrossClusterExposureType()
		},
	}

	clusterPredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc:  func(_ event.CreateEvent) bool { return true },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		UpdateFunc:  func(_ event.UpdateEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	})

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.MultiClusterService{}, builder.WithPredicates(mcsPredicateFn)).
		Watches(&clusterv1alpha1.Cluster{}, handler.EnqueueRequestsFromMapFunc(c.clusterMapFunc()), clusterPredicate).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(c.serviceMapFunc())).
		Watches(&discoveryv1.EndpointSlice{}, handler.EnqueueRequestsFromMapFunc(c.endpointSliceMapFunc())).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions)}).
		Complete(c)
}

func (c *MultiClusterServiceController) reconcileWithMCSCreateOrUpdate(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
) error {
	// 1. generate ServiceExport and propagate it to clusters
	err := c.ensureExportService(ctx, mcs)
	if err != nil {
		return err
	}

	// 2. import Service and EndpointSlice into destination clusters
	err = c.ensureImportService(ctx, mcs)
	if err != nil {
		return err
	}

	// 3. add own finalizer to MultiClusterService object
	if controllerutil.AddFinalizer(mcs, karmadaMCSFinalizer) {
		err = c.Client.Update(ctx, mcs)
		if err != nil {
			klog.ErrorS(err, "failed to update MultiClusterService object with finalizer",
				"namespace", mcs.Namespace, "name", mcs.Name, "finalizer", karmadaMCSFinalizer)
			return err
		}
	}
	return nil
}

func (c *MultiClusterServiceController) ensureExportService(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	namespacedName := types.NamespacedName{Namespace: mcs.Namespace, Name: mcs.Name}

	// 1. make sure serviceExport exist
	svcExport := &mcsv1alpha1.ServiceExport{}
	err := c.Client.Get(ctx, namespacedName, svcExport)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.ErrorS(err, "failed to get ServiceExport object",
			"namespacedName", namespacedName.String())
		return err
	}

	// 2. if serviceExport not exist, just create it
	if apierrors.IsNotFound(err) {
		svc := &corev1.Service{}
		err = c.Get(ctx, namespacedName, svc)
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.V(4).InfoS("target Service object doesn't exist",
					"namespacedName", namespacedName.String())
				return nil
			}
			klog.ErrorS(err, "failed to get target Service object",
				"namespacedName", namespacedName.String())
			return err
		}

		svcExport = createServiceExportTemplate(svc)
		err = c.Client.Create(ctx, svcExport)
		if err != nil {
			klog.ErrorS(err, "failed to create ServiceExport object",
				"namespacedName", namespacedName.String())
			return err
		}
		klog.V(4).InfoS("success to create ServiceExport object",
			"namespacedName", namespacedName.String())
	}

	// 3. otherwise serviceExport already exist, update it with own finalizer
	if controllerutil.AddFinalizer(svcExport, karmadaMCSFinalizer) {
		err = c.Client.Update(ctx, svcExport)
		if err != nil {
			klog.ErrorS(err, "failed to update ServiceExport object with finalizer",
				"namespacedName", namespacedName.String(), "finalizer", karmadaMCSFinalizer)
			return err
		}
		klog.V(4).InfoS("success to update ServiceExport object with finalizer",
			"namespacedName", namespacedName.String(), "finalizer", karmadaMCSFinalizer)
	}

	klog.V(4).InfoS("Success to ensure ServiceExport object",
		"namespacedName", namespacedName.String())
	return nil
}

func (c *MultiClusterServiceController) ensureImportService(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	destinationClusters, err := c.destinationClustersWithMCS(ctx, mcs)
	if err != nil {
		return err
	}

	if strategyWithNativeService(mcs) {
		return c.ensureWithNativeServiceMethod(ctx, mcs, destinationClusters)
	}
	return c.ensureWithServiceImportMethod(ctx, mcs, destinationClusters)
}

func (c *MultiClusterServiceController) ensureWithNativeServiceMethod(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
	destinationClusters []string,
) error {
	// 1. get the target Service object of MultiClusterService object.
	svcNamespacedName := types.NamespacedName{Namespace: mcs.Namespace, Name: mcs.Name}
	svc := &corev1.Service{}
	err := c.Get(ctx, svcNamespacedName, svc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).InfoS("target Service object of MultiClusterService object doesn't exist",
				"namespacedName", svcNamespacedName.String())
			return nil
		}
		klog.ErrorS(err, "failed to get target Service object of MultiClusterService object",
			"namespacedName", svcNamespacedName.String())
		return err
	}

	// 2. make sure to propagate the Service object with ResourceBinding.
	err = c.syncNativeService(ctx, svc, mcs, destinationClusters)
	if err != nil {
		return err
	}

	// 3. make sure to propagate the EndpointSlice list with ResourceBinding.
	err = c.syncEndpointSlice(ctx, mcs, destinationClusters)
	if err != nil {
		return err
	}
	return nil
}

func (c *MultiClusterServiceController) syncNativeService(
	ctx context.Context,
	svc *corev1.Service,
	mcs *networkingv1alpha1.MultiClusterService,
	destinationClusters []string,
) error {
	svcRB := &workv1alpha2.ResourceBinding{}
	rbNamespacedName := types.NamespacedName{
		Namespace: mcs.Namespace,
		Name:      names.GenerateBindingName(util.ServiceKind, mcs.Name),
	}
	err := c.Get(ctx, rbNamespacedName, svcRB)
	if err != nil {
		klog.ErrorS(err, "failed to get Service's ResourceBinding",
			"namespacedName", rbNamespacedName.String())
		return err
	}

	sourceClusterSet := util.ClusterSetToBePropagated(&svcRB.Spec)
	targetClusters := deDuplicatesInDestination(sourceClusterSet, destinationClusters)
	nativeSvcRB := buildNativeServiceRB(svc, targetClusters)

	var operationResult controllerutil.OperationResult
	bindingCopy := nativeSvcRB.DeepCopy()
	err = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		operationResult, err = controllerutil.CreateOrUpdate(context.TODO(), c.Client, bindingCopy, func() error {
			// just update necessary fields
			bindingCopy.Spec.Resource = nativeSvcRB.Spec.Resource
			bindingCopy.Spec.Clusters = nativeSvcRB.Spec.Clusters
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		klog.ErrorS(err, "failed to create or update native Service ResourceBinding",
			"Namespace", nativeSvcRB.Namespace, "Name", nativeSvcRB.Name)
		return err
	}

	if operationResult == controllerutil.OperationResultCreated {
		klog.V(4).InfoS("create native Service ResourceBinding successfully",
			"Namespace", nativeSvcRB.Namespace, "Name", nativeSvcRB.Name)
	} else if operationResult == controllerutil.OperationResultUpdated {
		klog.V(4).InfoS("Update native Service ResourceBinding successfully",
			"Namespace", nativeSvcRB.Namespace, "Name", nativeSvcRB.Name)
	} else {
		klog.V(4).InfoS("native Service ResourceBinding is up to date",
			"Namespace", nativeSvcRB.Namespace, "Name", nativeSvcRB.Name)
	}
	return nil
}

func (c *MultiClusterServiceController) syncEndpointSlice(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
	destinationClusters []string,
) error {
	epsList := &discoveryv1.EndpointSliceList{}
	err := c.List(ctx, epsList, &client.ListOptions{
		Namespace: mcs.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			discoveryv1.LabelServiceName: mcs.Name,
		}),
	})
	if err != nil {
		klog.ErrorS(err, "failed to list EndpointSlice associated with the Service",
			"Namespace", mcs.Namespace, "Service Name", mcs.Name)
		return err
	}

	var errs []error
	for index := range epsList.Items {
		eps := epsList.Items[index]
		epsRB := buildEndpointSliceRB(&eps, destinationClusters)

		var operationResult controllerutil.OperationResult
		bindingCopy := epsRB.DeepCopy()
		err = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
			operationResult, err = controllerutil.CreateOrUpdate(context.TODO(), c.Client, bindingCopy, func() error {
				// just update necessary fields
				bindingCopy.Spec.Resource = epsRB.Spec.Resource
				bindingCopy.Spec.Clusters = epsRB.Spec.Clusters
				return nil
			})
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			klog.ErrorS(err, "failed to create or update endpointSlice ResourceBinding",
				"Namespace", epsRB.Namespace, "Name", epsRB.Name)
			errs = append(errs, err)
			continue
		}

		if operationResult == controllerutil.OperationResultCreated {
			klog.InfoS("create endpointSlice ResourceBinding successfully",
				"Namespace", epsRB.Namespace, "Name", epsRB.Name)
		} else if operationResult == controllerutil.OperationResultUpdated {
			klog.InfoS("Update endpointSlice ResourceBinding successfully",
				"Namespace", epsRB.Namespace, "Name", epsRB.Name)
		} else {
			klog.InfoS("endpointSlice ResourceBinding is up to date",
				"Namespace", epsRB.Namespace, "Name", epsRB.Name)
		}
	}

	if len(errs) != 0 {
		return utilerrors.NewAggregate(errs)
	}
	return nil
}

func (c *MultiClusterServiceController) reconcileWithMCSDelete(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
) error {
	// 1. cleanup ServiceExport in karmada control-plane
	err := c.cleanupExportService(ctx, mcs)
	if err != nil {
		return err
	}

	// 2. when mcs is not in the deletion status, make cleanup related to the import service;
	// otherwise, gc-controller will be responsible for cleaning up.
	if mcs.DeletionTimestamp.IsZero() {
		err = c.cleanupImportService(ctx, mcs)
		if err != nil {
			return err
		}
	}

	// 3. remove own finalizer on MultiClusterService object
	if controllerutil.RemoveFinalizer(mcs, karmadaMCSFinalizer) {
		err := c.Client.Update(ctx, mcs)
		if err != nil {
			klog.ErrorS(err, "failed to update MultiClusterService object with finalizer",
				"namespace", mcs.Namespace, "name", mcs.Name, "finalizer", karmadaMCSFinalizer)
			return err
		}
	}
	return nil
}

func (c *MultiClusterServiceController) cleanupExportService(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
) error {
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

func (c *MultiClusterServiceController) cleanupImportService(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
) error {
	destinationClusters, err := c.destinationClustersWithMCS(ctx, mcs)
	if err != nil {
		return err
	}

	if strategyWithNativeService(mcs) {
		return c.cleanupWithNativeServiceMethod(ctx, mcs)
	}
	return c.cleanupWithServiceImportMethod(ctx, mcs, destinationClusters)
}

func (c *MultiClusterServiceController) cleanupWithNativeServiceMethod(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
) error {
	// 1. cleanup native Service ResourceBinding
	err := c.cleanupNativeService(ctx, mcs)
	if err != nil {
		return err
	}

	// 2. cleanup EndpointSlice ResourceBinding
	err = c.cleanupEndpointSlice(ctx, mcs)
	if err != nil {
		return err
	}
	return nil
}

func (c *MultiClusterServiceController) cleanupNativeService(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
) error {
	var nativeSvcRB *workv1alpha2.ResourceBinding
	rbName := generateNativeServiceBindingName(util.ServiceKind, mcs.Name)
	err := c.Get(ctx, types.NamespacedName{
		Namespace: mcs.Namespace,
		Name:      rbName,
	}, nativeSvcRB)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.ErrorS(err, "failed to get native Service ResourceBinding",
			"Namespace", mcs.Namespace, "Name", rbName)
		return err
	}

	err = c.Delete(ctx, nativeSvcRB)
	if err != nil {
		klog.ErrorS(err, "failed to delete native Service ResourceBinding",
			"Namespace", mcs.Namespace, "Name", rbName)
		return err
	}
	return nil
}

func (c *MultiClusterServiceController) cleanupEndpointSlice(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
) error {
	epsList := &discoveryv1.EndpointSliceList{}
	err := c.List(ctx, epsList, &client.ListOptions{
		Namespace: mcs.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			discoveryv1.LabelServiceName: mcs.Name,
		}),
	})
	if err != nil {
		klog.ErrorS(err, "failed to list EndpointSlice associated with the Service",
			"Namespace", mcs.Namespace, "Service Name", mcs.Name)
		return err
	}

	var errs []error
	for index := range epsList.Items {
		eps := epsList.Items[index]
		var epsRB *workv1alpha2.ResourceBinding
		err = c.Get(ctx, types.NamespacedName{
			Namespace: mcs.Namespace,
			Name:      names.GenerateBindingName(util.EndpointSliceKind, eps.Name),
		}, epsRB)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			klog.ErrorS(err, "failed to get endpointSlice ResourceBinding",
				"Namespace", epsRB.Namespace, "Name", epsRB.Name)
			errs = append(errs, err)
			continue
		}

		err = c.Delete(ctx, epsRB)
		if err != nil {
			klog.ErrorS(err, "failed to delete endpointSlice ResourceBinding",
				"Namespace", epsRB.Namespace, "Name", epsRB.Name)
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return utilerrors.NewAggregate(errs)
	}
	return nil
}

func (c *MultiClusterServiceController) ensureWithServiceImportMethod(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
	destinationClusters []string,
) error {
	// TODO: To be implemented.
	return nil
}

func (c *MultiClusterServiceController) cleanupWithServiceImportMethod(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
	destinationClusters []string,
) error {
	// TODO: To be implemented.
	return nil
}

func (c *MultiClusterServiceController) destinationClustersWithMCS(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
) ([]string, error) {
	var destinationClusters []string
	if mcs.Spec.Range == nil {
		clusters := &clusterv1alpha1.ClusterList{}
		err := c.List(ctx, clusters)
		if err != nil {
			klog.ErrorS(err, "failed to list Cluster objects")
			return nil, err
		}

		for _, cluster := range clusters.Items {
			destinationClusters = append(destinationClusters, cluster.Name)
		}
		return destinationClusters, nil
	}
	destinationClusters = mcs.Spec.Range.ClusterNames
	return destinationClusters, nil
}
