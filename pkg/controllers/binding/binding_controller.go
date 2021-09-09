package binding

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName                    = "binding-controller"
	eventTypeFindOrphanWorks          = "FindOrphanWorks"
	eventTypeFetchWorkload            = "FetchWorkload"
	eventTypeTransformResourceBinding = "TransformResourceBinding"
	eventTypeAggregateWorkStatuses    = "AggregateWorkStatuses"
)

// ResourceBindingController is to sync ResourceBinding.
type ResourceBindingController struct {
	client          client.Client     // used to operate ResourceBinding resources.
	dynamicClient   dynamic.Interface // used to fetch arbitrary resources.
	recorder        record.EventRecorder
	restMapper      meta.RESTMapper
	overrideManager overridemanager.OverrideManager
}

// NewResourceBindingController creates a new ResourceBindingController.
func NewResourceBindingController(client client.Client, dynamicClient dynamic.Interface, recorder record.EventRecorder, restMapper meta.RESTMapper, overrideManager overridemanager.OverrideManager) *ResourceBindingController {
	rbConroller := ResourceBindingController{
		client:          client,
		dynamicClient:   dynamicClient,
		recorder:        recorder,
		restMapper:      restMapper,
		overrideManager: overrideManager,
	}
	return &rbConroller
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *ResourceBindingController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling ResourceBinding %s.", req.NamespacedName.String())

	binding := &workv1alpha1.ResourceBinding{}
	if err := c.client.Get(context.TODO(), req.NamespacedName, binding); err != nil {
		// The resource no longer exist, clean up derived Work objects.
		if apierrors.IsNotFound(err) {
			return helper.DeleteWorks(c.client, labels.Set{
				workv1alpha1.ResourceBindingNamespaceLabel: req.Namespace,
				workv1alpha1.ResourceBindingNameLabel:      req.Name,
			})
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !binding.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	isReady := helper.IsBindingReady(binding.Spec.Clusters)
	if !isReady {
		msg := fmt.Sprintf("ResourceBinding %s is not ready.", req.NamespacedName.String())
		klog.Infof(msg)
		c.recorder.Event(binding, corev1.EventTypeWarning, "ResourceBindingNotReady", msg)

		return controllerruntime.Result{}, nil
	}

	return c.syncBinding(binding)
}

// syncBinding will sync resourceBinding to Works.
func (c *ResourceBindingController) syncBinding(binding *workv1alpha1.ResourceBinding) (controllerruntime.Result, error) {
	clusterNames := helper.GetBindingClusterNames(binding.Spec.Clusters)
	works, err := helper.FindOrphanWorks(c.client, binding.Namespace, binding.Name, clusterNames, apiextensionsv1.NamespaceScoped)
	if err != nil {
		klog.Errorf("Failed to find orphan works by resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		c.recorder.Event(binding, corev1.EventTypeWarning, fmt.Sprintf("Failed %s", eventTypeFindOrphanWorks), err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}

	err = helper.RemoveOrphanWorks(c.client, works)
	if err != nil {
		klog.Errorf("Failed to remove orphan works by resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		c.recorder.Event(binding, corev1.EventTypeWarning, fmt.Sprintf("Failed %s", eventTypeFindOrphanWorks), err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}

	workload, err := helper.FetchWorkload(c.dynamicClient, c.restMapper, binding.Spec.Resource)
	if err != nil {
		klog.Errorf("Failed to fetch workload for resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		c.recorder.Event(binding, corev1.EventTypeWarning, fmt.Sprintf("Failed %s", eventTypeFetchWorkload), err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}

	err = ensureWork(c.client, workload, c.overrideManager, binding, apiextensionsv1.NamespaceScoped)
	if err != nil {
		klog.Errorf("Failed to transform resourceBinding(%s/%s) to works. Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		c.recorder.Event(binding, corev1.EventTypeWarning, fmt.Sprintf("Failed %s", eventTypeTransformResourceBinding), err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}

	err = helper.AggregateResourceBindingWorkStatus(c.client, binding, workload)
	if err != nil {
		klog.Errorf("Failed to aggregate workStatuses to resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		c.recorder.Event(binding, corev1.EventTypeWarning, fmt.Sprintf("Failed %s", eventTypeAggregateWorkStatuses), err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}
	klog.V(4).Infof("Update resourceBinding(%s/%s) with AggregatedStatus successfully.", binding.Namespace, binding.Name)
	c.recorder.Event(binding, corev1.EventTypeNormal, "SyncResourceBinding", "ResourceBinding synced successfully")

	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ResourceBindingController) SetupWithManager(mgr controllerruntime.Manager) error {
	workFn := handler.MapFunc(
		func(a client.Object) []reconcile.Request {
			var requests []reconcile.Request

			labels := a.GetLabels()
			resourcebindingNamespace, namespaceExist := labels[workv1alpha1.ResourceBindingNamespaceLabel]
			resourcebindingName, nameExist := labels[workv1alpha1.ResourceBindingNameLabel]
			if !namespaceExist || !nameExist {
				return nil
			}
			namespacesName := types.NamespacedName{
				Namespace: resourcebindingNamespace,
				Name:      resourcebindingName,
			}

			requests = append(requests, reconcile.Request{NamespacedName: namespacesName})
			return requests
		})

	return controllerruntime.NewControllerManagedBy(mgr).For(&workv1alpha1.ResourceBinding{}).
		Watches(&source.Kind{Type: &workv1alpha1.Work{}}, handler.EnqueueRequestsFromMapFunc(workFn), workPredicateFn).
		Watches(&source.Kind{Type: &policyv1alpha1.OverridePolicy{}}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		Watches(&source.Kind{Type: &policyv1alpha1.ClusterOverridePolicy{}}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		Watches(&source.Kind{Type: &policyv1alpha1.ReplicaSchedulingPolicy{}}, handler.EnqueueRequestsFromMapFunc(c.newReplicaSchedulingPolicyFunc())).
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

		bindingList := &workv1alpha1.ResourceBindingList{}
		if err := c.client.List(context.TODO(), bindingList); err != nil {
			klog.Errorf("Failed to list resourceBindings, error: %v", err)
			return nil
		}

		var requests []reconcile.Request
		for _, binding := range bindingList.Items {
			workload, err := helper.FetchWorkload(c.dynamicClient, c.restMapper, binding.Spec.Resource)
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

func (c *ResourceBindingController) newReplicaSchedulingPolicyFunc() handler.MapFunc {
	return func(a client.Object) []reconcile.Request {
		rspResourceSelectors := a.(*policyv1alpha1.ReplicaSchedulingPolicy).Spec.ResourceSelectors
		bindingList := &workv1alpha1.ResourceBindingList{}
		if err := c.client.List(context.TODO(), bindingList); err != nil {
			klog.Errorf("Failed to list resourceBindings, error: %v", err)
			return nil
		}

		var requests []reconcile.Request
		for _, binding := range bindingList.Items {
			workload, err := helper.FetchWorkload(c.dynamicClient, c.restMapper, binding.Spec.Resource)
			if err != nil {
				klog.Errorf("Failed to fetch workload for resourceBinding(%s/%s). Error: %v.", binding.Namespace, binding.Name, err)
				return nil
			}

			for _, rs := range rspResourceSelectors {
				if util.ResourceMatches(workload, rs) {
					klog.V(2).Infof("Enqueue ResourceBinding(%s/%s) as replica scheduling policy(%s/%s) changes.", binding.Namespace, binding.Name, a.GetNamespace(), a.GetName())
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: binding.Namespace, Name: binding.Name}})
					break
				}
			}
		}
		return requests
	}
}
