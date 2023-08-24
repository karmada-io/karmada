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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// EndpointSliceControllerName is the controller name that will be used when reporting events.
const EndpointSliceControllerName = "endpointslice-controller"

const (
	mcsDiscoveryAnnotationKey = "discovery.karmada.io/strategy"

	mcsDiscoveryRemoteAndLocalStrategy = "RemoteAndLocal"
)

// EndpointSliceController is to collect EndpointSlice which reported by member cluster
// from executionNamespace to serviceexport namespace.
type EndpointSliceController struct {
	client.Client
	EventRecorder record.EventRecorder
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

// SetupWithManager creates a controller and register to controller manager.
func (c *EndpointSliceController) SetupWithManager(mgr controllerruntime.Manager) error {
	serviceImportPredicateFun := predicate.Funcs{
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
	return controllerruntime.NewControllerManagedBy(mgr).For(&workv1alpha1.Work{}, builder.WithPredicates(serviceImportPredicateFun)).Complete(c)
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
			klog.Errorf("Failed to get ref MultiClusterService(%s): %v", mcsNamespacedName.String(), err)
			return err
		}

		desiredEndpointSlice := deriveEndpointSlice(endpointSlice, clusterName, mcs)
		if err = helper.CreateOrUpdateEndpointSlice(c.Client, desiredEndpointSlice); err != nil {
			return err
		}
	}

	return nil
}

func deriveEndpointSlice(eps *discoveryv1.EndpointSlice, cluster string, mcs *networkingv1alpha1.MultiClusterService) *discoveryv1.EndpointSlice {
	eps.Name = names.GenerateEndpointSliceName(eps.Name, cluster)
	eps.Labels = map[string]string{
		discoveryv1.LabelServiceName: labelServiceName(mcs),
	}
	return eps
}

func labelServiceName(mcs *networkingv1alpha1.MultiClusterService) string {
	strategy, exist := mcs.Annotations[mcsDiscoveryAnnotationKey]
	if exist && strategy == mcsDiscoveryRemoteAndLocalStrategy {
		return mcs.Name
	}
	return names.GenerateDerivedServiceName(mcs.Name)
}
