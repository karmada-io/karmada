package status

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// CRBStatusControllerName is the controller name that will be used when reporting events.
const CRBStatusControllerName = "cluster-resource-binding-status-controller"

// CRBStatusController is to sync status of ClusterResourceBinding
// and aggregate status to the resource template.
type CRBStatusController struct {
	client.Client                                                   // used to operate ClusterResourceBinding resources.
	DynamicClient       dynamic.Interface                           // used to fetch arbitrary resources from api server.
	InformerManager     genericmanager.SingleClusterInformerManager // used to fetch arbitrary resources from cache.
	EventRecorder       record.EventRecorder
	RESTMapper          meta.RESTMapper
	ResourceInterpreter resourceinterpreter.ResourceInterpreter
	RateLimiterOptions  ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *CRBStatusController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling ClusterResourceBinding %s.", req.NamespacedName.String())

	binding := &workv1alpha2.ClusterResourceBinding{}
	if err := c.Client.Get(ctx, req.NamespacedName, binding); err != nil {
		// The rb no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	// The crb is being deleted, in which case we stop processing.
	if !binding.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	err := c.syncBindingStatus(binding)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *CRBStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	workMapFunc := handler.MapFunc(
		func(workObj client.Object) []reconcile.Request {
			var requests []reconcile.Request

			annotations := workObj.GetAnnotations()
			name, nameExist := annotations[workv1alpha2.ClusterResourceBindingAnnotationKey]
			if nameExist {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: name,
					},
				})
			}

			return requests
		})

	return controllerruntime.NewControllerManagedBy(mgr).Named("clusterResourceBinding_status_controller").
		Watches(&source.Kind{Type: &workv1alpha1.Work{}}, handler.EnqueueRequestsFromMapFunc(workMapFunc), workPredicateFn).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions)}).
		Complete(c)
}

func (c *CRBStatusController) syncBindingStatus(binding *workv1alpha2.ClusterResourceBinding) error {
	resource, err := helper.FetchResourceTemplate(c.DynamicClient, c.InformerManager, c.RESTMapper, binding.Spec.Resource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// It might happen when the resource template has been removed but the garbage collector hasn't removed
			// the ResourceBinding which dependent on resource template.
			// So, just return without retry(requeue) would save unnecessary loop.
			return nil
		}
		klog.Errorf("Failed to fetch workload for clusterResourceBinding(%s). Error: %v",
			binding.GetName(), err)
		return err
	}

	err = helper.AggregateClusterResourceBindingWorkStatus(c.Client, binding, resource, c.EventRecorder)
	if err != nil {
		klog.Errorf("Failed to aggregate workStatues to clusterResourceBinding(%s), Error: %v",
			binding.Name, err)
		return err
	}

	err = updateResourceStatus(c.DynamicClient, c.RESTMapper, c.ResourceInterpreter, resource, binding.Status)
	if err != nil {
		return err
	}
	return nil
}
