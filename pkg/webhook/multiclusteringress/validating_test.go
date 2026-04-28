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

package multiclusteringress

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
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
func (f *fakeValidationDecoder) DecodeRaw(rawObject runtime.RawExtension, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if rawObject.Object != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(rawObject.Object).Elem())
		return nil
	}
	return errors.New("decode raw object error; object is nil")
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
			name:    "Handle_DecodeOldObjectError_DeniesAdmission",
			decoder: &fakeValidationDecoder{},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					OldObject: runtime.RawExtension{
						Object: nil,
					},
				},
			},
			want: TestResponse{
				Type:    Errored,
				Message: "decode raw object error; object is nil",
			},
		},
		{
			name: "Handle_UpdateMCIWithInvalidSpec_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &networkingv1alpha1.MultiClusterIngress{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-mci",
						Namespace:       "test-namespace",
						ResourceVersion: "1001",
					},
					Spec: networkingv1.IngressSpec{
						DefaultBackend: &networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: "test-new-backend",
								Port: networkingv1.ServiceBackendPort{
									Name:   "",
									Number: 80,
								},
							},
						},
						Rules: []networkingv1.IngressRule{
							{Host: "10.0.0.5"},
						},
					},
				},
			},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					OldObject: runtime.RawExtension{
						Object: &networkingv1alpha1.MultiClusterIngress{
							ObjectMeta: metav1.ObjectMeta{
								Name:            "test-mci",
								Namespace:       "test-namespace",
								ResourceVersion: "1000",
							},
							Spec: networkingv1.IngressSpec{
								DefaultBackend: &networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test-backend",
										Port: networkingv1.ServiceBackendPort{
											Name:   "",
											Number: 80,
										},
									},
								},
								Rules: []networkingv1.IngressRule{
									{Host: "127.0.0.1"},
								},
							},
						},
					},
				},
			},
			want: TestResponse{
				Type:    Denied,
				Message: "Invalid value: \"10.0.0.5\": must be a DNS name, not an IP address",
			},
		},
		{
			name: "Handle_CreateMCIWithInvalidSpec_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &networkingv1alpha1.MultiClusterIngress{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-mci",
						Namespace:       "test-namespace",
						ResourceVersion: "1000",
					},
					Spec: networkingv1.IngressSpec{
						DefaultBackend: nil,
						Rules:          nil,
					},
				},
			},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
				},
			},
			want: TestResponse{
				Type:    Denied,
				Message: "either `defaultBackend` or `rules` must be specified",
			},
		},
		{
			name: "Handle_ValidationSucceeds_AllowsAdmission",
			decoder: &fakeValidationDecoder{
				obj: &networkingv1alpha1.MultiClusterIngress{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-mci",
						Namespace:       "test-namespace",
						ResourceVersion: "1001",
					},
					Spec: networkingv1.IngressSpec{
						DefaultBackend: &networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: "test-new-backend",
								Port: networkingv1.ServiceBackendPort{
									Name:   "",
									Number: 80,
								},
							},
						},
						Rules: []networkingv1.IngressRule{
							{Host: "test-new-backend.com"},
						},
					},
				},
			},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					OldObject: runtime.RawExtension{
						Object: &networkingv1alpha1.MultiClusterIngress{
							ObjectMeta: metav1.ObjectMeta{
								Name:            "test-mci",
								Namespace:       "test-namespace",
								ResourceVersion: "1000",
							},
							Spec: networkingv1.IngressSpec{
								DefaultBackend: &networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test-backend",
										Port: networkingv1.ServiceBackendPort{
											Name:   "",
											Number: 80,
										},
									},
								},
								Rules: []networkingv1.IngressRule{
									{Host: "test-backend.com"},
								},
							},
						},
					},
				},
			},
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

