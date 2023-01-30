// This code is mostly lifted from the Kubernetes codebase to establish the internal cache.
// https://github.com/kubernetes/kubernetes/blob/release-1.25/pkg/scheduler/eventhandlers.go

package server

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func addAllEventHandlers(es *AccurateSchedulerEstimatorServer, informerFactory informers.SharedInformerFactory) {
	// scheduled pod cache
	_, err := informerFactory.Core().V1().Pods().Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *corev1.Pod:
					return assignedPod(t)
				case cache.DeletedFinalStateUnknown:
					if _, ok := t.Obj.(*corev1.Pod); ok {
						// The carried object may be stale, so we don't use it to check if
						// it's assigned or not. Attempting to cleanup anyways.
						return true
					}
					utilruntime.HandleError(fmt.Errorf("unable to convert object %T to *v1.Pod", obj))
					return false
				default:
					utilruntime.HandleError(fmt.Errorf("unable to handle object: %T", obj))
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    es.addPodToCache,
				UpdateFunc: es.updatePodInCache,
				DeleteFunc: es.deletePodFromCache,
			},
		},
	)
	if err != nil {
		klog.Errorf("Failed to add handler for Pods: %v", err)
	}

	_, err = informerFactory.Core().V1().Nodes().Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    es.addNodeToCache,
			UpdateFunc: es.updateNodeInCache,
			DeleteFunc: es.deleteNodeFromCache,
		},
	)
	if err != nil {
		klog.Errorf("Failed to add handler for Pods: %v", err)
	}
}

func (es *AccurateSchedulerEstimatorServer) addPodToCache(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.ErrorS(nil, "Cannot convert to *v1.Pod", "obj", obj)
		return
	}
	klog.V(3).InfoS("Add event for scheduled pod", "pod", klog.KObj(pod))

	if err := es.Cache.AddPod(pod); err != nil {
		klog.ErrorS(err, "Estimator cache AddPod failed", "pod", klog.KObj(pod))
	}
}

func (es *AccurateSchedulerEstimatorServer) updatePodInCache(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*corev1.Pod)
	if !ok {
		klog.ErrorS(nil, "Cannot convert oldObj to *v1.Pod", "oldObj", oldObj)
		return
	}
	newPod, ok := newObj.(*corev1.Pod)
	if !ok {
		klog.ErrorS(nil, "Cannot convert newObj to *v1.Pod", "newObj", newObj)
		return
	}
	klog.V(4).InfoS("Update event for scheduled pod", "pod", klog.KObj(oldPod))

	if err := es.Cache.UpdatePod(oldPod, newPod); err != nil {
		klog.ErrorS(err, "Estimator cache UpdatePod failed", "pod", klog.KObj(oldPod))
	}
}

func (es *AccurateSchedulerEstimatorServer) deletePodFromCache(obj interface{}) {
	var pod *corev1.Pod
	switch t := obj.(type) {
	case *corev1.Pod:
		pod = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*corev1.Pod)
		if !ok {
			klog.ErrorS(nil, "Cannot convert to *v1.Pod", "obj", t.Obj)
			return
		}
	default:
		klog.ErrorS(nil, "Cannot convert to *v1.Pod", "obj", t)
		return
	}
	klog.V(3).InfoS("Delete event for scheduled pod", "pod", klog.KObj(pod))
	if err := es.Cache.RemovePod(pod); err != nil {
		klog.ErrorS(err, "Estimator cache RemovePod failed", "pod", klog.KObj(pod))
	}
}

func (es *AccurateSchedulerEstimatorServer) addNodeToCache(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.ErrorS(nil, "Cannot convert to *v1.Node", "obj", obj)
		return
	}

	es.Cache.AddNode(node)
	klog.V(3).InfoS("Add event for node", "node", klog.KObj(node))
}

func (es *AccurateSchedulerEstimatorServer) updateNodeInCache(oldObj, newObj interface{}) {
	oldNode, ok := oldObj.(*corev1.Node)
	if !ok {
		klog.ErrorS(nil, "Cannot convert oldObj to *v1.Node", "oldObj", oldObj)
		return
	}
	newNode, ok := newObj.(*corev1.Node)
	if !ok {
		klog.ErrorS(nil, "Cannot convert newObj to *v1.Node", "newObj", newObj)
		return
	}

	es.Cache.UpdateNode(oldNode, newNode)
}

func (es *AccurateSchedulerEstimatorServer) deleteNodeFromCache(obj interface{}) {
	var node *corev1.Node
	switch t := obj.(type) {
	case *corev1.Node:
		node = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		node, ok = t.Obj.(*corev1.Node)
		if !ok {
			klog.ErrorS(nil, "Cannot convert to *v1.Node", "obj", t.Obj)
			return
		}
	default:
		klog.ErrorS(nil, "Cannot convert to *v1.Node", "obj", t)
		return
	}
	klog.V(3).InfoS("Delete event for node", "node", klog.KObj(node))
	if err := es.Cache.RemoveNode(node); err != nil {
		klog.ErrorS(err, "Scheduler cache RemoveNode failed")
	}
}

// assignedPod selects pods that are assigned (scheduled and running).
func assignedPod(pod *corev1.Pod) bool {
	return len(pod.Spec.NodeName) != 0
}
