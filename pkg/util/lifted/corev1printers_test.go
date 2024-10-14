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
