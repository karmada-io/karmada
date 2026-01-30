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

package clusterpropagationpolicy

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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

var (
	failOverGracePeriodSeconds int32 = 600
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
			v := NewMutatingHandler(tt.decoder)
			got := v.Handle(context.Background(), tt.req)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Handle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutatingAdmission_Handle_FullCoverage(t *testing.T) {
	// Define the cp policy name to be used in the test.
	policyName := "test-cp-policy"

	// Mock admission request with no specific namespace.
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	// Create the initial cp policy with default values for testing.
	cpPolicy := &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: policyv1alpha1.PropagationSpec{
			Placement: policyv1alpha1.Placement{
				SpreadConstraints: []policyv1alpha1.SpreadConstraint{
					{SpreadByLabel: "", SpreadByField: "", MinGroups: 0},
				},
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
				},
			},
			PropagateDeps: false,
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					Kind:       util.ServiceImportKind,
					APIVersion: mcsv1alpha1.GroupVersion.String(),
				},
			},
			Failover: &policyv1alpha1.FailoverBehavior{
				Application: &policyv1alpha1.ApplicationFailoverBehavior{
					PurgeMode:          policyv1alpha1.PurgeModeGracefully,
					GracePeriodSeconds: nil,
				},
			},
		},
	}

	// Define the expected cp policy after mutations.
	wantCPPolicy := &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
			Labels: map[string]string{
				policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "some-unique-uuid",
			},
			Finalizers: []string{util.ClusterPropagationPolicyControllerFinalizer},
		},
		Spec: policyv1alpha1.PropagationSpec{
			Placement: policyv1alpha1.Placement{
				SpreadConstraints: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						MinGroups:     1,
					},
				},
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				},
			},
			PropagateDeps: true,
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					Kind:       util.ServiceImportKind,
					APIVersion: mcsv1alpha1.GroupVersion.String(),
				},
			},
			Failover: &policyv1alpha1.FailoverBehavior{
				Application: &policyv1alpha1.ApplicationFailoverBehavior{
					PurgeMode:          policyv1alpha1.PurgeModeGracefully,
					GracePeriodSeconds: ptr.To[int32](failOverGracePeriodSeconds),
				},
			},
		},
	}

	// Mock decoder that decodes the request into the cp policy object.
	decoder := &fakeMutationDecoder{
		obj: cpPolicy,
	}

	// Marshal the expected cp policy to simulate the final mutated object.
	wantBytes, err := json.Marshal(wantCPPolicy)
	if err != nil {
		t.Fatalf("Failed to marshal expected cp policy: %v", err)
	}
	req.Object.Raw = wantBytes

	// Instantiate the mutating handler.
	mutatingHandler := NewMutatingHandler(decoder)

	// Call the Handle function.
	got := mutatingHandler.Handle(context.Background(), req)

	// Check if exactly two patches are applied (one for UUID label, one for tolerationSeconds default).
	if len(got.Patches) != 2 {
		t.Errorf("Handle() returned an unexpected number of patches. Expected two patches, received: %v", got.Patches)
	}

	// Verify that the patches applied are for the UUID label and tolerationSeconds default.
	// If any other patches are present, it indicates that the cp policy was not handled as expected.
	expectedPatches := map[string]bool{
		"/metadata/labels/clusterpropagationpolicy.karmada.io~1permanent-id": false,
		"/spec/failover/application/decisionConditions/tolerationSeconds":    false,
	}
	for _, patch := range got.Patches {
		if _, ok := expectedPatches[patch.Path]; ok {
			expectedPatches[patch.Path] = true
		} else {
			t.Errorf("Handle() returned unexpected patch path: %v", patch.Path)
		}
	}
	for path, found := range expectedPatches {
		if !found {
			t.Errorf("Handle() missing expected patch for path: %v", path)
		}
	}

	// Check if the admission request was allowed.
	if !got.Allowed {
		t.Errorf("Handle() got.Allowed = false, want true")
	}
}
