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

package propagationpolicy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler"
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
		want    admission.Response
	}{
		{
			name: "Handle_DecodeError_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				err: errors.New("decode error"),
			},
			req:  admission.Request{},
			want: admission.Errored(http.StatusBadRequest, errors.New("decode error")),
		},
		{
			name: "Handle_SchedulerNameUpdated_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.PropagationPolicy{
					Spec: policyv1alpha1.PropagationSpec{
						SchedulerName: "new-scheduler",
					},
				},
			},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					OldObject: runtime.RawExtension{
						Object: &policyv1alpha1.PropagationPolicy{
							Spec: policyv1alpha1.PropagationSpec{
								SchedulerName: scheduler.DefaultScheduler,
							},
						},
					},
				},
			},
			want: admission.Denied("the schedulerName should not be updated"),
		},
		{
			name: "Handle_PermanentIDLabelUpdated_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							policyv1alpha1.PropagationPolicyPermanentIDLabel: "new-id",
						},
					},
				},
			},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Update,
					OldObject: runtime.RawExtension{
						Object: &policyv1alpha1.PropagationPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									policyv1alpha1.PropagationPolicyPermanentIDLabel: "old-id",
								},
							},
						},
					},
				},
			},
			want: admission.Denied(fmt.Sprintf("label %s is immutable, it can only be set by the system "+
				"during creation", policyv1alpha1.PropagationPolicyPermanentIDLabel)),
		},
		{
			name: "Handle_PermanentIDLabelMissing_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{},
					},
				},
			},
			req: admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
				},
			},
			want: admission.Denied(fmt.Sprintf("label %s is required, it should be set by the mutating "+
				"admission webhook during creation", policyv1alpha1.PropagationPolicyPermanentIDLabel)),
		},
		{
			name: "Handle_ValidationSucceeds_AllowsAdmission",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							policyv1alpha1.PropagationPolicyPermanentIDLabel: "new-id",
						},
					},
					Spec: policyv1alpha1.PropagationSpec{
						SchedulerName: scheduler.DefaultScheduler,
					},
				},
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
