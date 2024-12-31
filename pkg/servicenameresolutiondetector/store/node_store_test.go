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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestNodeStore(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	nodeInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{},
		&corev1.Node{},
		0,
		cache.Indexers{},
	)
	nodeLister := listcorev1.NewNodeLister(nodeInformer.GetIndexer())

	store := NewNodeConditionStore(
		clientset.CoreV1().Nodes(),
		nodeLister,
		string(corev1.NodeReady),
		5*time.Minute,
	)

	testCases := []struct {
		name           string
		setupNodes     []*corev1.Node
		operation      string
		key            string
		inputCondition *metav1.Condition
		expectedResult interface{}
		expectedError  bool
	}{
		{
			name: "Load existing node condition",
			setupNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
						},
					},
				},
			},
			operation: "Load",
			key:       "node1",
			expectedResult: &metav1.Condition{
				Type:   string(corev1.NodeReady),
				Status: metav1.ConditionTrue,
			},
			expectedError: false,
		},
		{
			name:          "Load non-existent node",
			setupNodes:    []*corev1.Node{},
			operation:     "Load",
			key:           "non-existent",
			expectedError: true,
		},
		{
			name: "Store condition for existing node",
			setupNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node2"},
					Status:     corev1.NodeStatus{},
				},
			},
			operation: "Store",
			key:       "node2",
			inputCondition: &metav1.Condition{
				Type:   string(corev1.NodeReady),
				Status: metav1.ConditionTrue,
			},
			expectedError: false,
		},
		{
			name: "Load existing node with mismatched condition type",
			setupNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node3"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
						},
					},
				},
			},
			operation:      "Load",
			key:            "node3",
			expectedResult: (*metav1.Condition)(nil),
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear the store
			err := nodeInformer.GetStore().Replace(nil, "")
			assert.NoError(t, err)

			// Setup nodes
			for _, node := range tc.setupNodes {
				err := nodeInformer.GetStore().Add(node)
				assert.NoError(t, err)
				_, err = clientset.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
				assert.NoError(t, err)
			}

			var result interface{}
			var opErr error

			switch tc.operation {
			case "Load":
				result, opErr = store.Load(tc.key)
			case "Store":
				opErr = store.Store(tc.key, tc.inputCondition)
			}

			if tc.expectedError {
				assert.Error(t, opErr)
			} else {
				assert.NoError(t, opErr)
				if tc.expectedResult != nil {
					assert.Equal(t, tc.expectedResult, result)
				}
			}
		})
	}
}

