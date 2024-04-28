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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
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
			// Clean up derived EndpointSlices after work has been removed.
			endpointSlices := &discoveryv1.EndpointSliceList{}
			if err = c.List(context.TODO(), endpointSlices, client.HasLabels{workv1alpha2.WorkPermanentIDLabel}); err != nil {
				return controllerruntime.Result{}, err
			}

			var errs []error
			for i, es := range endpointSlices.Items {
				if es.Annotations[workv1alpha2.WorkNamespaceAnnotation] == req.Namespace &&
					es.Annotations[workv1alpha2.WorkNameAnnotation] == req.Name {
					if err := c.Delete(context.TODO(), &endpointSlices.Items[i]); err != nil && !apierrors.IsNotFound(err) {
						klog.Errorf("Failed to delete endpointslice(%s/%s) after the work(%s/%s) has been removed, err: %v",
							es.Namespace, es.Name, req.Namespace, req.Name, err)
						errs = append(errs, err)
					}
				}
			}
			return controllerruntime.Result{}, utilerrors.NewAggregate(errs)
		}
		return controllerruntime.Result{}, err
	}

	if !work.DeletionTimestamp.IsZero() {
		// Clean up derived EndpointSlices when deleting the work.
		if err := helper.DeleteEndpointSlice(c.Client, labels.Set{
			workv1alpha2.WorkPermanentIDLabel: work.Labels[workv1alpha2.WorkPermanentIDLabel],
		}); err != nil {
			klog.Errorf("Failed to delete endpointslice of the work(%s/%s) when deleting the work, err is %v", work.Namespace, work.Name, err)
			return controllerruntime.Result{}, err
		}
		return controllerruntime.Result{}, c.removeFinalizer(work.DeepCopy())
	}

	// TBD: The work is managed by service-export-controller and endpointslice-collect-controller now,
	// after the ServiceExport is deleted, the corresponding works' labels will be cleaned, so we should delete the EndpointSlice in control plane.
	// Once the conflict between service_export_controller.go and endpointslice_collect_controller.go is fixed, the following code should be deleted.
	if serviceName := util.GetLabelValue(work.Labels, util.ServiceNameLabel); serviceName == "" {
		err := helper.DeleteEndpointSlice(c.Client, labels.Set{
			workv1alpha2.WorkPermanentIDLabel: work.Labels[workv1alpha2.WorkPermanentIDLabel],
		})
		if err != nil {
			klog.Errorf("Failed to delete endpointslice of the work(%s/%s) when the serviceexport is deleted, err is %v", work.Namespace, work.Name, err)
			return controllerruntime.Result{}, err
		}
		return controllerruntime.Result{}, c.removeFinalizer(work.DeepCopy())
	}

	return controllerruntime.Result{}, c.collectEndpointSliceFromWork(work)
}

func (c *EndpointSliceController) removeFinalizer(work *workv1alpha1.Work) error {
	if !controllerutil.RemoveFinalizer(work, util.EndpointSliceControllerFinalizer) {
		return nil
	}
	return c.Client.Update(context.TODO(), work)
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
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}
	return controllerruntime.NewControllerManagedBy(mgr).For(&workv1alpha1.Work{}, builder.WithPredicates(serviceImportPredicateFun)).Complete(c)
}

func (c *EndpointSliceController) collectEndpointSliceFromWork(work *workv1alpha1.Work) error {
	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.Errorf("Failed to get cluster name for work %s/%s", work.Namespace, work.Name)
		return err
	}

	for _, manifest := range work.Spec.Workload.Manifests {
		unstructObj := &unstructured.Unstructured{}
		if err := unstructObj.UnmarshalJSON(manifest.Raw); err != nil {
			klog.Errorf("Failed to unmarshal workload, error is: %v", err)
			return err
		}

		endpointSlice := &discoveryv1.EndpointSlice{}
		err = helper.ConvertToTypedObject(unstructObj, endpointSlice)
		if err != nil {
			klog.Errorf("Failed to convert unstructured to typed object: %v", err)
			return err
		}

		desiredEndpointSlice := deriveEndpointSlice(endpointSlice, clusterName)
		desiredEndpointSlice.Labels = util.DedupeAndMergeLabels(desiredEndpointSlice.Labels, map[string]string{
			workv1alpha2.WorkPermanentIDLabel: work.Labels[workv1alpha2.WorkPermanentIDLabel],
			discoveryv1.LabelServiceName:      names.GenerateDerivedServiceName(work.Labels[util.ServiceNameLabel]),
			util.ManagedByKarmadaLabel:        util.ManagedByKarmadaLabelValue,
		})

		if err = helper.CreateOrUpdateEndpointSlice(c.Client, desiredEndpointSlice); err != nil {
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
