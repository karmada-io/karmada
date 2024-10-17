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

package lifted

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"

	"github.com/karmada-io/karmada/pkg/printers"
)

// MockPrintHandler is a mock implementation of printers.PrintHandler
type MockPrintHandler struct {
	mock.Mock
}

func (m *MockPrintHandler) TableHandler(columnDefinitions []metav1.TableColumnDefinition, printFunc interface{}) error {
	args := m.Called(columnDefinitions, printFunc)
	return args.Error(0)
}

func TestAddCoreV1Handlers(t *testing.T) {
	testCases := []struct {
		name           string
		expectedCalls  int
		columnChecks   map[string][]string
		printFuncTypes map[string]reflect.Type
	}{
		{
			name:          "Verify handlers are added correctly",
			expectedCalls: 4,
			columnChecks: map[string][]string{
				"Pod":  {"Name", "Ready", "Status", "Restarts", "Age", "IP", "Node", "Nominated Node", "Readiness Gates"},
				"Node": {"Name", "Status", "Roles", "Age", "Version", "Internal-IP", "External-IP", "OS-Image", "Kernel-Version", "Container-Runtime"},
			},
			printFuncTypes: map[string]reflect.Type{
				"PodList":  reflect.TypeOf(func(*corev1.PodList, printers.GenerateOptions) ([]metav1.TableRow, error) { return nil, nil }),
				"Pod":      reflect.TypeOf(func(*corev1.Pod, printers.GenerateOptions) ([]metav1.TableRow, error) { return nil, nil }),
				"Node":     reflect.TypeOf(func(*corev1.Node, printers.GenerateOptions) ([]metav1.TableRow, error) { return nil, nil }),
				"NodeList": reflect.TypeOf(func(*corev1.NodeList, printers.GenerateOptions) ([]metav1.TableRow, error) { return nil, nil }),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockHandler := &MockPrintHandler{}
			mockHandler.On("TableHandler", mock.Anything, mock.Anything).Return(nil)

			AddCoreV1Handlers(mockHandler)

			assert.Equal(t, tc.expectedCalls, len(mockHandler.Calls))

			for i, call := range mockHandler.Calls {
				columnDefinitions := call.Arguments[0].([]metav1.TableColumnDefinition)
				printFunc := call.Arguments[1]

				resourceType := ""
				switch i {
				case 0:
					resourceType = "PodList"
				case 1:
					resourceType = "Pod"
				case 2:
					resourceType = "Node"
				case 3:
					resourceType = "NodeList"
				}

				// Check column definitions
				if expectedColumns, ok := tc.columnChecks[strings.TrimSuffix(resourceType, "List")]; ok {
					assert.Equal(t, len(expectedColumns), len(columnDefinitions))
					for j, name := range expectedColumns {
						assert.Equal(t, name, columnDefinitions[j].Name)
						assert.NotEmpty(t, columnDefinitions[j].Type)
						assert.NotEmpty(t, columnDefinitions[j].Description)
					}
				}

				// Check print function type
				if expectedType, ok := tc.printFuncTypes[resourceType]; ok {
					assert.Equal(t, expectedType, reflect.TypeOf(printFunc))
				}
			}

			mockHandler.AssertExpectations(t)
		})
	}
}

