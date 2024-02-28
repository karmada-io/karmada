/*
Copyright 2024 The Karmada Authors.

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

package store

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listcorev1 "k8s.io/client-go/listers/core/v1"

	"github.com/karmada-io/karmada/pkg/util/helper"
)

type nodeStore struct {
	nodeClient     typedcorev1.NodeInterface
	nodeLister     listcorev1.NodeLister
	conditionType  corev1.NodeConditionType
	staleThreshold time.Duration
}

// NewNodeConditionStore returns a store for node conditions.
func NewNodeConditionStore(client typedcorev1.NodeInterface, lister listcorev1.NodeLister, conditionType string, staleThreshold time.Duration) ConditionStore {
	return &nodeStore{
		nodeClient:     client,
		nodeLister:     lister,
		conditionType:  corev1.NodeConditionType(conditionType),
		staleThreshold: staleThreshold,
	}
}

func (n *nodeStore) Load(key string) (*metav1.Condition, error) {
	node, err := n.nodeLister.Get(key)
	if err != nil {
		return nil, err
	}
	return NodeCondition2MetaV1Condition(FindNodeStatusCondition(node.Status.Conditions, n.conditionType)), nil
}

func (n *nodeStore) Store(key string, cond *metav1.Condition) error {
	node, err := n.nodeLister.Get(key)
	if err != nil {
		return err
	}
	// do not modify on node returned from lister
	node = node.DeepCopy()
	SetNodeStatusCondition(&node.Status.Conditions, MetaV1Condition2NodeCondition(*cond))
	_, err = n.nodeClient.UpdateStatus(context.Background(), node, metav1.UpdateOptions{})
	return err
}

func (n *nodeStore) ListAll() ([]metav1.Condition, error) {
	nodes, err := n.nodeLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	conditions := make([]metav1.Condition, 0, len(nodes))
	for _, node := range nodes {
		if !helper.NodeReady(node) {
			// skip node if not ready
			continue
		}
		nodeCond := FindNodeStatusCondition(node.Status.Conditions, n.conditionType)
		if nodeCond == nil || time.Since(nodeCond.LastHeartbeatTime.Time) > n.staleThreshold {
			// if the node condition was absent or was too stale, we regard it as Unknown.
			nodeCond = &corev1.NodeCondition{Type: n.conditionType, Status: corev1.ConditionUnknown}
		}
		conditions = append(conditions, *NodeCondition2MetaV1Condition(nodeCond))
	}
	return conditions, nil
}

// MetaV1Condition2NodeCondition converts metav1.Condition to corev1.NodeCondition.
func MetaV1Condition2NodeCondition(cond metav1.Condition) corev1.NodeCondition {
	return corev1.NodeCondition{
		Type:               corev1.NodeConditionType(cond.Type),
		Status:             corev1.ConditionStatus(cond.Status),
		LastTransitionTime: cond.LastTransitionTime,
		Reason:             cond.Reason,
		Message:            cond.Message,
	}
}

// NodeCondition2MetaV1Condition converts corev1.NodeCondition to metav1.Condition.
func NodeCondition2MetaV1Condition(cond *corev1.NodeCondition) *metav1.Condition {
	if cond == nil {
		return nil
	}
	return &metav1.Condition{
		Type:               string(cond.Type),
		Status:             metav1.ConditionStatus(cond.Status),
		LastTransitionTime: cond.LastTransitionTime,
		Reason:             cond.Reason,
		Message:            cond.Message,
	}
}

// SetNodeStatusCondition set the condition into node conditions.
func SetNodeStatusCondition(conditions *[]corev1.NodeCondition, newCondition corev1.NodeCondition) {
	if conditions == nil {
		return
	}
	existingCondition := FindNodeStatusCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		if newCondition.LastTransitionTime.IsZero() {
			newCondition.LastTransitionTime = metav1.Now()
		}
		newCondition.LastHeartbeatTime = metav1.Now()
		*conditions = append(*conditions, newCondition)
		return
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		if !newCondition.LastTransitionTime.IsZero() {
			existingCondition.LastTransitionTime = newCondition.LastTransitionTime
		} else {
			existingCondition.LastTransitionTime = metav1.NewTime(time.Now())
		}
	}

	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
	existingCondition.LastHeartbeatTime = metav1.Now()
}

// FindNodeStatusCondition returns the condition with the specific type.
func FindNodeStatusCondition(conditions []corev1.NodeCondition, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
