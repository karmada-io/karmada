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

package native

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func Test_reflectPodDisruptionBudgetStatus(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			name: "PDB with valid status",
			object: func() *unstructured.Unstructured {
				pdb := &policyv1.PodDisruptionBudget{
					TypeMeta: metav1.TypeMeta{
						Kind:       "PodDisruptionBudget",
						APIVersion: policyv1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pdb",
						Namespace: "test-ns",
					},
					Status: policyv1.PodDisruptionBudgetStatus{
						CurrentHealthy:     2,
						DesiredHealthy:     3,
						DisruptionsAllowed: 1,
						ExpectedPods:       3,
						DisruptedPods: map[string]metav1.Time{
							"pod1": metav1.Now(),
						},
					},
				}
				obj, _ := helper.ToUnstructured(pdb)
				return obj
			}(),
			want: func() *runtime.RawExtension {
				status := &policyv1.PodDisruptionBudgetStatus{
					CurrentHealthy:     2,
					DesiredHealthy:     3,
					DisruptionsAllowed: 1,
					ExpectedPods:       3,
					DisruptedPods: map[string]metav1.Time{
						"pod1": metav1.Now(),
					},
				}
				raw, _ := helper.BuildStatusRawExtension(status)
				return raw
			}(),
			wantErr: false,
		},
		{
			name: "PDB without status",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "policy/v1",
					"kind":       "PodDisruptionBudget",
					"metadata": map[string]interface{}{
						"name":      "test-pdb",
						"namespace": "test-ns",
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "PDB with invalid status format",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "policy/v1",
					"kind":       "PodDisruptionBudget",
					"metadata": map[string]interface{}{
						"name":      "test-pdb",
						"namespace": "test-ns",
					},
					"status": "invalid",
				},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reflectPodDisruptionBudgetStatus(tt.object)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.JSONEq(t, string(tt.want.Raw), string(got.Raw))
			}
		})
	}
}

func Test_reflectHorizontalPodAutoscalerStatus(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			name: "HPA with valid status",
			object: func() *unstructured.Unstructured {
				hpa := &autoscalingv2.HorizontalPodAutoscaler{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HorizontalPodAutoscaler",
						APIVersion: autoscalingv2.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-hpa",
						Namespace: "test-ns",
					},
					Status: autoscalingv2.HorizontalPodAutoscalerStatus{
						CurrentReplicas: 2,
						DesiredReplicas: 3,
					},
				}
				obj, _ := helper.ToUnstructured(hpa)
				return obj
			}(),
			want: func() *runtime.RawExtension {
				status := &autoscalingv2.HorizontalPodAutoscalerStatus{
					CurrentReplicas: 2,
					DesiredReplicas: 3,
				}
				raw, _ := helper.BuildStatusRawExtension(status)
				return raw
			}(),
			wantErr: false,
		},
		{
			name: "HPA without status",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "autoscaling/v2",
					"kind":       "HorizontalPodAutoscaler",
					"metadata": map[string]interface{}{
						"name":      "test-hpa",
						"namespace": "test-ns",
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "HPA with invalid status format",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "autoscaling/v2",
					"kind":       "HorizontalPodAutoscaler",
					"metadata": map[string]interface{}{
						"name":      "test-hpa",
						"namespace": "test-ns",
					},
					"status": "invalid",
				},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reflectHorizontalPodAutoscalerStatus(tt.object)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.JSONEq(t, string(tt.want.Raw), string(got.Raw))
			}
		})
	}
}

func Test_reflectWholeStatus(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			name: "object with valid status",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"key": "value",
						"num": int64(1),
					},
				},
			},
			want: func() *runtime.RawExtension {
				status := map[string]interface{}{
					"key": "value",
					"num": int64(1),
				}
				raw, _ := helper.BuildStatusRawExtension(status)
				return raw
			}(),
			wantErr: false,
		},
		{
			name: "object without status",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "object with invalid status format",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": "invalid",
				},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reflectWholeStatus(tt.object)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.JSONEq(t, string(tt.want.Raw), string(got.Raw))
			}
		})
	}
}