func TestPrintNode(t *testing.T) {
	testCases := []struct {
		name           string
		node           *corev1.Node
		options        printers.GenerateOptions
		expectedChecks func(*testing.T, []metav1.TableRow)
	}{
		{
			name: "Basic ready node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "node1",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-24 * time.Hour)},
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.20.0",
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Len(t, rows[0].Cells, 5)
				assert.Equal(t, "node1", rows[0].Cells[0])
				assert.Contains(t, rows[0].Cells[1], "Ready")
				assert.NotEmpty(t, rows[0].Cells[3]) // Only check it's not empty due to time dependency
				assert.Equal(t, "v1.20.0", rows[0].Cells[4])
			},
		},
		{
			name: "Node with roles",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "node2",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-48 * time.Hour)},
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
						"node-role.kubernetes.io/worker":        "",
					},
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.21.0",
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Len(t, rows[0].Cells, 5)
				assert.Equal(t, "node2", rows[0].Cells[0])
				assert.Contains(t, rows[0].Cells[1], "Ready")
				// Use Contains for roles as the order might not be guaranteed
				assert.Contains(t, rows[0].Cells[2], "control-plane")
				assert.Contains(t, rows[0].Cells[2], "worker")
				assert.NotEmpty(t, rows[0].Cells[3])
				assert.Equal(t, "v1.21.0", rows[0].Cells[4])
			},
		},
		{
			name: "Unschedulable node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "node3",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-72 * time.Hour)},
				},
				Spec: corev1.NodeSpec{
					Unschedulable: true,
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.22.0",
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Len(t, rows[0].Cells, 5)
				assert.Equal(t, "node3", rows[0].Cells[0])
				assert.Contains(t, rows[0].Cells[1], "SchedulingDisabled")
				assert.NotEmpty(t, rows[0].Cells[3])
				assert.Equal(t, "v1.22.0", rows[0].Cells[4])
			},
		},
		{
			name: "Node with missing OS, kernel, and container runtime info",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "node6",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-144 * time.Hour)},
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.25.0",
						// OSImage, KernelVersion, and ContainerRuntimeVersion are intentionally left empty
					},
					Addresses: []corev1.NodeAddress{
						{Type: corev1.NodeInternalIP, Address: "192.168.1.2"},
						{Type: corev1.NodeExternalIP, Address: "203.0.113.2"},
					},
				},
			},
			options: printers.GenerateOptions{Wide: true},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Len(t, rows[0].Cells, 10) // Wide option should produce 10 columns
				assert.Equal(t, "node6", rows[0].Cells[0])
				assert.Contains(t, rows[0].Cells[1], "Ready")
				assert.NotEmpty(t, rows[0].Cells[3])
				assert.Equal(t, "v1.25.0", rows[0].Cells[4])
				assert.Equal(t, "192.168.1.2", rows[0].Cells[5]) // Internal IP
				assert.Equal(t, "203.0.113.2", rows[0].Cells[6]) // External IP
				assert.Equal(t, "<unknown>", rows[0].Cells[7])   // OSImage
				assert.Equal(t, "<unknown>", rows[0].Cells[8])   // KernelVersion
				assert.Equal(t, "<unknown>", rows[0].Cells[9])   // ContainerRuntimeVersion
			},
		},
		{
			name: "Node with wide option",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "node4",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-96 * time.Hour)},
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion:          "v1.23.0",
						OSImage:                 "Ubuntu 20.04",
						KernelVersion:           "5.4.0-42-generic",
						ContainerRuntimeVersion: "docker://19.03.8",
					},
					Addresses: []corev1.NodeAddress{
						{Type: corev1.NodeInternalIP, Address: "192.168.1.1"},
						{Type: corev1.NodeExternalIP, Address: "203.0.113.1"},
					},
				},
			},
			options: printers.GenerateOptions{Wide: true},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Len(t, rows[0].Cells, 10) // Wide option should produce 10 columns
				assert.Equal(t, "node4", rows[0].Cells[0])
				assert.Contains(t, rows[0].Cells[1], "Ready")
				assert.NotEmpty(t, rows[0].Cells[3])
				assert.Equal(t, "v1.23.0", rows[0].Cells[4])
				assert.Equal(t, "192.168.1.1", rows[0].Cells[5]) // Internal IP
				assert.Equal(t, "203.0.113.1", rows[0].Cells[6]) // External IP
				assert.Equal(t, "Ubuntu 20.04", rows[0].Cells[7])
				assert.Equal(t, "5.4.0-42-generic", rows[0].Cells[8])
				assert.Equal(t, "docker://19.03.8", rows[0].Cells[9])
			},
		},
		{
			name: "Node with no conditions",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "node5",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-120 * time.Hour)},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.24.0",
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Len(t, rows[0].Cells, 5)
				assert.Equal(t, "node5", rows[0].Cells[0])
				assert.Equal(t, "Unknown", rows[0].Cells[1])
				assert.Equal(t, "<none>", rows[0].Cells[2])
				assert.NotEmpty(t, rows[0].Cells[3])
				assert.Equal(t, "v1.24.0", rows[0].Cells[4])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := printNode(tc.node, tc.options)
			assert.NoError(t, err)

			// Run the custom checks for this test case
			tc.expectedChecks(t, rows)

			// Ensure the original node object is included in the output
			assert.Equal(t, runtime.RawExtension{Object: tc.node}, rows[0].Object)
		})
	}
}

