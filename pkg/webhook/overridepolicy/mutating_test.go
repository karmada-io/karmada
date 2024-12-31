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

package overridepolicy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

type fakeMutationDecoder struct {
	err error
	obj runtime.Object
}

// Decode mocks the Decode method of admission.Decoder.
func (f *fakeMutationDecoder) Decode(_ admission.Request, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.obj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.obj).Elem())
	}
	return nil
}

// DecodeRaw mocks the DecodeRaw method of admission.Decoder.
func (f *fakeMutationDecoder) DecodeRaw(_ runtime.RawExtension, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.obj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.obj).Elem())
	}
	return nil
}

func TestMutatingAdmission_Handle(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := MutatingAdmission{
				Decoder: tt.decoder,
			}
			got := m.Handle(context.Background(), tt.req)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Handle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutatingAdmission_Handle_FullCoverage(t *testing.T) {
	// Define the op name and namespace to be used in the test.
	policyName := "test-override-policy"
	namespace := "test-namespace"

	// Mock a request with a specific namespace.
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Namespace: namespace,
			Operation: admissionv1.Create,
		},
	}

	// Create the initial op with default values for testing.
	op := &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{Namespace: ""},
				{Namespace: ""},
			},
		},
	}

	// Define the expected op after mutations.
	wantOP := &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{Namespace: namespace},
				{Namespace: namespace},
			},
		},
	}

	// Mock decoder that decodes the request into the policy object.
	decoder := &fakeMutationDecoder{
		obj: op,
	}

	// Marshal the expected op to simulate the final mutated object.
	wantBytes, err := json.Marshal(wantOP)
	if err != nil {
		t.Fatalf("Failed to marshal expected override policy: %v", err)
	}
	req.Object.Raw = wantBytes

	// Instantiate the mutating handler.
	mutatingHandler := MutatingAdmission{
		Decoder: decoder,
	}

	// Call the Handle function.
	got := mutatingHandler.Handle(context.Background(), req)

	// Check if there are any patches applied. There should be no patches if the override policy is handled correctly.
	if len(got.Patches) > 0 {
		t.Errorf("Handle() returned patches, but no patches were expected. Got patches: %v", got.Patches)
	}

	// Check if the admission request was allowed.
	if !got.Allowed {
		t.Errorf("Handle() got.Allowed = false, want true")
	}
}
