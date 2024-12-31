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

package resourcedeletionprotection

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

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
func (f *fakeValidationDecoder) DecodeRaw(rawObj runtime.RawExtension, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if rawObj.Object != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(rawObj.Object).Elem())
	}
	return nil
}

func TestValidatingAdmission_Handle(t *testing.T) {
	tests := []struct {
		name    string
		decoder admission.Decoder
		req     admission.Request
		want    admission.Response
	}{
		{
			name: "Handle_DecodeRawError_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				err: errors.New("decode error"),
			},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Delete,
				},
			},
			want: admission.Errored(http.StatusBadRequest, errors.New("decode error")),
		},
		{
			name:    "Handle_DeleteWithProtectionLabel_DeniesAdmission",
			decoder: &fakeValidationDecoder{},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Delete,
					OldObject: runtime.RawExtension{
						Object: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"metadata": map[string]interface{}{
									"labels": map[string]interface{}{
										workv1alpha2.DeletionProtectionLabelKey: workv1alpha2.DeletionProtectionAlways,
									},
								},
							},
						},
					},
				},
			},
			want: admission.Denied(fmt.Sprintf("This resource is protected, please make sure to remove the label: %s", workv1alpha2.DeletionProtectionLabelKey)),
		},
		{
			name:    "Handle_DeleteWithoutLabel_AllowsAdmission",
			decoder: &fakeValidationDecoder{},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Delete,
					OldObject: runtime.RawExtension{
						Object: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"metadata": map[string]interface{}{
									"labels": map[string]interface{}{},
								},
							},
						},
					},
				},
			},
			want: admission.Allowed(""),
		},
		{
			name:    "Handle_DeleteWithDifferentLabel_AllowsAdmission",
			decoder: &fakeValidationDecoder{},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Delete,
					OldObject: runtime.RawExtension{
						Object: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"metadata": map[string]interface{}{
									"labels": map[string]interface{}{
										"some-other-label": "some-value",
									},
								},
							},
						},
					},
				},
			},
			want: admission.Allowed(""),
		},
		{
			name: "Handle_CreateOperation_AllowsAdmission",
			decoder: &fakeValidationDecoder{
				err: errors.New("decode error"),
			},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
				},
			},
			want: admission.Allowed(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ValidatingAdmission{
				Decoder: tt.decoder,
			}
			got := v.Handle(context.Background(), tt.req)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Handle() = %v, want %v", got, tt.want)
			}
		})
	}
}