func TestGetNodeExternalIP(t *testing.T) {
	tests := []struct {
		name     string
		node     *corev1.Node
		expected string
	}{
		{
			name: "Node with external IP",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{Type: corev1.NodeExternalIP, Address: "203.0.113.1"},
						{Type: corev1.NodeInternalIP, Address: "192.168.1.1"},
					},
				},
			},
			expected: "203.0.113.1",
		},
		{
			name: "Node without external IP",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{Type: corev1.NodeInternalIP, Address: "192.168.1.1"},
					},
				},
			},
			expected: "<none>",
		},
		{
			name:     "Node with no addresses",
			node:     &corev1.Node{},
			expected: "<none>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNodeExternalIP(tt.node)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetNodeInternalIP(t *testing.T) {
	tests := []struct {
		name     string
		node     *corev1.Node
		expected string
	}{
		{
			name: "Node with internal IP",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{Type: corev1.NodeExternalIP, Address: "203.0.113.1"},
						{Type: corev1.NodeInternalIP, Address: "192.168.1.1"},
					},
				},
			},
			expected: "192.168.1.1",
		},
		{
			name: "Node without internal IP",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{Type: corev1.NodeExternalIP, Address: "203.0.113.1"},
					},
				},
			},
			expected: "<none>",
		},
		{
			name:     "Node with no addresses",
			node:     &corev1.Node{},
			expected: "<none>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNodeInternalIP(tt.node)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindNodeRoles(t *testing.T) {
	tests := []struct {
		name     string
		node     *corev1.Node
		expected []string
	}{
		{
			name: "Node with single role",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
			},
			expected: []string{"control-plane"},
		},
		{
			name: "Node with multiple roles",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
						"node-role.kubernetes.io/worker":        "",
					},
				},
			},
			expected: []string{"control-plane", "worker"},
		},
		{
			name: "Node with kubernetes.io/role label",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"kubernetes.io/role": "special",
					},
				},
			},
			expected: []string{"special"},
		},
		{
			name: "Node with no role labels",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			expected: []string{},
		},
		{
			name: "Node with special characters in role names",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/role-with-hyphen":     "",
						"node-role.kubernetes.io/role_with_underscore": "",
					},
				},
			},
			expected: []string{"role-with-hyphen", "role_with_underscore"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findNodeRoles(tt.node)
			assert.ElementsMatch(t, tt.expected, result) // Order of roles can not be determined
		})
	}
}

