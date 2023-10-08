package fedinformer

import (
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// StripUnusedFields is the transform function for shared informers,
// it removes unused fields from objects before they are stored in the cache to save memory.
func StripUnusedFields(obj interface{}) (interface{}, error) {
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

// NodeTransformFunc is the dedicated transform function for Node objects.
// It cleans up some parts of the object before it will be put into the controller
// cache to reduce memory usage.
//
// Note: this function removes most of the fields, please make sure your controller
// doesn't care for the removed fields, especially when use in shared informers.
func NodeTransformFunc(obj interface{}) (interface{}, error) {
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
func PodTransformFunc(obj interface{}) (interface{}, error) {
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