func Test_getAllDefaultReflectStatusInterpreter(t *testing.T) {
	tests := []struct {
		name   string
		gvk    schema.GroupVersionKind
		wantFn bool
	}{
		{
			name:   "Deployment interpreter exists",
			gvk:    appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind),
			wantFn: true,
		},
		{
			name:   "Service interpreter exists",
			gvk:    corev1.SchemeGroupVersion.WithKind(util.ServiceKind),
			wantFn: true,
		},
		{
			name:   "Ingress interpreter exists",
			gvk:    networkingv1.SchemeGroupVersion.WithKind(util.IngressKind),
			wantFn: true,
		},
		{
			name:   "Job interpreter exists",
			gvk:    batchv1.SchemeGroupVersion.WithKind(util.JobKind),
			wantFn: true,
		},
		{
			name:   "DaemonSet interpreter exists",
			gvk:    appsv1.SchemeGroupVersion.WithKind(util.DaemonSetKind),
			wantFn: true,
		},
		{
			name:   "StatefulSet interpreter exists",
			gvk:    appsv1.SchemeGroupVersion.WithKind(util.StatefulSetKind),
			wantFn: true,
		},
		{
			name:   "PodDisruptionBudget interpreter exists",
			gvk:    policyv1.SchemeGroupVersion.WithKind(util.PodDisruptionBudgetKind),
			wantFn: true,
		},
		{
			name:   "HorizontalPodAutoscaler interpreter exists",
			gvk:    autoscalingv2.SchemeGroupVersion.WithKind(util.HorizontalPodAutoscalerKind),
			wantFn: true,
		},
		{
			name:   "Non-existent resource should not have interpreter",
			gvk:    schema.GroupVersionKind{Group: "fake", Version: "v1", Kind: "Fake"},
			wantFn: false,
		},
	}

	interpreters := getAllDefaultReflectStatusInterpreter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interpreter, exists := interpreters[tt.gvk]
			assert.Equal(t, tt.wantFn, exists, "interpreter existence mismatch for %v", tt.gvk)
			if tt.wantFn {
				assert.NotNil(t, interpreter, "interpreter should not be nil for %v", tt.gvk)
				// Type checking
				assert.IsType(t, (reflectStatusInterpreter)(nil), interpreter,
					"interpreter should be of type reflectStatusInterpreter")
			}
		})
	}

	// Verify total number of interpreters
	assert.Len(t, interpreters, 8, "unexpected number of interpreters")

	// Verify map is not nil
	assert.NotNil(t, interpreters, "interpreters map should not be nil")
}

func Test_reflectDeploymentStatus(t *testing.T) {
	validDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-deployment",
			Namespace:  "test-ns",
			Generation: 2,
			Annotations: map[string]string{
				workv1alpha2.ResourceTemplateGenerationAnnotationKey: "1",
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:            3,
			UpdatedReplicas:     3,
			ReadyReplicas:       3,
			AvailableReplicas:   3,
			UnavailableReplicas: 0,
			ObservedGeneration:  2,
		},
	}

	tests := []struct {
		name       string
		deployment *appsv1.Deployment
		modifyFunc func(*unstructured.Unstructured)
		want       *runtime.RawExtension
		wantErr    bool
	}{
		{
			name:       "deployment with valid status and generation annotation",
			deployment: validDeployment.DeepCopy(),
			want: func() *runtime.RawExtension {
				wantStatus := &WrappedDeploymentStatus{
					FederatedGeneration: FederatedGeneration{
						Generation:                 2,
						ResourceTemplateGeneration: 1,
					},
					DeploymentStatus: validDeployment.Status,
				}
				raw, _ := helper.BuildStatusRawExtension(wantStatus)
				return raw
			}(),
			wantErr: false,
		},
		{
			name:       "deployment without status field",
			deployment: validDeployment.DeepCopy(),
			modifyFunc: func(u *unstructured.Unstructured) {
				delete(u.Object, "status")
			},
			want:    nil,
			wantErr: false,
		},
		{
			name:       "deployment with invalid generation annotation",
			deployment: validDeployment.DeepCopy(),
			modifyFunc: func(u *unstructured.Unstructured) {
				annotations := u.GetAnnotations()
				annotations[workv1alpha2.ResourceTemplateGenerationAnnotationKey] = "invalid"
				u.SetAnnotations(annotations)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:       "deployment without generation annotation",
			deployment: validDeployment.DeepCopy(),
			modifyFunc: func(u *unstructured.Unstructured) {
				annotations := u.GetAnnotations()
				delete(annotations, workv1alpha2.ResourceTemplateGenerationAnnotationKey)
				u.SetAnnotations(annotations)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert deployment to unstructured
			unstrObj, err := helper.ToUnstructured(tt.deployment)
			require.NoError(t, err, "Failed to convert deployment to unstructured")

			// Apply modifications if specified
			if tt.modifyFunc != nil {
				tt.modifyFunc(unstrObj)
			}

			got, err := reflectDeploymentStatus(unstrObj)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			assert.NotNil(t, got)
			assert.JSONEq(t, string(tt.want.Raw), string(got.Raw))
		})
	}
}