func TestPrintNodeList(t *testing.T) {
	testCases := []struct {
		name     string
		nodeList *corev1.NodeList
		options  printers.GenerateOptions
		expected func(*testing.T, []metav1.TableRow)
	}{
		{
			name:     "Empty node list",
			nodeList: &corev1.NodeList{},
			options:  printers.GenerateOptions{},
			expected: func(t *testing.T, rows []metav1.TableRow) {
				assert.Empty(t, rows)
			},
		},
		{
			name: "Single node",
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "node1",
							CreationTimestamp: metav1.Time{Time: time.Now().Add(-24 * time.Hour)},
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
							},
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.20.0",
							},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expected: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "node1", rows[0].Cells[0])
				assert.Contains(t, rows[0].Cells[1], "Ready")
				assert.NotEmpty(t, rows[0].Cells[3])
				assert.Equal(t, "v1.20.0", rows[0].Cells[4])
			},
		},
		{
			name: "Multiple nodes with different states",
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "node1",
							CreationTimestamp: metav1.Time{Time: time.Now().Add(-24 * time.Hour)},
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
							},
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.20.0",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "node2",
							CreationTimestamp: metav1.Time{Time: time.Now().Add(-48 * time.Hour)},
						},
						Spec: corev1.NodeSpec{
							Unschedulable: true,
						},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{
								{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
							},
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.21.0",
							},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expected: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 2)
				assert.Equal(t, "node1", rows[0].Cells[0])
				assert.Contains(t, rows[0].Cells[1], "Ready")
				assert.Equal(t, "v1.20.0", rows[0].Cells[4])
				assert.Equal(t, "node2", rows[1].Cells[0])
				assert.Contains(t, rows[1].Cells[1], "NotReady")
				assert.Contains(t, rows[1].Cells[1], "SchedulingDisabled")
				assert.Equal(t, "v1.21.0", rows[1].Cells[4])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := printNodeList(tc.nodeList, tc.options)
			assert.NoError(t, err)
			tc.expected(t, rows)
		})
	}
}

func TestPrintPodList(t *testing.T) {
	testCases := []struct {
		name     string
		podList  *corev1.PodList
		options  printers.GenerateOptions
		expected func(*testing.T, []metav1.TableRow)
	}{
		{
			name:    "Empty pod list",
			podList: &corev1.PodList{},
			options: printers.GenerateOptions{},
			expected: func(t *testing.T, rows []metav1.TableRow) {
				assert.Empty(t, rows)
			},
		},
		{
			name: "Single running pod",
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "pod1",
							Namespace:         "default",
							CreationTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "container1"},
							},
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
							ContainerStatuses: []corev1.ContainerStatus{
								{
									Ready: true,
									State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
								},
							},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expected: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "pod1", rows[0].Cells[0])
				assert.Equal(t, "1/1", rows[0].Cells[1])
				assert.Equal(t, "Running", rows[0].Cells[2])
				assert.NotEmpty(t, rows[0].Cells[4])
			},
		},
		{
			name: "Multiple pods with different states",
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "pod1",
							Namespace:         "default",
							CreationTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "container1"},
							},
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
							ContainerStatuses: []corev1.ContainerStatus{
								{
									Ready: true,
									State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "pod2",
							Namespace:         "kube-system",
							CreationTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "container2"},
							},
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodPending,
							ContainerStatuses: []corev1.ContainerStatus{
								{
									Ready: false,
									State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
								},
							},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expected: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 2)
				assert.Equal(t, "pod1", rows[0].Cells[0])
				assert.Equal(t, "1/1", rows[0].Cells[1])
				assert.Equal(t, "Running", rows[0].Cells[2])
				assert.NotEmpty(t, rows[0].Cells[4])
				assert.Equal(t, "pod2", rows[1].Cells[0])
				assert.Equal(t, "0/1", rows[1].Cells[1])
				assert.Contains(t, []string{"Pending", "ContainerCreating"}, rows[1].Cells[2])
				assert.NotEmpty(t, rows[1].Cells[4])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := printPodList(tc.podList, tc.options)
			assert.NoError(t, err)
			tc.expected(t, rows)
		})
	}
}

