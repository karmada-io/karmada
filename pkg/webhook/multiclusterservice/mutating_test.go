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

package multiclusterservice

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

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
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
	// Define the multi-cluster service (mcs) name and namespace to be used in the test.
	name := "test-mcs"
	namespace := "test-namespace"

	// Mock a request with a specific namespace.
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Name:      name,
			Namespace: namespace,
		},
	}

	// Create the initial mcs with default values for testing.
	mcsObj := &networkingv1alpha1.MultiClusterService{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
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
	}

	// Define the expected mcs object after mutations.
	wantMCSObj := &networkingv1alpha1.MultiClusterService{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			ResourceVersion: "1001",
			Labels: map[string]string{
				networkingv1alpha1.MultiClusterServicePermanentIDLabel: "some-unique-id",
			},
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
	}

	// Mock decoder that decodes the request into the mcs object.
	decoder := &fakeMutationDecoder{
		obj: mcsObj,
	}

	// Marshal the expected policy to simulate the final mutated object.
	wantBytes, err := json.Marshal(wantMCSObj)
	if err != nil {
		t.Fatalf("Failed to marshal expected policy: %v", err)
	}
	req.Object.Raw = wantBytes

	// Instantiate the mutating handler.
	mutatingHandler := MutatingAdmission{
		Decoder: decoder,
	}

	// Call the Handle function.
	got := mutatingHandler.Handle(context.Background(), req)

	// Verify that the only patch applied is for the UUID label. If any other patches are present, it indicates that the mcs object was not handled as expected.
	if len(got.Patches) > 0 {
		firstPatch := got.Patches[0]
		if firstPatch.Operation != "replace" || firstPatch.Path != "/metadata/labels/multiclusterservice.karmada.io~1permanent-id" {
			t.Errorf("Handle() returned unexpected patches. Only the UUID patch was expected. Received patches: %v", got.Patches)
		}
	}

	// Check if the admission request was allowed.
	if !got.Allowed {
		t.Errorf("Handle() got.Allowed = false, want true")
	}
}