func Test_reflectServiceStatus(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			name: "non-LoadBalancer service should return nil",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"type": "ClusterIP",
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "LoadBalancer service without status should return nil",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"type": "LoadBalancer",
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "LoadBalancer service with status should return status",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"type": "LoadBalancer",
					},
					"status": map[string]interface{}{
						"loadBalancer": map[string]interface{}{
							"ingress": []interface{}{
								map[string]interface{}{
									"ip": "192.0.2.1",
								},
							},
						},
					},
				},
			},
			want: func() *runtime.RawExtension {
				status := corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{
								IP: "192.0.2.1",
							},
						},
					},
				}
				raw, _ := helper.BuildStatusRawExtension(status)
				return raw
			}(),
			wantErr: false,
		},
		{
			name: "invalid status format should return error",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"type": "LoadBalancer",
					},
					"status": "invalid",
				},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reflectServiceStatus(tt.object)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_reflectIngressStatus(t *testing.T) {
	testIngress := networkingv1.IngressStatus{
		LoadBalancer: networkingv1.IngressLoadBalancerStatus{
			Ingress: []networkingv1.IngressLoadBalancerIngress{
				{
					IP: "192.0.2.1",
				},
			},
		},
	}

	ingressStatusMap, _ := helper.ToUnstructured(&networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{Kind: "Ingress", APIVersion: networkingv1.SchemeGroupVersion.String()},
		Status:   testIngress,
	})
	wantRawExtension, _ := helper.BuildStatusRawExtension(testIngress)

	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			name:    "ingress with valid status",
			object:  ingressStatusMap,
			want:    wantRawExtension,
			wantErr: false,
		},
		{
			name: "ingress without status",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "ingress with invalid status",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": "invalid",
				},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reflectIngressStatus(tt.object)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_reflectJobStatus(t *testing.T) {
	timeNow := metav1.Now()
	completionTime := timeNow.DeepCopy()
	startTime := timeNow.DeepCopy()
	jobStatus := batchv1.JobStatus{
		Active:         1,
		Succeeded:      2,
		Failed:         0,
		CompletionTime: completionTime,
		StartTime:      startTime,
		Conditions: []batchv1.JobCondition{
			{
				Type:    "Complete",
				Status:  "True",
				Reason:  "JobComplete",
				Message: "Job completed successfully",
			},
		},
	}

	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			name: "job with all status fields",
			object: func() *unstructured.Unstructured {
				obj, _ := helper.ToUnstructured(&batchv1.Job{
					TypeMeta: metav1.TypeMeta{Kind: "Job", APIVersion: batchv1.SchemeGroupVersion.String()},
					Status:   jobStatus,
				})
				return obj
			}(),
			want: func() *runtime.RawExtension {
				raw, _ := helper.BuildStatusRawExtension(jobStatus)
				return raw
			}(),
			wantErr: false,
		},
		{
			name: "job without status",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "job with invalid status type",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"active": "invalid",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "job with empty conditions",
			object: func() *unstructured.Unstructured {
				status := jobStatus.DeepCopy()
				status.Conditions = []batchv1.JobCondition{}
				obj, _ := helper.ToUnstructured(&batchv1.Job{
					TypeMeta: metav1.TypeMeta{Kind: "Job", APIVersion: batchv1.SchemeGroupVersion.String()},
					Status:   *status,
				})
				return obj
			}(),
			want: func() *runtime.RawExtension {
				status := jobStatus.DeepCopy()
				status.Conditions = []batchv1.JobCondition{}
				raw, _ := helper.BuildStatusRawExtension(*status)
				return raw
			}(),
			wantErr: false,
		},
		{
			name: "job with failed status",
			object: func() *unstructured.Unstructured {
				status := jobStatus.DeepCopy()
				status.Failed = 1
				status.Active = 0
				status.Succeeded = 0
				status.Conditions = []batchv1.JobCondition{
					{
						Type:    "Failed",
						Status:  "True",
						Reason:  "JobFailed",
						Message: "Job failed due to error",
					},
				}
				obj, _ := helper.ToUnstructured(&batchv1.Job{
					TypeMeta: metav1.TypeMeta{Kind: "Job", APIVersion: batchv1.SchemeGroupVersion.String()},
					Status:   *status,
				})
				return obj
			}(),
			want: func() *runtime.RawExtension {
				status := jobStatus.DeepCopy()
				status.Failed = 1
				status.Active = 0
				status.Succeeded = 0
				status.Conditions = []batchv1.JobCondition{
					{
						Type:    "Failed",
						Status:  "True",
						Reason:  "JobFailed",
						Message: "Job failed due to error",
					},
				}
				raw, _ := helper.BuildStatusRawExtension(*status)
				return raw
			}(),
			wantErr: false,
		},
		{
			name: "job with only active status",
			object: func() *unstructured.Unstructured {
				status := jobStatus.DeepCopy()
				status.Failed = 0
				status.Succeeded = 0
				status.Active = 1
				status.CompletionTime = nil
				status.Conditions = []batchv1.JobCondition{}
				obj, _ := helper.ToUnstructured(&batchv1.Job{
					TypeMeta: metav1.TypeMeta{Kind: "Job", APIVersion: batchv1.SchemeGroupVersion.String()},
					Status:   *status,
				})
				return obj
			}(),
			want: func() *runtime.RawExtension {
				status := jobStatus.DeepCopy()
				status.Failed = 0
				status.Succeeded = 0
				status.Active = 1
				status.CompletionTime = nil
				status.Conditions = []batchv1.JobCondition{}
				raw, _ := helper.BuildStatusRawExtension(*status)
				return raw
			}(),
			wantErr: false,
		},
		{
			name: "job with invalid status field",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": "invalid",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "job with nil completion time",
			object: func() *unstructured.Unstructured {
				status := jobStatus.DeepCopy()
				status.CompletionTime = nil
				obj, _ := helper.ToUnstructured(&batchv1.Job{
					TypeMeta: metav1.TypeMeta{Kind: "Job", APIVersion: batchv1.SchemeGroupVersion.String()},
					Status:   *status,
				})
				return obj
			}(),
			want: func() *runtime.RawExtension {
				status := jobStatus.DeepCopy()
				status.CompletionTime = nil
				raw, _ := helper.BuildStatusRawExtension(*status)
				return raw
			}(),
			wantErr: false,
		},
		{
			name: "job with nil start time",
			object: func() *unstructured.Unstructured {
				status := jobStatus.DeepCopy()
				status.StartTime = nil
				obj, _ := helper.ToUnstructured(&batchv1.Job{
					TypeMeta: metav1.TypeMeta{Kind: "Job", APIVersion: batchv1.SchemeGroupVersion.String()},
					Status:   *status,
				})
				return obj
			}(),
			want: func() *runtime.RawExtension {
				status := jobStatus.DeepCopy()
				status.StartTime = nil
				raw, _ := helper.BuildStatusRawExtension(*status)
				return raw
			}(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reflectJobStatus(tt.object)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.JSONEq(t, string(tt.want.Raw), string(got.Raw))
			}
		})
	}
}

