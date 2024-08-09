/*
Copyright 2023 The Karmada Authors.

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

package status

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

var bindingPredicateFn = builder.WithPredicates(predicate.Funcs{
	CreateFunc: func(event.CreateEvent) bool { return false },
	UpdateFunc: func(e event.UpdateEvent) bool {
		var oldResourceVersion, newResourceVersion string

		// NOTE: We add this logic to prevent the situation as following:
		// 1. Create Deployment and HPA.
		// 2. Propagate Deployment with divided policy and propagate HPA with duplicated policy.
		// 3. HPA scaled up the Deployment in member clusters
		// 4. deploymentreplicassyncer update Deployment's replicas based on Deployment's status in Kamarda control plane.
		// 5. It will cause the Deployment.status.observedGeneration != Deployment.metadata.generation(this is needed in some workflow cases)
		switch oldBinding := e.ObjectOld.(type) {
		case *workv1alpha2.ResourceBinding:
			oldResourceVersion = oldBinding.Spec.Resource.ResourceVersion
		case *workv1alpha2.ClusterResourceBinding:
			oldResourceVersion = oldBinding.Spec.Resource.ResourceVersion
		default:
			return false
		}

		switch newBinding := e.ObjectNew.(type) {
		case *workv1alpha2.ResourceBinding:
			newResourceVersion = newBinding.Spec.Resource.ResourceVersion
		case *workv1alpha2.ClusterResourceBinding:
			newResourceVersion = newBinding.Spec.Resource.ResourceVersion
		default:
			return false
		}

		return oldResourceVersion != newResourceVersion
	},
	DeleteFunc: func(event.DeleteEvent) bool { return false },
})

var workPredicateFn = builder.WithPredicates(predicate.Funcs{
	CreateFunc: func(event.CreateEvent) bool { return false },
	UpdateFunc: func(e event.UpdateEvent) bool {
		var oldStatus, newStatus workv1alpha1.WorkStatus

		switch oldWork := e.ObjectOld.(type) {
		case *workv1alpha1.Work:
			oldStatus = oldWork.Status
		default:
			return false
		}

		switch newWork := e.ObjectNew.(type) {
		case *workv1alpha1.Work:
			newStatus = newWork.Status
		default:
			return false
		}

		return !reflect.DeepEqual(oldStatus, newStatus)
	},
	DeleteFunc:  func(event.DeleteEvent) bool { return true },
	GenericFunc: func(event.GenericEvent) bool { return false },
})

// updateResourceStatus will try to calculate the summary status and
// update to original object that the ResourceBinding refer to.
func updateResourceStatus(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	restMapper meta.RESTMapper,
	interpreter resourceinterpreter.ResourceInterpreter,
	eventRecorder record.EventRecorder,
	objRef workv1alpha2.ObjectReference,
	bindingStatus workv1alpha2.ResourceBindingStatus,
) error {
	gvr, err := restmapper.GetGroupVersionResource(restMapper, schema.FromAPIVersionAndKind(objRef.APIVersion, objRef.Kind))
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK(%s/%s), Error: %v", objRef.APIVersion, objRef.Kind, err)
		return err
	}

	gvk := schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: objRef.Kind}
	if !interpreter.HookEnabled(gvk, configv1alpha1.InterpreterOperationAggregateStatus) {
		return nil
	}

	// get resource template directly from apiserver, rather than informer cache, detail refers to:
	// https://github.com/karmada-io/karmada/issues/5285
	resource, err := dynamicClient.Resource(gvr).Namespace(objRef.Namespace).Get(ctx, objRef.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// It might happen when the resource template has been removed but the garbage collector hasn't removed
			// the ResourceBinding which dependent on resource template.
			// So, just return without retry(requeue) would save unnecessary loop.
			return nil
		}
		klog.Errorf("Failed to fetch resource(%s/%s) for resourceBinding. Error: %v.", resource.GetNamespace(), resource.GetName(), err)
		return err
	}

	newObj, err := interpreter.AggregateStatus(resource, bindingStatus.AggregatedStatus)
	if err != nil {
		klog.Errorf("Failed to aggregate status for resource(%s/%s/%s), Error: %v", gvr, resource.GetNamespace(), resource.GetName(), err)
		return err
	}

	if reflect.DeepEqual(resource, newObj) {
		klog.V(3).Infof("Ignore update resource(%s/%s/%s) status as up to date.", gvr, resource.GetNamespace(), resource.GetName())
		return nil
	}

	if _, err = dynamicClient.Resource(gvr).Namespace(resource.GetNamespace()).UpdateStatus(ctx, newObj, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("Failed to update resource(%s/%s/%s), Error: %v", gvr, resource.GetNamespace(), resource.GetName(), err)
		return err
	}

	eventRecorder.Event(resource, corev1.EventTypeNormal, events.EventReasonAggregateStatusSucceed, "Update Resource with AggregatedStatus successfully.")
	klog.V(3).Infof("Update resource(%s/%s/%s) status successfully.", gvr, resource.GetNamespace(), resource.GetName())

	return nil
}
