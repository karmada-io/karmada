/*
Copyright 2023 The Karmada Authors.

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

package multiclusterservice

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
			name: "Handle_DecodeOldObjectError_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				err: errors.New("decode raw error"),
			},
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
				Message: "decode raw error",
			},
		},
		{
			name: "Handle_UpdateMCSWithInvalidSpec_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &networkingv1alpha1.MultiClusterService{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-mcs",
						Namespace:       "test-namespace",
						ResourceVersion: "1001",
					},
					Spec: networkingv1alpha1.MultiClusterServiceSpec{
						Ports: []networkingv1alpha1.ExposurePort{
							{
								Name: "foo.withdot",
								Port: 16312,
							},
							{
								Name: "bar",
								Port: 16313,
							},
						},
						Types: []networkingv1alpha1.ExposureType{
							networkingv1alpha1.ExposureTypeLoadBalancer,
						},
						ProviderClusters: []networkingv1alpha1.ClusterSelector{
							{Name: "member1"},
							{Name: "member2"},
						},
						ConsumerClusters: []networkingv1alpha1.ClusterSelector{
							{Name: "member1"},
							{Name: "member2"},
						},
					},
				},
			},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					OldObject: runtime.RawExtension{
						Object: &networkingv1alpha1.MultiClusterService{
							ObjectMeta: metav1.ObjectMeta{
								Name:            "test-mcs",
								Namespace:       "test-namespace",
								ResourceVersion: "1000",
							},
							Spec: networkingv1alpha1.MultiClusterServiceSpec{
								Types: []networkingv1alpha1.ExposureType{
									networkingv1alpha1.ExposureTypeLoadBalancer,
								},
							},
						},
					},
				},
			},
			want: TestResponse{
				Type:    Denied,
				Message: "Invalid value: \"foo.withdot\": must not contain dots",
			},
		},
		{
			name: "Handle_CreateMCSWithInvalidSpec_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &networkingv1alpha1.MultiClusterService{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-mcs",
						Namespace: "test-namespace",
					},
					Spec: networkingv1alpha1.MultiClusterServiceSpec{
						Ports: []networkingv1alpha1.ExposurePort{
							{
								Name: "foo.withdot",
								Port: 16312,
							},
							{
								Name: "bar",
								Port: 16313,
							},
						},
						Types: []networkingv1alpha1.ExposureType{
							networkingv1alpha1.ExposureTypeLoadBalancer,
						},
						ProviderClusters: []networkingv1alpha1.ClusterSelector{
							{Name: "member1"},
							{Name: "member2"},
						},
						ConsumerClusters: []networkingv1alpha1.ClusterSelector{
							{Name: "member1"},
							{Name: "member2"},
						},
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
				Message: "Invalid value: \"foo.withdot\": must not contain dots",
			},
		},
		{
			name: "Handle_ValidationSucceeds_AllowsAdmission",
			decoder: &fakeValidationDecoder{
				obj: &networkingv1alpha1.MultiClusterService{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-mcs",
						Namespace:       "test-namespace",
						ResourceVersion: "1001",
					},
					Spec: networkingv1alpha1.MultiClusterServiceSpec{
						Ports: []networkingv1alpha1.ExposurePort{
							{
								Name: "foo",
								Port: 16312,
							},
							{
								Name: "bar",
								Port: 16313,
							},
						},
						ProviderClusters: []networkingv1alpha1.ClusterSelector{
							{Name: "member1"},
							{Name: "member2"},
						},
						ConsumerClusters: []networkingv1alpha1.ClusterSelector{
							{Name: "member1"},
							{Name: "member2"},
						},
						Types: []networkingv1alpha1.ExposureType{
							networkingv1alpha1.ExposureTypeLoadBalancer,
						},
					},
				},
			},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					OldObject: runtime.RawExtension{
						Object: &networkingv1alpha1.MultiClusterService{
							ObjectMeta: metav1.ObjectMeta{
								Name:            "test-mcs",
								Namespace:       "test-namespace",
								ResourceVersion: "1000",
							},
							Spec: networkingv1alpha1.MultiClusterServiceSpec{
								Types: []networkingv1alpha1.ExposureType{
									networkingv1alpha1.ExposureTypeLoadBalancer,
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

func TestValidateMultiClusterServiceSpec(t *testing.T) {
	validator := &ValidatingAdmission{}
	specFld := field.NewPath("spec")

	tests := []struct {
		name        string
		mcs         *networkingv1alpha1.MultiClusterService
		expectedErr field.ErrorList
	}{
		{
			name: "normal mcs",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Ports: []networkingv1alpha1.ExposurePort{
						{
							Name: "foo",
							Port: 16312,
						},
						{
							Name: "bar",
							Port: 16313,
						},
					},
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeLoadBalancer,
					},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
				},
			},
			expectedErr: field.ErrorList{},
		},
		{
			name: "multiple exposure type mcs",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Ports: []networkingv1alpha1.ExposurePort{
						{
							Name: "foo",
							Port: 16312,
						},
						{
							Name: "bar",
							Port: 16313,
						},
					},
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeLoadBalancer,
						networkingv1alpha1.ExposureTypeCrossCluster,
					},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
				},
			},
			expectedErr: field.ErrorList{field.Invalid(specFld.Child("types"), []networkingv1alpha1.ExposureType{
				networkingv1alpha1.ExposureTypeLoadBalancer,
				networkingv1alpha1.ExposureTypeCrossCluster,
			}, "MultiClusterService types should not contain more than one type")},
		},
		{
			name: "duplicated svc name",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Ports: []networkingv1alpha1.ExposurePort{
						{
							Name: "foo",
							Port: 16312,
						},
						{
							Name: "foo",
							Port: 16313,
						},
					},
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeLoadBalancer,
						networkingv1alpha1.ExposureTypeLoadBalancer,
					},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
				},
			},
			expectedErr: field.ErrorList{field.Duplicate(specFld.Child("ports").Index(1).Child("name"), "foo")},
		},
		{
			name: "invalid svc port",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Ports: []networkingv1alpha1.ExposurePort{
						{
							Name: "foo",
							Port: 163121,
						},
					},
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeLoadBalancer,
					},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
				},
			},
			expectedErr: field.ErrorList{field.Invalid(specFld.Child("ports").Index(0).Child("port"), int32(163121), validation.InclusiveRangeError(1, 65535))},
		},
		{
			name: "invalid ExposureType",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Ports: []networkingv1alpha1.ExposurePort{
						{
							Name: "foo",
							Port: 16312,
						},
					},
					Types: []networkingv1alpha1.ExposureType{
						"",
					},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
				},
			},
			expectedErr: field.ErrorList{field.Invalid(specFld.Child("types").Index(0), networkingv1alpha1.ExposureType(""), "ExposureType Error")},
		},
		{
			name: "invalid cluster name",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Ports: []networkingv1alpha1.ExposurePort{
						{
							Name: "foo",
							Port: 16312,
						},
					},
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeCrossCluster,
					},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{
						{Name: strings.Repeat("a", 49)},
					},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{},
				},
			},
			expectedErr: field.ErrorList{field.Invalid(specFld.Child("range").Child("providerClusters").Index(0), strings.Repeat("a", 49), "must be no more than 48 characters")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validator.validateMultiClusterServiceSpec(tt.mcs); !reflect.DeepEqual(got, tt.expectedErr) {
				t.Errorf("validateMultiClusterServiceSpec() = %v, want %v", got, tt.expectedErr)
			}
		})
	}
}

func TestValidatingSpec_validateMCSUpdate(t *testing.T) {
	tests := []struct {
		name    string
		oldMcs  *networkingv1alpha1.MultiClusterService
		newMcs  *networkingv1alpha1.MultiClusterService
		wantErr bool
		errMsg  string
	}{
		{
			name: "validateMCSUpdate_ValidMetadataUpdate_NoError",
			oldMcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-mcs",
					Namespace:       "test-namespace",
					Labels:          map[string]string{"key": "oldValue"},
					ResourceVersion: "1000",
				},
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeLoadBalancer,
					},
				},
			},
			newMcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-mcs",
					Namespace:       "test-namespace",
					Labels:          map[string]string{"key": "newValue"},
					ResourceVersion: "1001",
				},
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeLoadBalancer,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "validateMCSUpdate_InvalidMetadataUpdate_Error",
			oldMcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-mcs",
					Namespace:       "test-namespace",
					ResourceVersion: "1000",
				},
			},
			newMcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "invalid-name",
					Namespace:       "test-namespace",
					ResourceVersion: "1001",
				},
			},
			wantErr: true,
			errMsg:  "metadata.name: Invalid value: \"invalid-name\"",
		},
		{
			name: "validateMCSUpdate_InvalidTypesUpdate_Error",
			oldMcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-mcs",
					Namespace:       "test-namespace",
					ResourceVersion: "1000",
				},
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeLoadBalancer,
					},
				},
			},
			newMcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-mcs",
					Namespace:       "test-namespace",
					ResourceVersion: "1001",
				},
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeCrossCluster,
					},
				},
			},
			wantErr: true,
			errMsg:  "MultiClusterService types are immutable",
		},
		{
			name: "validateMCSUpdate_InvalidLoadBalancerStatus_Error",
			oldMcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-mcs",
					Namespace:       "test-namespace",
					ResourceVersion: "1000",
				},
			},
			newMcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-mcs",
					Namespace:       "test-namespace",
					ResourceVersion: "1001",
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{IP: "invalid IP"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "Invalid value: \"invalid IP\": must be a valid IP address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ValidatingAdmission{}
			errs := v.validateMCSUpdate(tt.oldMcs, tt.newMcs)
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
