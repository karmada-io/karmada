package binding

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

// ControllerName is the controller name that will be used when reporting events.
const ControllerName = "binding-controller"

// ResourceBindingController is to sync ResourceBinding.
type ResourceBindingController struct {
	client.Client                     // used to operate ResourceBinding resources.
	DynamicClient   dynamic.Interface // used to fetch arbitrary resources.
	EventRecorder   record.EventRecorder
	RESTMapper      meta.RESTMapper
	OverrideManager overridemanager.OverrideManager
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *ResourceBindingController) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling ResourceBinding %s.", req.NamespacedName.String())

	binding := &workv1alpha1.ResourceBinding{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, binding); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !binding.DeletionTimestamp.IsZero() {
		// Do nothing, just return as we have added owner reference to Work.
		// Work will be removed automatically by garbage collector.
		return controllerruntime.Result{}, nil
	}

	isReady := helper.IsBindingReady(binding.Spec.Clusters)
	if !isReady {
		klog.Infof("ResourceBinding(%s/%s) is not ready to sync", binding.GetNamespace(), binding.GetName())
		return controllerruntime.Result{}, nil
	}

	return c.syncBinding(binding)
}

// syncBinding will sync resourceBinding to Works.
func (c *ResourceBindingController) syncBinding(binding *workv1alpha1.ResourceBinding) (controllerruntime.Result, error) {
	clusterNames := helper.GetBindingClusterNames(binding.Spec.Clusters)
	works, err := helper.FindOrphanWorks(c.Client, binding.Namespace, binding.Name, clusterNames, apiextensionsv1.NamespaceScoped)
	if err != nil {
		klog.Errorf("Failed to find orphan works by resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	err = helper.RemoveOrphanWorks(c.Client, works)
	if err != nil {
		klog.Errorf("Failed to remove orphan works by resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	workload, err := helper.FetchWorkload(c.DynamicClient, c.RESTMapper, binding.Spec.Resource)
	if err != nil {
		klog.Errorf("Failed to fetch workload for resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	err = helper.EnsureWork(c.Client, workload, clusterNames, c.OverrideManager, binding, apiextensionsv1.NamespaceScoped)
	if err != nil {
		klog.Errorf("Failed to transform resourceBinding(%s/%s) to works. Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	err = helper.AggregateResourceBindingWorkStatus(c.Client, binding, workload)
	if err != nil {
		klog.Errorf("Failed to aggregate workStatuses to resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ResourceBindingController) SetupWithManager(mgr controllerruntime.Manager) error {
	workFn := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			var requests []reconcile.Request

			labels := a.Meta.GetLabels()
			_, namespaceExist := labels[util.ResourceBindingNamespaceLabel]
			_, nameExist := labels[util.ResourceBindingNameLabel]
			if !namespaceExist || !nameExist {
				return nil
			}
			namespacesName := types.NamespacedName{
				Namespace: labels[util.ResourceBindingNamespaceLabel],
				Name:      labels[util.ResourceBindingNameLabel],
			}

			requests = append(requests, reconcile.Request{NamespacedName: namespacesName})
			return requests
		})

	return controllerruntime.NewControllerManagedBy(mgr).For(&workv1alpha1.ResourceBinding{}).
		Watches(&source.Kind{Type: &workv1alpha1.Work{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: workFn}, workPredicateFn).
		Complete(c)
}
