package binding

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

// ClusterResourceBindingControllerName is the controller name that will be used when reporting events.
const ClusterResourceBindingControllerName = "cluster-resource-binding-controller"

// ClusterResourceBindingController is to sync ClusterResourceBinding.
type ClusterResourceBindingController struct {
	client.Client                                                    // used to operate ClusterResourceBinding resources.
	DynamicClient       dynamic.Interface                            // used to fetch arbitrary resources from api server.
	InformerManager     informermanager.SingleClusterInformerManager // used to fetch arbitrary resources from cache.
	EventRecorder       record.EventRecorder
	RESTMapper          meta.RESTMapper
	OverrideManager     overridemanager.OverrideManager
	ResourceInterpreter resourceinterpreter.ResourceInterpreter
	RateLimiterOptions  ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *ClusterResourceBindingController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling ClusterResourceBinding %s.", req.NamespacedName.String())

	clusterResourceBinding := &workv1alpha2.ClusterResourceBinding{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, clusterResourceBinding); err != nil {
		// The resource no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !clusterResourceBinding.DeletionTimestamp.IsZero() {
		klog.V(4).Infof("Begin to delete works owned by binding(%s).", req.NamespacedName.String())
		if err := helper.DeleteWorkByCRBName(c.Client, req.Name); err != nil {
			klog.Errorf("Failed to delete works related to %s: %v", clusterResourceBinding.GetName(), err)
			return controllerruntime.Result{Requeue: true}, err
		}
		return c.removeFinalizer(clusterResourceBinding)
	}

	return c.syncBinding(clusterResourceBinding)
}

// removeFinalizer removes finalizer from the given ClusterResourceBinding
func (c *ClusterResourceBindingController) removeFinalizer(crb *workv1alpha2.ClusterResourceBinding) (controllerruntime.Result, error) {
	if !controllerutil.ContainsFinalizer(crb, util.ClusterResourceBindingControllerFinalizer) {
		return controllerruntime.Result{}, nil
	}

	controllerutil.RemoveFinalizer(crb, util.ClusterResourceBindingControllerFinalizer)
	err := c.Client.Update(context.TODO(), crb)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// syncBinding will sync clusterResourceBinding to Works.
func (c *ClusterResourceBindingController) syncBinding(binding *workv1alpha2.ClusterResourceBinding) (controllerruntime.Result, error) {
	clusterNames := helper.GetBindingClusterNames(binding.Spec.Clusters, binding.Spec.RequiredBy)
	works, err := helper.FindOrphanWorks(c.Client, "", binding.Name, clusterNames, apiextensionsv1.ClusterScoped)
	if err != nil {
		klog.Errorf("Failed to find orphan works by ClusterResourceBinding(%s). Error: %v.", binding.GetName(), err)
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, workv1alpha2.EventReasonCleanupWorkFailed, err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}

	err = helper.RemoveOrphanWorks(c.Client, works)
	if err != nil {
		klog.Errorf("Failed to remove orphan works by clusterResourceBinding(%s). Error: %v.", binding.GetName(), err)
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, workv1alpha2.EventReasonCleanupWorkFailed, err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}

	workload, err := helper.FetchWorkload(c.DynamicClient, c.InformerManager, c.RESTMapper, binding.Spec.Resource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// It might happen when the resource template has been removed but the garbage collector hasn't removed
			// the ClusterResourceBinding which dependent on resource template.
			// So, just return without retry(requeue) would save unnecessary loop.
			return controllerruntime.Result{}, nil
		}
		klog.Errorf("Failed to fetch workload for clusterResourceBinding(%s). Error: %v.", binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	var errs []error
	err = ensureWork(c.Client, c.ResourceInterpreter, workload, c.OverrideManager, binding, apiextensionsv1.ClusterScoped)
	if err != nil {
		klog.Errorf("Failed to transform clusterResourceBinding(%s) to works. Error: %v.", binding.GetName(), err)
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, workv1alpha2.EventReasonSyncWorkFailed, err.Error())
		errs = append(errs, err)
	} else {
		msg := fmt.Sprintf("Sync work of clusterResourceBinding(%s) successful.", binding.GetName())
		klog.V(4).Infof(msg)
		c.EventRecorder.Event(binding, corev1.EventTypeNormal, workv1alpha2.EventReasonSyncWorkSucceed, msg)
	}

	err = helper.AggregateClusterResourceBindingWorkStatus(c.Client, binding, workload)
	if err != nil {
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, workv1alpha2.EventReasonAggregateStatusFailed, err.Error())
		errs = append(errs, err)
	} else {
		msg := fmt.Sprintf("Update clusterResourceBinding(%s) with AggregatedStatus successfully.", binding.Name)
		klog.V(4).Infof(msg)
		c.EventRecorder.Event(binding, corev1.EventTypeNormal, workv1alpha2.EventReasonAggregateStatusSucceed, msg)
	}
	if len(errs) > 0 {
		return controllerruntime.Result{Requeue: true}, errors.NewAggregate(errs)
	}

	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ClusterResourceBindingController) SetupWithManager(mgr controllerruntime.Manager) error {
	workFn := handler.MapFunc(
		func(a client.Object) []reconcile.Request {
			var requests []reconcile.Request
			annotations := a.GetAnnotations()
			crbName, nameExist := annotations[workv1alpha2.ClusterResourceBindingAnnotationKey]
			if nameExist {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: crbName,
					},
				})
			}

			return requests
		})

	return controllerruntime.NewControllerManagedBy(mgr).For(&workv1alpha2.ClusterResourceBinding{}).
		Watches(&source.Kind{Type: &workv1alpha1.Work{}}, handler.EnqueueRequestsFromMapFunc(workFn), workPredicateFn).
		Watches(&source.Kind{Type: &policyv1alpha1.OverridePolicy{}}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		Watches(&source.Kind{Type: &policyv1alpha1.ClusterOverridePolicy{}}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions),
		}).
		Complete(c)
}

func (c *ClusterResourceBindingController) newOverridePolicyFunc() handler.MapFunc {
	return func(a client.Object) []reconcile.Request {
		var overrideRS []policyv1alpha1.ResourceSelector
		switch t := a.(type) {
		case *policyv1alpha1.ClusterOverridePolicy:
			overrideRS = t.Spec.ResourceSelectors
		case *policyv1alpha1.OverridePolicy:
			overrideRS = t.Spec.ResourceSelectors
		default:
			return nil
		}

		bindingList := &workv1alpha2.ClusterResourceBindingList{}
		if err := c.Client.List(context.TODO(), bindingList); err != nil {
			klog.Errorf("Failed to list clusterResourceBindings, error: %v", err)
			return nil
		}

		var requests []reconcile.Request
		for _, binding := range bindingList.Items {
			workload, err := helper.FetchWorkload(c.DynamicClient, c.InformerManager, c.RESTMapper, binding.Spec.Resource)
			if err != nil {
				klog.Errorf("Failed to fetch workload for clusterResourceBinding(%s). Error: %v.", binding.Name, err)
				return nil
			}

			for _, rs := range overrideRS {
				if util.ResourceMatches(workload, rs) {
					klog.V(2).Infof("Enqueue ClusterResourceBinding(%s) as override policy(%s/%s) changes.", binding.Name, a.GetNamespace(), a.GetName())
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: binding.Name}})
					break
				}
			}
		}
		return requests
	}
}