func Test_reflectDaemonSetStatus(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			name: "daemonset with valid status and generation annotation",
			object: func() *unstructured.Unstructured {
				ds := &appsv1.DaemonSet{
					TypeMeta: metav1.TypeMeta{
						Kind:       "DaemonSet",
						APIVersion: appsv1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test-daemonset",
						Namespace:  "test-ns",
						Generation: 2,
						Annotations: map[string]string{
							workv1alpha2.ResourceTemplateGenerationAnnotationKey: "1",
						},
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 3,
						DesiredNumberScheduled: 3,
						NumberAvailable:        3,
						NumberMisscheduled:     0,
						NumberReady:            3,
						UpdatedNumberScheduled: 3,
						NumberUnavailable:      0,
						ObservedGeneration:     2,
					},
				}
				obj, _ := helper.ToUnstructured(ds)
				return obj
			}(),
			want: func() *runtime.RawExtension {
				status := &WrappedDaemonSetStatus{
					FederatedGeneration: FederatedGeneration{
						Generation:                 2,
						ResourceTemplateGeneration: 1,
					},
					DaemonSetStatus: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 3,
						DesiredNumberScheduled: 3,
						NumberAvailable:        3,
						NumberMisscheduled:     0,
						NumberReady:            3,
						UpdatedNumberScheduled: 3,
						NumberUnavailable:      0,
						ObservedGeneration:     2,
					},
				}
				raw, _ := helper.BuildStatusRawExtension(status)
				return raw
			}(),
			wantErr: false,
		},
		{
			name: "daemonset without status field",
			object: func() *unstructured.Unstructured {
				obj := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "DaemonSet",
						"metadata": map[string]interface{}{
							"name":       "test-daemonset",
							"namespace":  "test-ns",
							"generation": int64(1),
							"annotations": map[string]interface{}{
								workv1alpha2.ResourceTemplateGenerationAnnotationKey: "1",
							},
						},
					},
				}
				return obj
			}(),
			want:    nil,
			wantErr: false,
		},
		{
			name: "daemonset with invalid generation annotation",
			object: func() *unstructured.Unstructured {
				obj := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "DaemonSet",
						"metadata": map[string]interface{}{
							"name":       "test-daemonset",
							"namespace":  "test-ns",
							"generation": int64(1),
							"annotations": map[string]interface{}{
								workv1alpha2.ResourceTemplateGenerationAnnotationKey: "invalid",
							},
						},
						"status": map[string]interface{}{
							"currentNumberScheduled": int64(1),
							"numberReady":            int64(1),
						},
					},
				}
				return obj
			}(),
			want:    nil,
			wantErr: true,
		},
		{
			name: "daemonset with invalid status format",
			object: func() *unstructured.Unstructured {
				return &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "DaemonSet",
						"metadata": map[string]interface{}{
							"name":      "test-daemonset",
							"namespace": "test-ns",
							"annotations": map[string]interface{}{
								workv1alpha2.ResourceTemplateGenerationAnnotationKey: "1",
							},
						},
						"status": "invalid",
					},
				}
			}(),
			want:    nil,
			wantErr: true,
		},
		{
			name: "daemonset without generation annotation",
			object: func() *unstructured.Unstructured {
				return &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "DaemonSet",
						"metadata": map[string]interface{}{
							"name":      "test-daemonset",
							"namespace": "test-ns",
						},
						"status": map[string]interface{}{
							"currentNumberScheduled": int64(1),
						},
					},
				}
			}(),
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reflectDaemonSetStatus(tt.object)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.JSONEq(t, string(tt.want.Raw), string(got.Raw))
			}
		})
	}
}

