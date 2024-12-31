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

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	workloadv1alpha1 "github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/webhook/interpreter"
)

func TestWorkloadInterpreter_responseWithExploreReplica(t *testing.T) {
	testCases := []struct {
		name     string
		workload *workloadv1alpha1.Workload
		expected int32
	}{
		{
			name: "Workload with replicas",
			workload: &workloadv1alpha1.Workload{
				Spec: workloadv1alpha1.WorkloadSpec{
					Replicas: ptr.To[int32](3),
				},
			},
			expected: 3,
		},
		{
			name: "Workload without replicas",
			workload: &workloadv1alpha1.Workload{
				Spec: workloadv1alpha1.WorkloadSpec{},
			},
			expected: 0,
		},
	}

	interpreter := &workloadInterpreter{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response := interpreter.responseWithExploreReplica(tc.workload)
			if response.Replicas != nil {
				assert.Equal(t, tc.expected, *response.Replicas)
			} else {
				assert.Equal(t, tc.expected, int32(0))
			}
		})
	}
}

func TestWorkloadInterpreter_responseWithExploreDependency(t *testing.T) {
	workload := &workloadv1alpha1.Workload{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
		},
		Spec: workloadv1alpha1.WorkloadSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "test-configmap",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	interpreter := &workloadInterpreter{}
	response := interpreter.responseWithExploreDependency(workload)

	expectedDependencies := []configv1alpha1.DependentObjectReference{
		{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Namespace:  "test-namespace",
			Name:       "test-configmap",
		},
	}

	assert.Equal(t, expectedDependencies, response.Dependencies)
}

func TestWorkloadInterpreter_responseWithExploreInterpretHealth(t *testing.T) {
	testCases := []struct {
		name     string
		workload *workloadv1alpha1.Workload
		expected bool
	}{
		{
			name: "Healthy workload",
			workload: &workloadv1alpha1.Workload{
				Spec: workloadv1alpha1.WorkloadSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: workloadv1alpha1.WorkloadStatus{
					ReadyReplicas: 3,
				},
			},
			expected: true,
		},
		{
			name: "Unhealthy workload",
			workload: &workloadv1alpha1.Workload{
				Spec: workloadv1alpha1.WorkloadSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: workloadv1alpha1.WorkloadStatus{
					ReadyReplicas: 2,
				},
			},
			expected: false,
		},
	}

	interpreter := &workloadInterpreter{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response := interpreter.responseWithExploreInterpretHealth(tc.workload)
			assert.Equal(t, tc.expected, *response.Healthy)
		})
	}
}

func TestWorkloadInterpreter_responseWithExploreInterpretStatus(t *testing.T) {
	workload := &workloadv1alpha1.Workload{
		Status: workloadv1alpha1.WorkloadStatus{
			ReadyReplicas: 3,
		},
	}

	interpreter := &workloadInterpreter{}
	response := interpreter.responseWithExploreInterpretStatus(workload)

	expectedStatus := &workloadv1alpha1.WorkloadStatus{
		ReadyReplicas: 3,
	}
	expectedBytes, _ := json.Marshal(expectedStatus)

	assert.Equal(t, expectedBytes, response.RawStatus.Raw)
}

func TestWorkloadInterpreter_Handle(t *testing.T) {
	testCases := []struct {
		name           string
		operation      configv1alpha1.InterpreterOperation
		expectedStatus int32
		checkResponse  func(*testing.T, interpreter.Response)
	}{
		{
			name:           "InterpretReplica operation",
			operation:      configv1alpha1.InterpreterOperationInterpretReplica,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "InterpretDependency operation",
			operation:      configv1alpha1.InterpreterOperationInterpretDependency,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "InterpretHealth operation",
			operation:      configv1alpha1.InterpreterOperationInterpretHealth,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "InterpretStatus operation",
			operation:      configv1alpha1.InterpreterOperationInterpretStatus,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid operation",
			operation:      "InvalidOperation",
			expectedStatus: http.StatusBadRequest,
		},
	}

	scheme := runtime.NewScheme()
	_ = workloadv1alpha1.Install(scheme)
	decoder := interpreter.NewDecoder(scheme)

	interpreterInstance := &workloadInterpreter{
		decoder: decoder,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workload := &workloadv1alpha1.Workload{
				Spec: workloadv1alpha1.WorkloadSpec{
					Replicas: ptr.To[int32](3),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									EnvFrom: []corev1.EnvFromSource{
										{
											ConfigMapRef: &corev1.ConfigMapEnvSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "test-configmap",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Status: workloadv1alpha1.WorkloadStatus{
					ReadyReplicas: 3,
				},
			}
			workloadBytes, _ := json.Marshal(workload)

			req := interpreter.Request{
				ResourceInterpreterRequest: configv1alpha1.ResourceInterpreterRequest{
					Operation: tc.operation,
					Object: runtime.RawExtension{
						Raw: workloadBytes,
					},
				},
			}

			response := interpreterInstance.Handle(context.Background(), req)
			assert.Equal(t, tc.expectedStatus, response.Status.Code)
		})
	}
}
