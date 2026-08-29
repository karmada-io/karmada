/*
Copyright 2022 The Karmada Authors.

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

package fedinformer

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// StripUnusedFields is the transform function for shared informers,
// it removes unused fields from objects before they are stored in the cache to save memory.
func StripUnusedFields(obj any) (any, error) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		// shouldn't happen
		return obj, nil
	}
	// ManagedFields is large and we never use it
	accessor.SetManagedFields(nil)
	return obj, nil
}

// RetainMetadataFields keeps only type and object metadata on informer objects.
func RetainMetadataFields(obj any) (any, error) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return obj, nil
	}
	return metadataOnlyUnstructured(unstructuredObj), nil
}

func metadataOnlyUnstructured(obj *unstructured.Unstructured) *unstructured.Unstructured {
	obj.Object = map[string]any{
		"apiVersion": obj.GetAPIVersion(),
		"kind":       obj.GetKind(),
		"metadata":   obj.Object["metadata"],
	}
	return obj
}

// NodeTransformFunc is the dedicated transform function for Node objects.
// It cleans up some parts of the object before it will be put into the controller
// cache to reduce memory usage.
//
// Note: this function removes most of the fields, please make sure your controller
// doesn't care for the removed fields, especially when use in shared informers.
func NodeTransformFunc(obj any) (any, error) {
	var node *corev1.Node
	switch t := obj.(type) {
	case *corev1.Node:
		node = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		node, ok = t.Obj.(*corev1.Node)
		if !ok {
			return obj, fmt.Errorf("expect resource Node but got %v", reflect.TypeOf(t.Obj))
		}
	default:
		return obj, fmt.Errorf("expect resource Node but got %v", reflect.TypeOf(obj))
	}

	aggregatedNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.Name,
		},
		Status: corev1.NodeStatus{
			Allocatable: node.Status.Allocatable,
			Conditions:  node.Status.Conditions,
		},
	}
	return aggregatedNode, nil
}

// PodTransformFunc is the dedicated transform function for Pod objects.
// It cleans up some parts of the object before it will be put into the controller
// cache to reduce memory usage.
//
// Note: this function removes most of the fields, please make sure your controller
// doesn't care for the removed fields, especially when use in shared informers.
func PodTransformFunc(obj any) (any, error) {
	var pod *corev1.Pod
	switch t := obj.(type) {
	case *corev1.Pod:
		pod = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*corev1.Pod)
		if !ok {
			return obj, fmt.Errorf("expect resource Pod but got %v", reflect.TypeOf(t.Obj))
		}
	default:
		return obj, fmt.Errorf("expect resource Pod but got %v", reflect.TypeOf(obj))
	}

	aggregatedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              pod.Name,
			Namespace:         pod.Namespace,
			Labels:            pod.Labels,
			DeletionTimestamp: pod.DeletionTimestamp,
		},
		Spec: corev1.PodSpec{
			NodeName:       pod.Spec.NodeName,
			InitContainers: pod.Spec.InitContainers,
			Containers:     pod.Spec.Containers,
			Overhead:       pod.Spec.Overhead,
		},
		Status: corev1.PodStatus{
			Phase:      pod.Status.Phase,
			Conditions: pod.Status.Conditions,
			StartTime:  pod.Status.StartTime,
		},
	}
	return aggregatedPod, nil
}

// NewWorkMappingTransformFunc keeps complete member-cluster objects only when
// they are associated with an existing Work; unrelated objects retain metadata only.
func NewWorkMappingTransformFunc(controlPlaneClient client.Reader) cache.TransformFunc {
	return func(obj any) (any, error) {
		if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			obj = tombstone.Obj
		}

		accessor, err := meta.Accessor(obj)
		if err != nil {
			// shouldn't happen
			return obj, nil
		}
		// ManagedFields is large and we never use it
		accessor.SetManagedFields(nil)

		mappedToWork, err := mapsToWork(controlPlaneClient, obj)
		if err != nil {
			return obj, err
		}
		if mappedToWork {
			return obj, nil
		}
		return RetainMetadataFields(obj)
	}
}

func mapsToWork(controlPlaneClient client.Reader, obj any) (bool, error) {
	resource, err := meta.Accessor(obj)
	if err != nil {
		return false, nil
	}
	annotations := resource.GetAnnotations()
	workNamespace, namespaceExists := annotations[workv1alpha2.WorkNamespaceAnnotation]
	workName, nameExists := annotations[workv1alpha2.WorkNameAnnotation]
	if !namespaceExists || !nameExists {
		return false, nil
	}

	work := &workv1alpha1.Work{}
	if err := controlPlaneClient.Get(context.Background(), client.ObjectKey{Namespace: workNamespace, Name: workName}, work); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}