func Test_reflectStatefulSetStatus(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			name: "statefulset with valid status and generation annotation",
			object: func() *unstructured.Unstructured {
				sts := &appsv1.StatefulSet{
					TypeMeta: metav1.TypeMeta{
						Kind:       "StatefulSet",
						APIVersion: appsv1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test-statefulset",
						Namespace:  "test-ns",
						Generation: 2,
						Annotations: map[string]string{
							workv1alpha2.ResourceTemplateGenerationAnnotationKey: "1",
						},
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:          3,
						ReadyReplicas:     3,
						CurrentReplicas:   3,
						UpdatedReplicas:   3,
						AvailableReplicas: 3,
					},
				}
				obj, _ := helper.ToUnstructured(sts)
				return obj
			}(),
			want: func() *runtime.RawExtension {
				status := &WrappedStatefulSetStatus{
					FederatedGeneration: FederatedGeneration{
						Generation:                 2,
						ResourceTemplateGeneration: 1,
					},
					StatefulSetStatus: appsv1.StatefulSetStatus{
						Replicas:          3,
						ReadyReplicas:     3,
						CurrentReplicas:   3,
						UpdatedReplicas:   3,
						AvailableReplicas: 3,
					},
				}
				raw, _ := helper.BuildStatusRawExtension(status)
				return raw
			}(),
			wantErr: false,
		},
		{
			name: "statefulset without status field",
			object: func() *unstructured.Unstructured {
				obj := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "StatefulSet",
						"metadata": map[string]interface{}{
							"name":       "test-statefulset",
							"namespace":  "test-ns",
							"generation": int64(1),
							"annotations": map[string]interface{}{
								workv1alpha2.ResourceTemplateGenerationAnnotationKey: "1",
							},
						},
					},
				}
				return obj
			}(),
			want:    nil,
			wantErr: false,
		},
		{
			name: "statefulset with invalid status format",
			object: func() *unstructured.Unstructured {
				return &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "StatefulSet",
						"metadata": map[string]interface{}{
							"name":      "test-statefulset",
							"namespace": "test-ns",
							"annotations": map[string]interface{}{
								workv1alpha2.ResourceTemplateGenerationAnnotationKey: "1",
							},
						},
						"status": "invalid",
					},
				}
			}(),
			want:    nil,
			wantErr: true,
		},
		{
			name: "statefulset with partial status fields",
			object: func() *unstructured.Unstructured {
				sts := &appsv1.StatefulSet{
					TypeMeta: metav1.TypeMeta{
						Kind:       "StatefulSet",
						APIVersion: appsv1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test-statefulset",
						Namespace:  "test-ns",
						Generation: 2,
						Annotations: map[string]string{
							workv1alpha2.ResourceTemplateGenerationAnnotationKey: "1",
						},
					},
					Status: appsv1.StatefulSetStatus{
						Replicas:        2,
						ReadyReplicas:   1,
						CurrentReplicas: 1,
						// UpdatedReplicas and AvailableReplicas missing
					},
				}
				obj, _ := helper.ToUnstructured(sts)
				return obj
			}(),
			want: func() *runtime.RawExtension {
				status := &WrappedStatefulSetStatus{
					FederatedGeneration: FederatedGeneration{
						Generation:                 2,
						ResourceTemplateGeneration: 1,
					},
					StatefulSetStatus: appsv1.StatefulSetStatus{
						Replicas:        2,
						ReadyReplicas:   1,
						CurrentReplicas: 1,
					},
				}
				raw, _ := helper.BuildStatusRawExtension(status)
				return raw
			}(),
			wantErr: false,
		},
		{
			name: "statefulset with no annotations",
			object: func() *unstructured.Unstructured {
				return &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "StatefulSet",
						"metadata": map[string]interface{}{
							"name":      "test-statefulset",
							"namespace": "test-ns",
						},
						"status": map[string]interface{}{
							"replicas":        int64(1),
							"readyReplicas":   int64(1),
							"currentReplicas": int64(1),
						},
					},
				}
			}(),
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reflectStatefulSetStatus(tt.object)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.JSONEq(t, string(tt.want.Raw), string(got.Raw))
			}
		})
	}
}
