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

package clusterresourcebinding

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
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
			decoder: &fakeMutationDecoder{
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
	// Define the crb object name and namespace to be used in the test.
	name := "test-cluster-resource-binding"
	namespace := "test-namespace"
	podName := "test-pod"

	// Mock an admission request.
	req := admission.Request{}

	// Create the initial crb object with default values for testing.
	crb := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource: workv1alpha2.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  namespace,
				Name:       podName,
			},
			Clusters: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 1,
				},
			},
		},
	}

	// Define the expected crb object after mutations.
	wantCRB := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				workv1alpha2.ClusterResourceBindingPermanentIDLabel: "some-unique-uuid",
			},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource: workv1alpha2.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  namespace,
				Name:       podName,
			},
			Clusters: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 1,
				},
			},
		},
	}

	// Mock decoder that decodes the request into the crb object.
	decoder := &fakeMutationDecoder{
		obj: crb,
	}

	// Marshal the expected crb object to simulate the final mutated object.
	wantBytes, err := json.Marshal(wantCRB)
	if err != nil {
		t.Fatalf("Failed to marshal expected crb object: %v", err)
	}
	req.Object.Raw = wantBytes

	// Instantiate the mutating handler.
	mutatingHandler := MutatingAdmission{
		Decoder: decoder,
	}

	// Call the Handle function.
	got := mutatingHandler.Handle(context.Background(), req)

	// Check if exactly one patch is applied.
	if len(got.Patches) != 1 {
		t.Errorf("Handle() returned an unexpected number of patches. Expected one patch, received: %v", got.Patches)
	}

	// Verify that the patches are for the UUID label.
	// If any other patches are present, it indicates that the crb object was not handled as expected.
	firstPatch := got.Patches[0]
	if firstPatch.Operation != "replace" || firstPatch.Path != "/metadata/labels/clusterresourcebinding.karmada.io~1permanent-id" {
		t.Errorf("Handle() returned unexpected patches. Only the UUID patch was expected. Received patches: %v", got.Patches)
	}

	// Check if the admission request was allowed.
	if !got.Allowed {
		t.Errorf("Handle() got.Allowed = false, want true")
	}
}
