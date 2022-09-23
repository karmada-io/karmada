package binding

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ControllerName is the controller name that will be used when reporting events.
const ControllerName = "binding-controller"

// ResourceBindingController is to sync ResourceBinding.
type ResourceBindingController struct {
	client.Client                                                   // used to operate ClusterResourceBinding resources.
	DynamicClient       dynamic.Interface                           // used to fetch arbitrary resources from api server.
	InformerManager     genericmanager.SingleClusterInformerManager // used to fetch arbitrary resources from cache.
	EventRecorder       record.EventRecorder
	RESTMapper          meta.RESTMapper
	OverrideManager     overridemanager.OverrideManager
	ResourceInterpreter resourceinterpreter.ResourceInterpreter
	RateLimiterOptions  ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *ResourceBindingController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling ResourceBinding %s.", req.NamespacedName.String())

	binding := &workv1alpha2.ResourceBinding{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, binding); err != nil {
		// The resource no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !binding.DeletionTimestamp.IsZero() {
		klog.V(4).Infof("Begin to delete works owned by binding(%s).", req.NamespacedName.String())
		if err := helper.DeleteWorkByRBNamespaceAndName(c.Client, req.Namespace, req.Name); err != nil {
			klog.Errorf("Failed to delete works related to %s/%s: %v", binding.GetNamespace(), binding.GetName(), err)
			return controllerruntime.Result{Requeue: true}, err
		}
		return c.removeFinalizer(binding)
	}

	return c.syncBinding(binding)
}

