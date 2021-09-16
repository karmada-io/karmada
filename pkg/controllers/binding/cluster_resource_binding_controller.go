package binding

import (
	"context"

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

// ClusterResourceBindingControllerName is the controller name that will be used when reporting events.
const ClusterResourceBindingControllerName = "cluster-resource-binding-controller"

// ClusterResourceBindingController is to sync ClusterResourceBinding.
type ClusterResourceBindingController struct {
	client.Client                     // used to operate ClusterResourceBinding resources.
	DynamicClient   dynamic.Interface // used to fetch arbitrary resources.
	EventRecorder   record.EventRecorder
	RESTMapper      meta.RESTMapper
	OverrideManager overridemanager.OverrideManager
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *ClusterResourceBindingController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling ClusterResourceBinding %s.", req.NamespacedName.String())

	clusterResourceBinding := &workv1alpha1.ClusterResourceBinding{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, clusterResourceBinding); err != nil {
		// The resource no longer exist, clean up derived Work objects.
		if apierrors.IsNotFound(err) {
			return helper.DeleteWorks(c.Client, labels.Set{
				workv1alpha1.ClusterResourceBindingLabel: req.Name,
			})
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !clusterResourceBinding.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	isReady := helper.IsBindingReady(clusterResourceBinding.Spec.Clusters)
	if !isReady {
		klog.Infof("ClusterResourceBinding %s is not ready to sync", clusterResourceBinding.GetName())
		return controllerruntime.Result{}, nil
	}

	return c.syncBinding(clusterResourceBinding)
}

// syncBinding will sync clusterResourceBinding to Works.
func (c *ClusterResourceBindingController) syncBinding(binding *workv1alpha1.ClusterResourceBinding) (controllerruntime.Result, error) {
	clusterNames := helper.GetBindingClusterNames(binding.Spec.Clusters)
	works, err := helper.FindOrphanWorks(c.Client, "", binding.Name, clusterNames, apiextensionsv1.ClusterScoped)
	if err != nil {
		klog.Errorf("Failed to find orphan works by ClusterResourceBinding(%s). Error: %v.", binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	err = helper.RemoveOrphanWorks(c.Client, works)
	if err != nil {
		klog.Errorf("Failed to remove orphan works by clusterResourceBinding(%s). Error: %v.", binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	workload, err := helper.FetchWorkload(c.DynamicClient, c.RESTMapper, binding.Spec.Resource)
	if err != nil {
		klog.Errorf("Failed to fetch workload for clusterResourceBinding(%s). Error: %v.", binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	err = ensureWork(c.Client, workload, c.OverrideManager, binding, apiextensionsv1.ClusterScoped)
	if err != nil {
		klog.Errorf("Failed to transform clusterResourceBinding(%s) to works. Error: %v.", binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	err = helper.AggregateClusterResourceBindingWorkStatus(c.Client, binding, workload)
	if err != nil {
		klog.Errorf("Failed to aggregate workStatuses to clusterResourceBinding(%s). Error: %v.", binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	klog.V(4).Infof("Update clusterResourceBinding(%s) with AggregatedStatus successfully.", binding.Name)

	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ClusterResourceBindingController) SetupWithManager(mgr controllerruntime.Manager) error {
	workFn := handler.MapFunc(
		func(a client.Object) []reconcile.Request {
			var requests []reconcile.Request

			labels := a.GetLabels()
			clusterResourcebindingName, nameExist := labels[workv1alpha1.ClusterResourceBindingLabel]
			if !nameExist {
				return nil
			}
			namespacesName := types.NamespacedName{
				Name: clusterResourcebindingName,
			}

			requests = append(requests, reconcile.Request{NamespacedName: namespacesName})
			return requests
		})

	return controllerruntime.NewControllerManagedBy(mgr).For(&workv1alpha1.ClusterResourceBinding{}).
		Watches(&source.Kind{Type: &workv1alpha1.Work{}}, handler.EnqueueRequestsFromMapFunc(workFn), workPredicateFn).
		Watches(&source.Kind{Type: &policyv1alpha1.OverridePolicy{}}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		Watches(&source.Kind{Type: &policyv1alpha1.ClusterOverridePolicy{}}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		Watches(&source.Kind{Type: &policyv1alpha1.ReplicaSchedulingPolicy{}}, handler.EnqueueRequestsFromMapFunc(c.newReplicaSchedulingPolicyFunc())).
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

		bindingList := &workv1alpha1.ClusterResourceBindingList{}
		if err := c.Client.List(context.TODO(), bindingList); err != nil {
			klog.Errorf("Failed to list clusterResourceBindings, error: %v", err)
			return nil
		}

		var requests []reconcile.Request
		for _, binding := range bindingList.Items {
			workload, err := helper.FetchWorkload(c.DynamicClient, c.RESTMapper, binding.Spec.Resource)
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

func (c *ClusterResourceBindingController) newReplicaSchedulingPolicyFunc() handler.MapFunc {
	return func(a client.Object) []reconcile.Request {
		rspResourceSelectors := a.(*policyv1alpha1.ReplicaSchedulingPolicy).Spec.ResourceSelectors
		bindingList := &workv1alpha1.ClusterResourceBindingList{}
		if err := c.Client.List(context.TODO(), bindingList); err != nil {
			klog.Errorf("Failed to list clusterResourceBindings, error: %v", err)
			return nil
		}

		var requests []reconcile.Request
		for _, binding := range bindingList.Items {
			workload, err := helper.FetchWorkload(c.DynamicClient, c.RESTMapper, binding.Spec.Resource)
			if err != nil {
				klog.Errorf("Failed to fetch workload for clusterResourceBinding(%s). Error: %v.", binding.Name, err)
				return nil
			}

			for _, rs := range rspResourceSelectors {
				if util.ResourceMatches(workload, rs) {
					klog.V(2).Infof("Enqueue ClusterResourceBinding(%s) as replica scheduling policy(%s/%s) changes.", binding.Name, a.GetNamespace(), a.GetName())
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: binding.Name}})
					break
				}
			}
		}
		return requests
	}
}