func TestValidatingSpec_validateMCIUpdate(t *testing.T) {
	tests := []struct {
		name    string
		oldMcs  *networkingv1alpha1.MultiClusterIngress
		newMcs  *networkingv1alpha1.MultiClusterIngress
		wantErr bool
		errMsg  string
	}{
		{
			name: "validateMCIUpdate_ValidMetadataUpdate_NoError",
			oldMcs: &networkingv1alpha1.MultiClusterIngress{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-mci",
					Namespace:       "test-namespace",
					Labels:          map[string]string{"key": "oldValue"},
					ResourceVersion: "1000",
				},
				Spec: networkingv1.IngressSpec{
					DefaultBackend: &networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "test-backend",
							Port: networkingv1.ServiceBackendPort{
								Name:   "",
								Number: 80,
							},
						},
					},
					Rules: []networkingv1.IngressRule{
						{Host: "test-backend.com"},
					},
				},
			},
			newMcs: &networkingv1alpha1.MultiClusterIngress{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-mci",
					Namespace:       "test-namespace",
					Labels:          map[string]string{"key": "oldValue"},
					ResourceVersion: "1001",
				},
				Spec: networkingv1.IngressSpec{
					DefaultBackend: &networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "test-new-backend",
							Port: networkingv1.ServiceBackendPort{
								Name:   "",
								Number: 80,
							},
						},
					},
					Rules: []networkingv1.IngressRule{
						{Host: "test-new-backend.com"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "validateMCIUpdate_InvalidMetadataUpdate_Error",
			oldMcs: &networkingv1alpha1.MultiClusterIngress{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-mci",
					Namespace:       "test-namespace",
					ResourceVersion: "1000",
				},
			},
			newMcs: &networkingv1alpha1.MultiClusterIngress{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mutated-name",
					Namespace:       "test-namespace",
					ResourceVersion: "1001",
				},
			},
			wantErr: true,
			errMsg:  "metadata.name: Invalid value: \"mutated-name\"",
		},
		{
			name: "validateMCSUpdate_InvalidIngressLoadBalancerStatus_Error",
			oldMcs: &networkingv1alpha1.MultiClusterIngress{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-mci",
					Namespace:       "test-namespace",
					ResourceVersion: "1000",
				},
				Spec: networkingv1.IngressSpec{
					DefaultBackend: &networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "test-backend",
							Port: networkingv1.ServiceBackendPort{
								Name:   "",
								Number: 80,
							},
						},
					},
					Rules: []networkingv1.IngressRule{
						{Host: "test-backend.com"},
					},
				},
				Status: networkingv1alpha1.MultiClusterIngressStatus{
					IngressStatus: networkingv1.IngressStatus{
						LoadBalancer: networkingv1.IngressLoadBalancerStatus{
							Ingress: []networkingv1.IngressLoadBalancerIngress{
								{IP: "127.0.0.1", Hostname: "test-backend.com"},
							},
						},
					},
				},
			},
			newMcs: &networkingv1alpha1.MultiClusterIngress{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-mci",
					Namespace:       "test-namespace",
					ResourceVersion: "1001",
				},
				Spec: networkingv1.IngressSpec{
					DefaultBackend: &networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "test-new-backend",
							Port: networkingv1.ServiceBackendPort{
								Name:   "",
								Number: 80,
							},
						},
					},
					Rules: []networkingv1.IngressRule{
						{Host: "test-new-backend.com"},
					},
				},
				Status: networkingv1alpha1.MultiClusterIngressStatus{
					IngressStatus: networkingv1.IngressStatus{
						LoadBalancer: networkingv1.IngressLoadBalancerStatus{
							Ingress: []networkingv1.IngressLoadBalancerIngress{
								{IP: "test-new-backend.com", Hostname: "test-new-backend.com"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "Invalid value: \"test-new-backend.com\": must be a valid IP address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateMCIUpdate(tt.oldMcs, tt.newMcs)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("validateMCSUpdate() gotErr = %v, wantErr %v", len(errs) > 0, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(errs.ToAggregate().Error(), tt.errMsg) {
				t.Errorf("Expected error message: %v, got: %v", tt.errMsg, errs.ToAggregate().Error())
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
