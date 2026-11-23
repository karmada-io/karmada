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

package replica

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	listappsv1 "k8s.io/client-go/listers/apps/v1"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	utilworkload "github.com/karmada-io/karmada/pkg/util/lifted"
)

// appsDeploymentGroupKind identifies the built-in Deployment. A custom resource that
// is also named Deployment belongs to another group and is not handled like it.
var appsDeploymentGroupKind = schema.GroupKind{Group: appsv1.GroupName, Kind: util.DeploymentKind}

// PodsByControllerUIDIndex is the name of the pod informer index that maps the UID
// of a controller to the pods it controls.
const PodsByControllerUIDIndex = "byControllerUID"

// IndexPodByControllerUID is the index function for PodsByControllerUIDIndex.
func IndexPodByControllerUID(obj any) ([]string, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, nil
	}
	ref := metav1.GetControllerOf(pod)
	if ref == nil {
		return nil, nil
	}
	return []string{string(ref.UID)}, nil
}

// ListerWrapper is a wrapper which wraps the pod lister and replicaset lister.
type ListerWrapper struct {
	listcorev1.PodLister
	listappsv1.ReplicaSetLister
	// PodIndexer is the indexer of the pod informer, with PodsByControllerUIDIndex.
	PodIndexer cache.Indexer
}

// GetUnschedulablePodsOfWorkload will return how many unschedulable pods a workload derives.
func GetUnschedulablePodsOfWorkload(unstructObj *unstructured.Unstructured, threshold time.Duration, listers *ListerWrapper) (int32, error) {
	if threshold < 0 {
		threshold = 0
	}
	unschedulable := 0
	// Workloads could be classified into two types. The one is which owns ReplicaSet
	// and the other is which owns Pod directly. Deployments go through their newest
	// ReplicaSet; any other kind is matched by controller owner references.
	switch unstructObj.GroupVersionKind().GroupKind() {
	case appsDeploymentGroupKind:
		deployment := &appsv1.Deployment{}
		if err := helper.ConvertToTypedObject(unstructObj, deployment); err != nil {
			return 0, fmt.Errorf("failed to convert ReplicaSet from unstructured object: %v", err)
		}
		pods, err := listDeploymentPods(deployment, listers)
		if err != nil {
			return 0, err
		}
		for _, pod := range pods {
			if podUnschedulable(pod, threshold) {
				unschedulable++
			}
		}
	default:
		pods, err := listControlledPods(unstructObj, listers)
		if err != nil {
			return 0, err
		}
		for _, pod := range pods {
			if podUnschedulable(pod, threshold) {
				unschedulable++
			}
		}
	}
	return int32(unschedulable), nil // #nosec G115: integer overflow conversion int -> int32
}

func podUnschedulable(pod *corev1.Pod, threshold time.Duration) bool {
	_, cond := helper.GetPodCondition(&pod.Status, corev1.PodScheduled)
	return cond != nil && cond.Status == corev1.ConditionFalse && cond.Reason == corev1.PodReasonUnschedulable &&
		cond.LastTransitionTime.Add(threshold).Before(time.Now())
}

func listDeploymentPods(deployment *appsv1.Deployment, listers *ListerWrapper) ([]*corev1.Pod, error) {
	// Get ReplicaSet
	rsListFunc := func(namespace string, selector labels.Selector) ([]*appsv1.ReplicaSet, error) {
		return listers.ReplicaSetLister.ReplicaSets(namespace).List(selector)
	}

	rs, err := utilworkload.GetNewReplicaSet(deployment, rsListFunc)
	if err != nil {
		return nil, err
	}

	podListFunc := func(namespace string, selector labels.Selector) ([]*corev1.Pod, error) {
		return listers.PodLister.Pods(namespace).List(selector)
	}

	pods, err := utilworkload.ListPodsByRS(deployment, []*appsv1.ReplicaSet{rs}, podListFunc)
	if err != nil {
		return nil, err
	}
	return pods, nil
}

// listControlledPods returns the pods that the workload controls, either directly
// or through a ReplicaSet that it controls. This covers custom workloads (CRDs)
// that create pods themselves, like most batch and AI operators do, and the ones
// that create ReplicaSets the way a Deployment does. Pods are looked up by the UID
// of their controller in the pod informer index, so no namespace is listed in full.
func listControlledPods(workload *unstructured.Unstructured, listers *ListerWrapper) ([]*corev1.Pod, error) {
	uid := workload.GetUID()
	if uid == "" {
		return nil, fmt.Errorf("workload(%s) of kind(%s) has no UID", klog.KObj(workload).String(), workload.GetKind())
	}
	if listers.PodIndexer == nil {
		return nil, fmt.Errorf("pod indexer with index %q is not set", PodsByControllerUIDIndex)
	}

	res, err := podsControlledBy(listers.PodIndexer, workload.GetNamespace(), uid)
	if err != nil {
		return nil, err
	}

	replicaSets, err := listers.ReplicaSetLister.ReplicaSets(workload.GetNamespace()).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, rs := range replicaSets {
		ref := metav1.GetControllerOf(rs)
		if ref == nil || ref.UID != uid {
			continue
		}
		pods, err := podsControlledBy(listers.PodIndexer, workload.GetNamespace(), rs.UID)
		if err != nil {
			return nil, err
		}
		res = append(res, pods...)
	}
	return res, nil
}

// podsControlledBy returns the pods in namespace whose controller has the given UID.
func podsControlledBy(indexer cache.Indexer, namespace string, uid types.UID) ([]*corev1.Pod, error) {
	objs, err := indexer.ByIndex(PodsByControllerUIDIndex, string(uid))
	if err != nil {
		return nil, err
	}
	var pods []*corev1.Pod
	for _, obj := range objs {
		if pod, ok := obj.(*corev1.Pod); ok && pod.Namespace == namespace {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}
