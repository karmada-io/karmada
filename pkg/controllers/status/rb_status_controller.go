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

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// RBStatusControllerName is the controller name that will be used when reporting events.
const RBStatusControllerName = "resource-binding-status-controller"

// RBStatusController is to sync status of ResourceBinding
// and aggregate status to the resource template.
type RBStatusController struct {
	client.Client                                                   // used to operate ResourceBinding resources.
	DynamicClient       dynamic.Interface                           // used to fetch arbitrary resources from api server.
	InformerManager     genericmanager.SingleClusterInformerManager // used to fetch arbitrary resources from cache.
	ResourceInterpreter resourceinterpreter.ResourceInterpreter
	EventRecorder       record.EventRecorder
	RESTMapper          meta.RESTMapper
	RateLimiterOptions  ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *RBStatusController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling ResourceBinding %s.", req.NamespacedName.String())

	binding := &workv1alpha2.ResourceBinding{}
	if err := c.Client.Get(ctx, req.NamespacedName, binding); err != nil {
		// The rb no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	// The rb is being deleted, in which case we stop processing.
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
func (c *RBStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	workMapFunc := handler.MapFunc(
		func(ctx context.Context, workObj client.Object) []reconcile.Request {
			var requests []reconcile.Request

			annotations := workObj.GetAnnotations()
			namespace, nsExist := annotations[workv1alpha2.ResourceBindingNamespaceAnnotationKey]
			name, nameExist := annotations[workv1alpha2.ResourceBindingNameAnnotationKey]
			if nsExist && nameExist {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: namespace,
						Name:      name,
					},
				})
			}

			return requests
		})

	return controllerruntime.NewControllerManagedBy(mgr).Named("resourceBinding_status_controller").
		Watches(&workv1alpha1.Work{}, handler.EnqueueRequestsFromMapFunc(workMapFunc), workPredicateFn).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions)}).
		Complete(c)
}

func (c *RBStatusController) syncBindingStatus(binding *workv1alpha2.ResourceBinding) error {
	resourceTemplate, err := helper.FetchResourceTemplate(c.DynamicClient, c.InformerManager, c.RESTMapper, binding.Spec.Resource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// It might happen when the resource template has been removed but the garbage collector hasn't removed
			// the ResourceBinding which dependent on resource template.
			// So, just return without retry(requeue) would save unnecessary loop.
			return nil
		}
		klog.Errorf("Failed to fetch workload for resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return err
	}

	err = helper.AggregateResourceBindingWorkStatus(c.Client, binding, resourceTemplate, c.EventRecorder)
	if err != nil {
		klog.Errorf("Failed to aggregate workStatues to resourceBinding(%s/%s), Error: %v",
			binding.Namespace, binding.Name, err)
		return err
	}

	err = updateResourceStatus(c.DynamicClient, c.RESTMapper, c.ResourceInterpreter, resourceTemplate, binding.Status)
	if err != nil {
		return err
	}
	return nil
}
