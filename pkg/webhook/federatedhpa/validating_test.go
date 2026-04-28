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

package federatedhpa

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
)

// ResponseType represents the type of admission response.
type ResponseType string

const (
	Denied  ResponseType = "Denied"
	Allowed ResponseType = "Allowed"
	Errored ResponseType = "Errored"
)

// TestResponse is used to define expected response in a test case.
type TestResponse struct {
	Type    ResponseType
	Message string
}

type fakeValidationDecoder struct {
	err error
	obj runtime.Object
}

// Decode mocks the Decode method of admission.Decoder.
func (f *fakeValidationDecoder) Decode(_ admission.Request, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.obj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.obj).Elem())
	}
	return nil
}

// DecodeRaw mocks the DecodeRaw method of admission.Decoder.
func (f *fakeValidationDecoder) DecodeRaw(_ runtime.RawExtension, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.obj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.obj).Elem())
	}
	return nil
}

func TestValidatingAdmission_Handle(t *testing.T) {
	tests := []struct {
		name    string
		decoder admission.Decoder
		req     admission.Request
		want    TestResponse
	}{
		{
			name: "Handle_DecodeError_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				err: errors.New("decode error"),
			},
			req: admission.Request{},
			want: TestResponse{
				Type:    Errored,
				Message: "decode error",
			},
		},
		{
			name: "Handle_ValidationFederatedHPASpecFails_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &autoscalingv1alpha1.FederatedHPA{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app",
						Namespace: "test-namespace",
					},
					Spec: autoscalingv1alpha1.FederatedHPASpec{
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-app",
						},
						MinReplicas: ptr.To[int32](0),
						MaxReplicas: 10,
						Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
							ScaleUp: &autoscalingv2.HPAScalingRules{
								StabilizationWindowSeconds: ptr.To[int32](0),
								Policies: []autoscalingv2.HPAScalingPolicy{
									{
										PeriodSeconds: 15,
										Type:          autoscalingv2.PodsScalingPolicy,
										Value:         4,
									},
									{
										PeriodSeconds: 15,
										Type:          autoscalingv2.PercentScalingPolicy,
										Value:         100,
									},
								},
							},
							ScaleDown: &autoscalingv2.HPAScalingRules{
								StabilizationWindowSeconds: ptr.To[int32](300),
								Policies: []autoscalingv2.HPAScalingPolicy{
									{
										PeriodSeconds: 15,
										Type:          autoscalingv2.PercentScalingPolicy,
										Value:         100,
									},
								},
							},
						},
						Metrics: []autoscalingv2.MetricSpec{
							{
								Type: autoscalingv2.PodsMetricSourceType,
								Pods: &autoscalingv2.PodsMetricSource{
									Metric: autoscalingv2.MetricIdentifier{
										Name: "http_requests",
									},
									Target: autoscalingv2.MetricTarget{
										AverageValue: resource.NewMilliQuantity(700, resource.DecimalSI),
										Type:         autoscalingv2.ValueMetricType,
									},
								},
							},
						},
					},
					Status: autoscalingv2.HorizontalPodAutoscalerStatus{
						CurrentReplicas: 2,
						DesiredReplicas: 2,
					},
				},
			},
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: "spec.minReplicas: Invalid value: 0",
			},
		},
		{
			name: "Handle_ValidationSucceeds_AllowsAdmission",
			decoder: &fakeValidationDecoder{
				obj: &autoscalingv1alpha1.FederatedHPA{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app",
						Namespace: "test-namespace",
					},
					Spec: autoscalingv1alpha1.FederatedHPASpec{
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-app",
						},
						MinReplicas: ptr.To[int32](1),
						MaxReplicas: 10,
						Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
							ScaleUp: &autoscalingv2.HPAScalingRules{
								StabilizationWindowSeconds: ptr.To[int32](0),
								Policies: []autoscalingv2.HPAScalingPolicy{
									{
										PeriodSeconds: 15,
										Type:          autoscalingv2.PodsScalingPolicy,
										Value:         4,
									},
									{
										PeriodSeconds: 15,
										Type:          autoscalingv2.PercentScalingPolicy,
										Value:         100,
									},
								},
							},
							ScaleDown: &autoscalingv2.HPAScalingRules{
								StabilizationWindowSeconds: ptr.To[int32](300),
								Policies: []autoscalingv2.HPAScalingPolicy{
									{
										PeriodSeconds: 15,
										Type:          autoscalingv2.PercentScalingPolicy,
										Value:         100,
									},
								},
							},
						},
						Metrics: []autoscalingv2.MetricSpec{
							{
								Type: autoscalingv2.PodsMetricSourceType,
								Pods: &autoscalingv2.PodsMetricSource{
									Metric: autoscalingv2.MetricIdentifier{
										Name: "http_requests",
									},
									Target: autoscalingv2.MetricTarget{
										AverageValue: resource.NewMilliQuantity(700, resource.DecimalSI),
										Type:         autoscalingv2.ValueMetricType,
									},
								},
							},
						},
					},
					Status: autoscalingv2.HorizontalPodAutoscalerStatus{
						CurrentReplicas: 2,
						DesiredReplicas: 2,
					},
				},
			},
			req: admission.Request{},
			want: TestResponse{
				Type:    Allowed,
				Message: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ValidatingAdmission{
				Decoder: tt.decoder,
			}
			got := v.Handle(context.Background(), tt.req)

			// Extract type and message from the actual response.
			gotType := extractResponseType(got)
			gotMessage := extractErrorMessage(got)

			if gotType != tt.want.Type || !strings.Contains(gotMessage, tt.want.Message) {
				t.Errorf("Handle() = {Type: %v, Message: %v}, want {Type: %v, Message: %v}", gotType, gotMessage, tt.want.Type, tt.want.Message)
			}
		})
	}
}

// extractResponseType extracts the type of admission response.
func extractResponseType(resp admission.Response) ResponseType {
	if resp.Allowed {
		return Allowed
	}
	if resp.Result != nil {
		if resp.Result.Code == http.StatusBadRequest {
			return Errored
		}
	}
	return Denied
}

// extractErrorMessage extracts the error message from a Denied/Errored response.
func extractErrorMessage(resp admission.Response) string {
	if !resp.Allowed && resp.Result != nil {
		return resp.Result.Message
	}
	return ""
}