func TestPrintPod(t *testing.T) {
	testCases := []struct {
		name           string
		pod            *corev1.Pod
		options        printers.GenerateOptions
		expectedChecks func(*testing.T, []metav1.TableRow)
	}{
		{
			name: "Running pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "running-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Len(t, rows[0].Cells, 5)
				assert.Equal(t, "running-pod", rows[0].Cells[0])
				assert.Equal(t, "1/1", rows[0].Cells[1])
				assert.Equal(t, "Running", rows[0].Cells[2])
				assert.Equal(t, int64(0), rows[0].Cells[3])
				assert.Regexp(t, `\d+[hm]`, rows[0].Cells[4])
			},
		},
		{
			name: "Pending pod with ContainerCreating",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "pending-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: false, State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Len(t, rows[0].Cells, 5)
				assert.Equal(t, "pending-pod", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "ContainerCreating", rows[0].Cells[2])
				assert.Equal(t, int64(0), rows[0].Cells[3])
				assert.Regexp(t, `\d+m`, rows[0].Cells[4])
			},
		},
		{
			name: "Succeeded pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "succeeded-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-3 * time.Hour)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodSucceeded,
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "succeeded-pod", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "Succeeded", rows[0].Cells[2])
				assert.Equal(t, int64(0), rows[0].Cells[3])
				assert.Regexp(t, `\d+h`, rows[0].Cells[4])
				assert.Equal(t, podSuccessConditions, rows[0].Conditions)
			},
		},
		{
			name: "Failed pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "failed-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: false, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error"}}},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "failed-pod", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "Error", rows[0].Cells[2])
				assert.Equal(t, int64(0), rows[0].Cells[3])
				assert.Regexp(t, `(\d+h|\d+m)`, rows[0].Cells[4]) // Match either hours or minutes
				assert.Equal(t, podFailedConditions, rows[0].Conditions)
			},
		},
		{
			name: "Pod with multiple containers and restarts",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "multi-container-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-3 * time.Hour)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}, {Name: "container2"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, RestartCount: 2, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
						{Ready: true, RestartCount: 1, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Len(t, rows[0].Cells, 5)
				assert.Equal(t, "multi-container-pod", rows[0].Cells[0])
				assert.Equal(t, "2/2", rows[0].Cells[1])
				assert.Equal(t, "Running", rows[0].Cells[2])
				assert.Equal(t, int64(3), rows[0].Cells[3])
				assert.Regexp(t, `\d+h`, rows[0].Cells[4])
			},
		},
		{
			name: "Pod with readiness gates",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "readiness-gate-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-4 * time.Hour)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
					ReadinessGates: []corev1.PodReadinessGate{
						{ConditionType: "custom-condition"},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
					},
					Conditions: []corev1.PodCondition{
						{Type: "custom-condition", Status: corev1.ConditionTrue},
					},
				},
			},
			options: printers.GenerateOptions{Wide: true},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Len(t, rows[0].Cells, 9)
				assert.Equal(t, "readiness-gate-pod", rows[0].Cells[0])
				assert.Equal(t, "1/1", rows[0].Cells[1])
				assert.Equal(t, "Running", rows[0].Cells[2])
				assert.Equal(t, int64(0), rows[0].Cells[3])
				assert.Regexp(t, `\d+h`, rows[0].Cells[4])
				assert.Equal(t, "<none>", rows[0].Cells[5]) // IP
				assert.Equal(t, "<none>", rows[0].Cells[6]) // Node
				assert.Equal(t, "<none>", rows[0].Cells[7]) // Nominated Node
				assert.Equal(t, "1/1", rows[0].Cells[8])    // Readiness Gates
			},
		},
		{
			name: "Pod with init container - waiting",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "init-pod-waiting",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{Name: "init-container"}},
					Containers:     []corev1.Container{{Name: "main-container"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							Name:  "init-container",
							Ready: false,
							State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Len(t, rows[0].Cells, 5)
				assert.Equal(t, "init-pod-waiting", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "Init:ContainerCreating", rows[0].Cells[2])
				assert.Equal(t, int64(0), rows[0].Cells[3])
				assert.Regexp(t, `\d+[hm]`, rows[0].Cells[4])
			},
		},
		{
			name: "Terminating pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "terminating-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-6 * time.Hour)},
					DeletionTimestamp: &metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Len(t, rows[0].Cells, 5)
				assert.Equal(t, "terminating-pod", rows[0].Cells[0])
				assert.Equal(t, "1/1", rows[0].Cells[1])
				assert.Equal(t, "Terminating", rows[0].Cells[2])
				assert.Equal(t, int64(0), rows[0].Cells[3])
				assert.Regexp(t, `\d+h`, rows[0].Cells[4])
			},
		},
		{
			name: "Node unreachable pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "unreachable-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-7 * time.Hour)},
					DeletionTimestamp: &metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				Status: corev1.PodStatus{
					Phase:  corev1.PodRunning,
					Reason: NodeUnreachablePodReason,
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "unreachable-pod", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "Unknown", rows[0].Cells[2])
				assert.Equal(t, int64(0), rows[0].Cells[3])
				assert.Regexp(t, `\d+h`, rows[0].Cells[4])
			},
		},
		{
			name: "Pod with init container terminated with signal",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "init-signal-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{Name: "init-container"}},
					Containers:     []corev1.Container{{Name: "main-container"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "init-container",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Signal: 9,
								},
							},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "init-signal-pod", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "Pending", rows[0].Cells[2])
			},
		},
		{
			name: "Pod with init container terminated with exit code",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "init-exit-code-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-35 * time.Minute)},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{Name: "init-container"}},
					Containers:     []corev1.Container{{Name: "main-container"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "init-container",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode: 1,
								},
							},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "init-exit-code-pod", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "Init:ExitCode:1", rows[0].Cells[2])
			},
		},
		{
			name: "Pod with init container terminated with reason",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "init-reason-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-40 * time.Minute)},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{Name: "init-container"}},
					Containers:     []corev1.Container{{Name: "main-container"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "init-container",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "Error",
								},
							},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "init-reason-pod", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "Pending", rows[0].Cells[2])
			},
		},
		{
			name: "Pod with multiple init containers, some pending",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "multi-init-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-45 * time.Minute)},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{Name: "init-container-1"},
						{Name: "init-container-2"},
						{Name: "init-container-3"},
					},
					Containers: []corev1.Container{{Name: "main-container"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							Name:  "init-container-1",
							Ready: true,
							State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}},
						},
						{
							Name:  "init-container-2",
							Ready: false,
							State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}},
						},
						{
							Name:  "init-container-3",
							Ready: false,
							State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "multi-init-pod", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "Init:1/3", rows[0].Cells[2])
			},
		},
		{
			name: "Pod with container terminated with signal",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "terminated-signal-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "container1",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Signal: 15,
								},
							},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "terminated-signal-pod", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "Signal:15", rows[0].Cells[2])
			},
		},
		{
			name: "Pod with container terminated with exit code",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "terminated-exit-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "container1",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode: 2,
								},
							},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "terminated-exit-pod", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "ExitCode:2", rows[0].Cells[2])
			},
		},
		{
			name: "Running pod with ready condition",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "ready-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-3 * time.Hour)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{Type: corev1.PodReady, Status: corev1.ConditionTrue},
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:  "container1",
							Ready: true,
							State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "ready-pod", rows[0].Cells[0])
				assert.Equal(t, "1/1", rows[0].Cells[1])
				assert.Equal(t, "Running", rows[0].Cells[2])
			},
		},
		{
			name: "Running pod without ready condition",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "not-ready-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-4 * time.Hour)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:  "container1",
							Ready: false,
							State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "not-ready-pod", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "Running", rows[0].Cells[2])
			},
		},
		{
			name: "Running pod with container not ready",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "running-not-ready-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-55 * time.Minute)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main-container"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:  "main-container",
							Ready: false,
							State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "running-not-ready-pod", rows[0].Cells[0])
				assert.Equal(t, "0/1", rows[0].Cells[1])
				assert.Equal(t, "Running", rows[0].Cells[2])
			},
		},
		{
			name: "Completed pod with running container",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "completed-running-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-50 * time.Minute)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main-container"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{Type: corev1.PodReady, Status: corev1.ConditionTrue},
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:  "main-container",
							Ready: true,
							State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
						},
					},
				},
			},
			options: printers.GenerateOptions{},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "completed-running-pod", rows[0].Cells[0])
				assert.Equal(t, "1/1", rows[0].Cells[1])
				assert.Equal(t, "Running", rows[0].Cells[2])
			},
		},
		{
			name: "Pod with multiple IPs",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "multi-ip-pod",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-5 * time.Hour)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					PodIPs: []corev1.PodIP{
						{IP: "192.168.1.10"},
						{IP: "fd00::10"},
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:  "container1",
							Ready: true,
							State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
						},
					},
				},
			},
			options: printers.GenerateOptions{Wide: true},
			expectedChecks: func(t *testing.T, rows []metav1.TableRow) {
				assert.Len(t, rows, 1)
				assert.Equal(t, "multi-ip-pod", rows[0].Cells[0])
				assert.Equal(t, "1/1", rows[0].Cells[1])
				assert.Equal(t, "Running", rows[0].Cells[2])
				assert.Equal(t, "192.168.1.10", rows[0].Cells[5]) // IP column in wide output
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := printPod(tc.pod, tc.options)
			assert.NoError(t, err)
			tc.expectedChecks(t, rows)
			assert.Equal(t, runtime.RawExtension{Object: tc.pod}, rows[0].Object)
		})
	}
}

