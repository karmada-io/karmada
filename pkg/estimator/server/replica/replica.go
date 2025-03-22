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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	listappsv1 "k8s.io/client-go/listers/apps/v1"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	utilworkload "github.com/karmada-io/karmada/pkg/util/lifted"
)

// ListerWrapper is a wrapper which wraps the pod lister and replicaset lister.
type ListerWrapper struct {
	listcorev1.PodLister
	listappsv1.ReplicaSetLister
}

// GetUnschedulablePodsOfWorkload will return how many unschedulable pods a workload derives.
func GetUnschedulablePodsOfWorkload(unstructObj *unstructured.Unstructured, threshold time.Duration, listers *ListerWrapper) (int32, error) {
	if threshold < 0 {
		threshold = 0
	}
	unschedulable := 0
	// Workloads could be classified into two types. The one is which owns ReplicaSet
	// and the other is which owns Pod directly.
	switch unstructObj.GetKind() {
	case util.DeploymentKind:
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
		// TODO(Garrybest): add abstract workload
		return 0, fmt.Errorf("kind(%s) of workload(%s) is not supported", unstructObj.GetKind(), klog.KObj(unstructObj).String())
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
