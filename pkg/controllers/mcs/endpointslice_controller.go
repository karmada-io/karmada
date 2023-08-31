package mcs

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// EndpointSliceControllerName is the controller name that will be used when reporting events.
const EndpointSliceControllerName = "endpointslice-controller"

// EndpointSliceController is to collect EndpointSlice which reported by member cluster
// from executionNamespace to serviceexport namespace.
type EndpointSliceController struct {
	client.Client
	EventRecorder      record.EventRecorder
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
func (c *EndpointSliceController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling Work %s.", req.NamespacedName.String())

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, req.NamespacedName, work); err != nil {
		if apierrors.IsNotFound(err) {
			// Cleanup derived EndpointSlices after work has been removed.
			err = helper.DeleteEndpointSlice(c.Client, labels.Set{
				workv1alpha1.WorkNamespaceLabel: req.Namespace,
				workv1alpha1.WorkNameLabel:      req.Name,
			})
			if err == nil {
				return controllerruntime.Result{}, nil
			}
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !work.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	return controllerruntime.Result{}, c.unBoxingEndpointSlice(work)
}

func (c *EndpointSliceController) endpointSliceMapFunc() handler.MapFunc {
	return func(ctx context.Context, object client.Object) []reconcile.Request {
		var requests []reconcile.Request

		workList := &workv1alpha1.WorkList{}
		if err := c.List(context.TODO(), workList, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				util.ServiceNamespaceLabel: object.GetNamespace(),
				util.ServiceNameLabel:      object.GetName(),
			}),
		}); err != nil {
			klog.ErrorS(err, "failed to list works reported by member clusters and relate with Service",
				"Namespace", object.GetNamespace(), "Name", object.GetName())
			return nil
		}

		for _, work := range workList.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: work.Namespace,
					Name:      work.Name,
				}})
		}
		return requests
	}
}

// SetupWithManager creates a controller and register to controller manager.
func (c *EndpointSliceController) SetupWithManager(mgr controllerruntime.Manager) error {
	workPredicateFun := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return util.GetLabelValue(createEvent.Object.GetLabels(), util.ServiceNameLabel) != ""
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return util.GetLabelValue(updateEvent.ObjectNew.GetLabels(), util.ServiceNameLabel) != ""
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return util.GetLabelValue(deleteEvent.Object.GetLabels(), util.ServiceNameLabel) != ""
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&workv1alpha1.Work{}, builder.WithPredicates(workPredicateFun)).
		Watches(&networkingv1alpha1.MultiClusterService{}, handler.EnqueueRequestsFromMapFunc(c.endpointSliceMapFunc())).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions)}).
		Complete(c)
}

func (c *EndpointSliceController) unBoxingEndpointSlice(work *workv1alpha1.Work) error {
	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.Errorf("Failed to get cluster name for work %s/%s", work.Namespace, work.Name)
		return err
	}

	for _, manifest := range work.Spec.Workload.Manifests {
		unstructObj := &unstructured.Unstructured{}
		if err = unstructObj.UnmarshalJSON(manifest.Raw); err != nil {
			klog.Errorf("Failed to unmarshal workload, error is: %v", err)
			return err
		}

		endpointSlice := &discoveryv1.EndpointSlice{}
		err = helper.ConvertToTypedObject(unstructObj, endpointSlice)
		if err != nil {
			klog.Errorf("Failed to convert unstructured to typed object: %v", err)
			return err
		}

		mcsNamespacedName := types.NamespacedName{
			Namespace: endpointSlice.Namespace,
			Name:      work.Labels[util.ServiceNameLabel],
		}
		mcs := &networkingv1alpha1.MultiClusterService{}
		err = c.Get(context.TODO(), mcsNamespacedName, mcs)
		if err != nil {
			if apierrors.IsNotFound(err) {
				mcs = nil
			} else {
				klog.Errorf("Failed to get ref MultiClusterService(%s): %v", mcsNamespacedName.String(), err)
				return err
			}
		}

		desiredEndpointSlice := deriveEndpointSlice(endpointSlice, clusterName, mcs, work)
		if err = helper.CreateOrUpdateEndpointSlice(c.Client, desiredEndpointSlice); err != nil {
			return err
		}
	}

	return nil
}

func deriveEndpointSlice(
	eps *discoveryv1.EndpointSlice,
	cluster string,
	mcs *networkingv1alpha1.MultiClusterService,
	work *workv1alpha1.Work,
) *discoveryv1.EndpointSlice {
	eps.Name = names.GenerateEndpointSliceName(eps.Name, cluster)
	eps.Labels = map[string]string{
		workv1alpha1.WorkNamespaceLabel: work.Namespace,
		workv1alpha1.WorkNameLabel:      work.Name,
		discoveryv1.LabelServiceName:    getServiceName(mcs, work.Labels[util.ServiceNameLabel]),
	}
	return eps
}

func getServiceName(mcs *networkingv1alpha1.MultiClusterService, svcName string) string {
	if mcs != nil {
		strategy, exist := mcs.Annotations[mcsDiscoveryAnnotationKey]
		if exist && strategy == mcsDiscoveryRemoteAndLocalStrategy {
			return mcs.Name
		}
	}
	return names.GenerateDerivedServiceName(svcName)
}