func TestHasPodReadyCondition(t *testing.T) {
	testCases := []struct {
		name       string
		conditions []corev1.PodCondition
		expected   bool
	}{
		{
			name:       "Empty conditions",
			conditions: []corev1.PodCondition{},
			expected:   false,
		},
		{
			name: "Ready condition is true",
			conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			expected: true,
		},
		{
			name: "Ready condition is false",
			conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				},
			},
			expected: false,
		},
		{
			name: "Ready condition is unknown",
			conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionUnknown,
				},
			},
			expected: false,
		},
		{
			name: "Multiple conditions, Ready is true",
			conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodInitialized,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   corev1.ContainersReady,
					Status: corev1.ConditionTrue,
				},
			},
			expected: true,
		},
		{
			name: "Multiple conditions, Ready is false",
			conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodInitialized,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				},
				{
					Type:   corev1.ContainersReady,
					Status: corev1.ConditionTrue,
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasPodReadyCondition(tc.conditions)
			assert.Equal(t, tc.expected, result, "hasPodReadyCondition returned unexpected result")
		})
	}
}

func TestPrintCoreV1(t *testing.T) {
	testCases := []struct {
		pod             corev1.Pod
		generateOptions printers.GenerateOptions
		expect          []metav1.TableRow
	}{
		// Test name, kubernetes version, sync mode, cluster ready status,
		{
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test1"},
				Spec:       corev1.PodSpec{},
				Status: corev1.PodStatus{
					Phase:  corev1.PodPending,
					HostIP: "1.2.3.4",
					PodIP:  "2.3.4.5",
				},
			},
			printers.GenerateOptions{Wide: false},
			[]metav1.TableRow{{Cells: []interface{}{"test1", "0/0", "Pending", int64(0), "<unknown>"}}},
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("pod=%s, expected value=%v", tc.pod.Name, tc.expect), func(t *testing.T) {
			rows, err := printPod(&tc.pod, tc.generateOptions)
			if err != nil {
				t.Fatal(err)
			}
			for i := range rows {
				rows[i].Object.Object = nil
			}
			if !reflect.DeepEqual(tc.expect, rows) {
				t.Errorf("%d mismatch: %s", i, diff.ObjectReflectDiff(tc.expect, rows))
			}
		})
	}
}
