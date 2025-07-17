/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// EndpointSliceControllerName is the controller name that will be used when reporting events and metrics.
const EndpointSliceControllerName = "endpointslice-controller"

// EndpointSliceController is to collect EndpointSlice which reported by member cluster from executionNamespace to serviceexport namespace.
type EndpointSliceController struct {
	client.Client
	EventRecorder      record.EventRecorder
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
func (c *EndpointSliceController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling Work", "namespace", req.NamespacedName.Namespace, "name", req.NamespacedName.Name)

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, req.NamespacedName, work); err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	if !work.DeletionTimestamp.IsZero() {
		// Clean up derived EndpointSlices when deleting the work.
		if err := helper.DeleteEndpointSlice(ctx, c.Client, labels.Set{
			workv1alpha2.WorkPermanentIDLabel: work.Labels[workv1alpha2.WorkPermanentIDLabel],
		}); err != nil {
			klog.ErrorS(err, "Failed to delete endpointslice when deleting work", "namespace",
				work.Namespace, "workName", work.Name)
			return controllerruntime.Result{}, err
		}
		return controllerruntime.Result{}, c.removeFinalizer(ctx, work.DeepCopy())
	}

	// TBD: The work is managed by service-export-controller and endpointslice-collect-controller now,
	// after the ServiceExport is deleted, the corresponding works' labels will be cleaned, so we should delete the EndpointSlice in control plane.
	// Once the conflict between service_export_controller.go and endpointslice_collect_controller.go is fixed, the following code should be deleted.
	if serviceName := util.GetLabelValue(work.Labels, util.ServiceNameLabel); serviceName == "" {
		err := helper.DeleteEndpointSlice(ctx, c.Client, labels.Set{
			workv1alpha2.WorkPermanentIDLabel: work.Labels[workv1alpha2.WorkPermanentIDLabel],
		})
		if err != nil {
			klog.ErrorS(err, "Failed to delete endpointslice when serviceexport is deleted", "namespace", work.Namespace, "workName", work.Name)
			return controllerruntime.Result{}, err
		}
		return controllerruntime.Result{}, c.removeFinalizer(ctx, work.DeepCopy())
	}

	return controllerruntime.Result{}, c.collectEndpointSliceFromWork(ctx, work)
}

func (c *EndpointSliceController) removeFinalizer(ctx context.Context, work *workv1alpha1.Work) error {
	if !controllerutil.RemoveFinalizer(work, util.EndpointSliceControllerFinalizer) {
		return nil
	}
	return c.Client.Update(ctx, work)
}

// SetupWithManager creates a controller and register to controller manager.
func (c *EndpointSliceController) SetupWithManager(mgr controllerruntime.Manager) error {
	serviceImportPredicateFun := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return util.GetLabelValue(createEvent.Object.GetLabels(), util.ServiceNameLabel) != ""
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			// TBD: We care about the work with label util.MultiClusterServiceNameLabel because the work is
			// managed by service-export-controller and endpointslice-collect-controller now, We should delete this after the conflict is fixed.
			return util.GetLabelValue(updateEvent.ObjectNew.GetLabels(), util.ServiceNameLabel) != "" ||
				util.GetLabelValue(updateEvent.ObjectNew.GetLabels(), util.MultiClusterServiceNameLabel) != ""
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return util.GetLabelValue(deleteEvent.Object.GetLabels(), util.ServiceNameLabel) != ""
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(EndpointSliceControllerName).
		For(&workv1alpha1.Work{}, builder.WithPredicates(serviceImportPredicateFun)).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions),
		}).
		Complete(c)
}

func (c *EndpointSliceController) collectEndpointSliceFromWork(ctx context.Context, work *workv1alpha1.Work) error {
	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.ErrorS(err, "Failed to get cluster name for work", "namespace", work.Namespace, "workName", work.Name)
		return err
	}

	for _, manifest := range work.Spec.Workload.Manifests {
		unstructObj := &unstructured.Unstructured{}
		if err := unstructObj.UnmarshalJSON(manifest.Raw); err != nil {
			klog.ErrorS(err, "Failed to unmarshal workload")
			return err
		}

		endpointSlice := &discoveryv1.EndpointSlice{}
		err = helper.ConvertToTypedObject(unstructObj, endpointSlice)
		if err != nil {
			klog.ErrorS(err, "Failed to convert unstructured to typed object")
			return err
		}

		desiredEndpointSlice := deriveEndpointSlice(endpointSlice, clusterName)
		desiredEndpointSlice.Labels = util.DedupeAndMergeLabels(desiredEndpointSlice.Labels, map[string]string{
			workv1alpha2.WorkPermanentIDLabel: work.Labels[workv1alpha2.WorkPermanentIDLabel],
			discoveryv1.LabelServiceName:      names.GenerateDerivedServiceName(work.Labels[util.ServiceNameLabel]),
		})
		desiredEndpointSlice.Annotations = util.DedupeAndMergeAnnotations(desiredEndpointSlice.Annotations, map[string]string{
			workv1alpha2.WorkNamespaceAnnotation: work.Namespace,
			workv1alpha2.WorkNameAnnotation:      work.Name,
		})

		if err = helper.CreateOrUpdateEndpointSlice(ctx, c.Client, desiredEndpointSlice); err != nil {
			klog.ErrorS(err, "Failed to create or update endpointslice", "namespace", desiredEndpointSlice.Namespace, "name", desiredEndpointSlice.Name)
			return err
		}
	}

	return nil
}

func deriveEndpointSlice(original *discoveryv1.EndpointSlice, migratedFrom string) *discoveryv1.EndpointSlice {
	endpointSlice := original.DeepCopy()
	endpointSlice.ObjectMeta = metav1.ObjectMeta{
		Namespace: original.Namespace,
		Name:      names.GenerateEndpointSliceName(original.GetName(), migratedFrom),
	}

	return endpointSlice
}
