package mcs

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// EndpointSliceControllerName is the controller name that will be used when reporting events.
const EndpointSliceControllerName = "endpointslice-controller"

// EndpointSliceController is to collect EndpointSlice which reported by member cluster from executionNamespace to serviceexport namespace.
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

	return c.collectEndpointSliceFromWork(work)
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

func (c *EndpointSliceController) collectEndpointSliceFromWork(work *workv1alpha1.Work) (controllerruntime.Result, error) {
	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.Errorf("Failed to get cluster name for work %s/%s", work.Namespace, work.Name)
		return controllerruntime.Result{Requeue: true}, err
	}

	for _, manifest := range work.Spec.Workload.Manifests {
		unstructObj := &unstructured.Unstructured{}
		if err := unstructObj.UnmarshalJSON(manifest.Raw); err != nil {
			klog.Errorf("Failed to unmarshal workload, error is: %v", err)
			return controllerruntime.Result{Requeue: true}, err
		}

		endpointSlice := &discoveryv1.EndpointSlice{}
		err = helper.ConvertToTypedObject(unstructObj, endpointSlice)
		if err != nil {
			klog.Errorf("failed to convert unstructured to typed object: %v", err)
			return controllerruntime.Result{Requeue: true}, err
		}

		desiredEndpointSlice := deriveEndpointSlice(endpointSlice, clusterName)
		desiredEndpointSlice.Labels = map[string]string{
			workv1alpha1.WorkNamespaceLabel: work.Namespace,
			workv1alpha1.WorkNameLabel:      work.Name,
			discoveryv1.LabelServiceName:    names.GenerateDerivedServiceName(work.Labels[util.ServiceNameLabel]),
		}

		if err = helper.CreateOrUpdateEndpointSlice(c.Client, desiredEndpointSlice); err != nil {
			return controllerruntime.Result{Requeue: true}, err
		}
	}

	return controllerruntime.Result{}, nil
}

func deriveEndpointSlice(original *discoveryv1.EndpointSlice, migratedFrom string) *discoveryv1.EndpointSlice {
	endpointSlice := original.DeepCopy()
	endpointSlice.ObjectMeta = metav1.ObjectMeta{
		Namespace: original.Namespace,
		Name:      names.GenerateEndpointSliceName(original.GetName(), migratedFrom),
	}

	return endpointSlice
}