// removeFinalizer removes finalizer from the given ResourceBinding
func (c *ResourceBindingController) removeFinalizer(rb *workv1alpha2.ResourceBinding) (controllerruntime.Result, error) {
	if !controllerutil.ContainsFinalizer(rb, util.BindingControllerFinalizer) {
		return controllerruntime.Result{}, nil
	}

	controllerutil.RemoveFinalizer(rb, util.BindingControllerFinalizer)
	err := c.Client.Update(context.TODO(), rb)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// syncBinding will sync resourceBinding to Works.
func (c *ResourceBindingController) syncBinding(binding *workv1alpha2.ResourceBinding) (controllerruntime.Result, error) {
	if err := c.removeOrphanWorks(binding); err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	workload, err := helper.FetchWorkload(c.DynamicClient, c.InformerManager, c.RESTMapper, binding.Spec.Resource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// It might happen when the resource template has been removed but the garbage collector hasn't removed
			// the ResourceBinding which dependent on resource template.
			// So, just return without retry(requeue) would save unnecessary loop.
			return controllerruntime.Result{}, nil
		}
		klog.Errorf("Failed to fetch workload for resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	var errs []error
	err = ensureWork(c.Client, c.ResourceInterpreter, workload, c.OverrideManager, binding, apiextensionsv1.NamespaceScoped)
	if err != nil {
		klog.Errorf("Failed to transform resourceBinding(%s/%s) to works. Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, workv1alpha2.EventReasonSyncWorkFailed, err.Error())
		c.EventRecorder.Event(workload, corev1.EventTypeWarning, workv1alpha2.EventReasonSyncWorkFailed, err.Error())
		errs = append(errs, err)
	} else {
		msg := fmt.Sprintf("Sync work of resourceBinding(%s/%s) successful.", binding.Namespace, binding.Name)
		klog.V(4).Infof(msg)
		c.EventRecorder.Event(binding, corev1.EventTypeNormal, workv1alpha2.EventReasonSyncWorkSucceed, msg)
		c.EventRecorder.Event(workload, corev1.EventTypeNormal, workv1alpha2.EventReasonSyncWorkSucceed, msg)
	}
	if err = helper.AggregateResourceBindingWorkStatus(c.Client, binding, workload, c.EventRecorder); err != nil {
		klog.Errorf("Failed to aggregate workStatuses to resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return controllerruntime.Result{Requeue: true}, errors.NewAggregate(errs)
	}

	err = c.updateResourceStatus(binding)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	return controllerruntime.Result{}, nil
}

func (c *ResourceBindingController) removeOrphanWorks(binding *workv1alpha2.ResourceBinding) error {
	works, err := helper.FindOrphanWorks(c.Client, binding.Namespace, binding.Name, helper.ObtainBindingSpecExistingClusters(binding.Spec))
	if err != nil {
		klog.Errorf("Failed to find orphan works by resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, workv1alpha2.EventReasonCleanupWorkFailed, err.Error())
		return err
	}

	err = helper.RemoveOrphanWorks(c.Client, works)
	if err != nil {
		klog.Errorf("Failed to remove orphan works by resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, workv1alpha2.EventReasonCleanupWorkFailed, err.Error())
		return err
	}

	return nil
}

// updateResourceStatus will try to calculate the summary status and update to original object
// that the ResourceBinding refer to.
func (c *ResourceBindingController) updateResourceStatus(binding *workv1alpha2.ResourceBinding) error {
	resource := binding.Spec.Resource
	gvr, err := restmapper.GetGroupVersionResource(
		c.RESTMapper, schema.FromAPIVersionAndKind(resource.APIVersion, resource.Kind),
	)
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK %s %s. Error: %v", resource.APIVersion, resource.Kind, err)
		return err
	}
	obj, err := helper.FetchWorkload(c.DynamicClient, c.InformerManager, c.RESTMapper, resource)
	if err != nil {
		klog.Errorf("Failed to get resource(%s/%s/%s), Error: %v", resource.Kind, resource.Namespace, resource.Name, err)
		return err
	}

	if !c.ResourceInterpreter.HookEnabled(obj.GroupVersionKind(), configv1alpha1.InterpreterOperationAggregateStatus) {
		return nil
	}
	newObj, err := c.ResourceInterpreter.AggregateStatus(obj, binding.Status.AggregatedStatus)
	if err != nil {
		klog.Errorf("AggregateStatus for resource(%s/%s/%s) failed: %v", resource.Kind, resource.Namespace, resource.Name, err)
		return err
	}
	if reflect.DeepEqual(obj, newObj) {
		klog.V(3).Infof("ignore update resource(%s/%s/%s) status as up to date", resource.Kind, resource.Namespace, resource.Name)
		return nil
	}

	if _, err = c.DynamicClient.Resource(gvr).Namespace(resource.Namespace).UpdateStatus(context.TODO(), newObj, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("Failed to update resource(%s/%s/%s), Error: %v", resource.Kind, resource.Namespace, resource.Name, err)
		return err
	}
	klog.V(3).Infof("update resource status successfully for resource(%s/%s/%s)", resource.Kind, resource.Namespace, resource.Name)
	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ResourceBindingController) SetupWithManager(mgr controllerruntime.Manager) error {
	workFn := handler.MapFunc(
		func(a client.Object) []reconcile.Request {
			var requests []reconcile.Request
			annotations := a.GetAnnotations()
			crNamespace, namespaceExist := annotations[workv1alpha2.ResourceBindingNamespaceAnnotationKey]
			crName, nameExist := annotations[workv1alpha2.ResourceBindingNameAnnotationKey]
			if namespaceExist && nameExist {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: crNamespace,
						Name:      crName,
					},
				})
			}

			return requests
		})

	return controllerruntime.NewControllerManagedBy(mgr).For(&workv1alpha2.ResourceBinding{}).
		Watches(&source.Kind{Type: &workv1alpha1.Work{}}, handler.EnqueueRequestsFromMapFunc(workFn), workPredicateFn).
		Watches(&source.Kind{Type: &policyv1alpha1.OverridePolicy{}}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		Watches(&source.Kind{Type: &policyv1alpha1.ClusterOverridePolicy{}}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions),
		}).
		Complete(c)
}

func (c *ResourceBindingController) newOverridePolicyFunc() handler.MapFunc {
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

		bindingList := &workv1alpha2.ResourceBindingList{}
		if err := c.Client.List(context.TODO(), bindingList); err != nil {
			klog.Errorf("Failed to list resourceBindings, error: %v", err)
			return nil
		}

		var requests []reconcile.Request
		for _, binding := range bindingList.Items {
			workload, err := helper.FetchWorkload(c.DynamicClient, c.InformerManager, c.RESTMapper, binding.Spec.Resource)
			if err != nil {
				klog.Errorf("Failed to fetch workload for resourceBinding(%s/%s). Error: %v.", binding.Namespace, binding.Name, err)
				return nil
			}

			for _, rs := range overrideRS {
				if util.ResourceMatches(workload, rs) {
					klog.V(2).Infof("Enqueue ResourceBinding(%s/%s) as override policy(%s/%s) changes.", binding.Namespace, binding.Name, a.GetNamespace(), a.GetName())
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: binding.Namespace, Name: binding.Name}})
					break
				}
			}
		}
		return requests
	}
}