func TestNodeStore_ListAll(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	nodeInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{},
		&corev1.Node{},
		0,
		cache.Indexers{},
	)
	nodeLister := listcorev1.NewNodeLister(nodeInformer.GetIndexer())

	store := NewNodeConditionStore(
		clientset.CoreV1().Nodes(),
		nodeLister,
		string(corev1.NodeReady),
		5*time.Minute,
	)

	testCases := []struct {
		name           string
		nodes          []*corev1.Node
		expectedResult []metav1.Condition
		expectedError  bool
	}{
		{
			name: "List all ready nodes",
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{Type: corev1.NodeReady, Status: corev1.ConditionTrue, LastHeartbeatTime: metav1.Now()},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node2"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{Type: corev1.NodeReady, Status: corev1.ConditionTrue, LastHeartbeatTime: metav1.Now()},
						},
					},
				},
			},
			expectedResult: []metav1.Condition{
				{Type: string(corev1.NodeReady), Status: metav1.ConditionTrue},
				{Type: string(corev1.NodeReady), Status: metav1.ConditionTrue},
			},
			expectedError: false,
		},
		{
			name: "List with not ready and stale nodes",
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{Type: corev1.NodeReady, Status: corev1.ConditionTrue, LastHeartbeatTime: metav1.Now()},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node2"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{Type: corev1.NodeReady, Status: corev1.ConditionFalse, LastHeartbeatTime: metav1.Now()},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node3"},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{Type: corev1.NodeReady, Status: corev1.ConditionTrue, LastHeartbeatTime: metav1.NewTime(time.Now().Add(-10 * time.Minute))},
						},
					},
				},
			},
			expectedResult: []metav1.Condition{
				{Type: string(corev1.NodeReady), Status: metav1.ConditionTrue},
				{Type: string(corev1.NodeReady), Status: metav1.ConditionUnknown},
			},
			expectedError: false,
		},
		{
			name:           "Empty node list",
			nodes:          []*corev1.Node{},
			expectedResult: []metav1.Condition{},
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nodeInterfaces := make([]interface{}, len(tc.nodes))
			for i, node := range tc.nodes {
				nodeInterfaces[i] = node
			}

			err := nodeInformer.GetStore().Replace(nodeInterfaces, "")
			assert.NoError(t, err)

			result, err := store.ListAll()

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tc.expectedResult), len(result))

				expectedMap := make(map[string]int)
				resultMap := make(map[string]int)

				for _, cond := range tc.expectedResult {
					key := fmt.Sprintf("%s:%s", cond.Type, cond.Status)
					expectedMap[key]++
				}

				for _, cond := range result {
					key := fmt.Sprintf("%s:%s", cond.Type, cond.Status)
					resultMap[key]++
				}

				assert.Equal(t, expectedMap, resultMap)
			}
		})
	}
}

func TestSetNodeStatusCondition(t *testing.T) {
	testCases := []struct {
		name              string
		existingCondition *corev1.NodeCondition
		newCondition      corev1.NodeCondition
		expectedCondition corev1.NodeCondition
	}{
		{
			name: "Update existing condition with status change",
			existingCondition: &corev1.NodeCondition{
				Type:               corev1.NodeReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
				LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-1 * time.Hour)),
			},
			newCondition: corev1.NodeCondition{
				Type:    corev1.NodeReady,
				Status:  corev1.ConditionFalse,
				Reason:  "TestReason",
				Message: "Test message",
			},
			expectedCondition: corev1.NodeCondition{
				Type:    corev1.NodeReady,
				Status:  corev1.ConditionFalse,
				Reason:  "TestReason",
				Message: "Test message",
			},
		},
		{
			name: "Update existing condition without status change",
			existingCondition: &corev1.NodeCondition{
				Type:               corev1.NodeReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
				LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-1 * time.Hour)),
			},
			newCondition: corev1.NodeCondition{
				Type:    corev1.NodeReady,
				Status:  corev1.ConditionTrue,
				Reason:  "UpdatedReason",
				Message: "Updated message",
			},
			expectedCondition: corev1.NodeCondition{
				Type:    corev1.NodeReady,
				Status:  corev1.ConditionTrue,
				Reason:  "UpdatedReason",
				Message: "Updated message",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conditions := []corev1.NodeCondition{*tc.existingCondition}
			SetNodeStatusCondition(&conditions, tc.newCondition)

			assert.Equal(t, 1, len(conditions))
			updatedCondition := conditions[0]

			assert.Equal(t, tc.expectedCondition.Type, updatedCondition.Type)
			assert.Equal(t, tc.expectedCondition.Status, updatedCondition.Status)
			assert.Equal(t, tc.expectedCondition.Reason, updatedCondition.Reason)
			assert.Equal(t, tc.expectedCondition.Message, updatedCondition.Message)

			if tc.existingCondition.Status != tc.newCondition.Status {
				assert.True(t, updatedCondition.LastTransitionTime.After(tc.existingCondition.LastTransitionTime.Time))
			} else {
				assert.Equal(t, tc.existingCondition.LastTransitionTime, updatedCondition.LastTransitionTime)
			}

			assert.True(t, updatedCondition.LastHeartbeatTime.After(tc.existingCondition.LastHeartbeatTime.Time))
		})
	}
}
