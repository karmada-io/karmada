/*
Copyright 2025 The Karmada Authors.

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

package clustertaintpolicy

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func TestValidatePolicyTaints(t *testing.T) {
	tests := []struct {
		name     string
		taints   []policyv1alpha1.Taint
		fldPath  *field.Path
		expected field.ErrorList
	}{
		{
			name: "Non-repeating key-value pairs",
			taints: []policyv1alpha1.Taint{
				{Key: "key1", Effect: "NoSchedule"},
				{Key: "key2", Effect: "PreferNoSchedule"},
			},
			fldPath:  field.NewPath("spec").Child("taints"),
			expected: field.ErrorList{},
		},
		{
			name: "There are duplicate key-value pairs",
			taints: []policyv1alpha1.Taint{
				{Key: "key1", Effect: "NoSchedule"},
				{Key: "key1", Effect: "NoSchedule", Value: "value1"},
			},
			fldPath: field.NewPath("spec").Child("taints"),
			expected: field.ErrorList{
				field.Duplicate(field.NewPath("spec").Child("taints").Index(0), "taint(key1:NoSchedule) already exist"),
			},
		},
		{
			name: "Some duplicate key-value pairs",
			taints: []policyv1alpha1.Taint{
				{Key: "key1", Effect: "NoSchedule"},
				{Key: "key2", Effect: "PreferNoSchedule"},
				{Key: "key1", Effect: "NoSchedule"},
			},
			fldPath: field.NewPath("spec").Child("taints"),
			expected: field.ErrorList{
				field.Duplicate(field.NewPath("spec").Child("taints").Index(0), "taint(key1:NoSchedule) already exist"),
			},
		},
		{
			name: "Multi duplicate key-value pairs",
			taints: []policyv1alpha1.Taint{
				{Key: "key1", Effect: "NoSchedule"},
				{Key: "key2", Effect: "PreferNoSchedule", Value: "value1"},
				{Key: "key1", Effect: "NoSchedule"},
				{Key: "key3", Effect: "NoExecute"},
				{Key: "key2", Effect: "PreferNoSchedule"},
			},
			fldPath: field.NewPath("spec").Child("taints"),
			expected: field.ErrorList{
				field.Duplicate(field.NewPath("spec").Child("taints").Index(0), "taint(key1:NoSchedule) already exist"),
				field.Duplicate(field.NewPath("spec").Child("taints").Index(1), "taint(key2:PreferNoSchedule) already exist"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := validatePolicyTaints(tt.taints, tt.fldPath)
			if len(actual) != len(tt.expected) {
				t.Errorf("test failed: %s, expected %v, actual %v", tt.name, tt.expected, actual)
				return
			}
			for i := range actual {
				if actual[i].Error() != tt.expected[i].Error() {
					t.Errorf("test failed: %s, expected %v, actual %v", tt.name, tt.expected[i], actual[i])
				}
			}
		})
	}
}

func TestValidatePolicyTaintsWithOtherPolicies(t *testing.T) {
	tests := []struct {
		name      string
		newPolicy *policyv1alpha1.ClusterTaintPolicy
		policies  []policyv1alpha1.ClusterTaintPolicy
		expectErr bool
		expectMsg string
	}{
		{
			name: "Situation without conflicts",
			newPolicy: &policyv1alpha1.ClusterTaintPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: policyv1alpha1.ClusterTaintPolicySpec{
					Taints: []policyv1alpha1.Taint{
						{Key: "key1", Effect: "NoSchedule"},
					},
				},
			},
			policies: []policyv1alpha1.ClusterTaintPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "policy1",
					},
					Spec: policyv1alpha1.ClusterTaintPolicySpec{
						Taints: []policyv1alpha1.Taint{
							{Key: "key1", Effect: "NoSchedule"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "policy2",
					},
					Spec: policyv1alpha1.ClusterTaintPolicySpec{
						Taints: []policyv1alpha1.Taint{
							{Key: "key2", Effect: "NoExecute"},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Situation with conflicts",
			newPolicy: &policyv1alpha1.ClusterTaintPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: policyv1alpha1.ClusterTaintPolicySpec{
					Taints: []policyv1alpha1.Taint{
						{Key: "key1", Effect: "NoSchedule"},
					},
				},
			},
			policies: []policyv1alpha1.ClusterTaintPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "policy1",
					},
					Spec: policyv1alpha1.ClusterTaintPolicySpec{
						Taints: []policyv1alpha1.Taint{
							{Key: "key1", Effect: "NoSchedule"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "policy2",
					},
					Spec: policyv1alpha1.ClusterTaintPolicySpec{
						Taints: []policyv1alpha1.Taint{
							{Key: "key1", Effect: "NoSchedule"},
						},
					},
				},
			},
			expectErr: true,
			expectMsg: "add the same taint(key1:NoSchedule) conflicted with the exist clusterTaintPolicy(policy2)",
		},
		{
			name: "Situation with multi conflicts",
			newPolicy: &policyv1alpha1.ClusterTaintPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: policyv1alpha1.ClusterTaintPolicySpec{
					Taints: []policyv1alpha1.Taint{
						{Key: "key1", Effect: "NoSchedule"},
						{Key: "key2", Effect: "NoExecute"},
					},
				},
			},
			policies: []policyv1alpha1.ClusterTaintPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "policy1",
					},
					Spec: policyv1alpha1.ClusterTaintPolicySpec{
						Taints: []policyv1alpha1.Taint{
							{Key: "key1", Effect: "NoSchedule"},
							{Key: "key2", Effect: "NoExecute"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "policy2",
					},
					Spec: policyv1alpha1.ClusterTaintPolicySpec{
						Taints: []policyv1alpha1.Taint{
							{Key: "key1", Effect: "NoSchedule"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "policy3",
					},
					Spec: policyv1alpha1.ClusterTaintPolicySpec{
						Taints: []policyv1alpha1.Taint{
							{Key: "key2", Effect: "NoExecute"},
						},
					},
				},
			},
			expectErr: true,
			expectMsg: "[add the same taint(key1:NoSchedule) conflicted with the exist clusterTaintPolicy(policy2), " +
				"add the same taint(key2:NoExecute) conflicted with the exist clusterTaintPolicy(policy3)]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePolicyTaintsWithOtherPolicies(tt.newPolicy, tt.policies)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected an error, but no error was returned")
				} else if err.Error() != tt.expectMsg {
					t.Errorf("expected: %s, actual: %s", tt.expectMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("An error was not expected, but an error was returned: %v", err)
				}
			}
		})
	}
}

func TestHandle(t *testing.T) {
	tests := []struct {
		name         string
		decodeErr    error
		decodeObj    runtime.Object
		listErr      error
		expectedCode int32
		expectedMsg  string
	}{
		{
			name:         "Decoding failed",
			decodeErr:    errors.New("decode error"),
			expectedCode: http.StatusBadRequest,
			expectedMsg:  "decode error",
		},
		{
			name:         "List Policies failed",
			listErr:      errors.New("list error"),
			expectedCode: http.StatusInternalServerError,
			expectedMsg:  "list error",
		},
		{
			name: "Validation success",
			decodeObj: &policyv1alpha1.ClusterTaintPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: policyv1alpha1.ClusterTaintPolicySpec{
					Taints: []policyv1alpha1.Taint{
						{Key: "key1", Effect: "NoSchedule"},
						{Key: "key2", Effect: "PreferNoSchedule"},
					},
				},
			},
			expectedCode: http.StatusOK,
			expectedMsg:  "",
		},
		{
			name: "Validation failed",
			decodeObj: &policyv1alpha1.ClusterTaintPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: policyv1alpha1.ClusterTaintPolicySpec{
					Taints: []policyv1alpha1.Taint{
						{Key: "key1", Effect: "NoSchedule"},
						{Key: "key2", Effect: "PreferNoSchedule"},
						{Key: "key1", Effect: "NoSchedule"},
					},
				},
			},
			expectedCode: http.StatusForbidden,
			expectedMsg:  "spec.taints[0]: Duplicate value: \"taint(key1:NoSchedule) already exist\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ValidatingAdmission{
				Decoder: &fakeValidationDecoder{err: tt.decodeErr, obj: tt.decodeObj},
				Client:  &fakeClient{listErr: tt.listErr},
			}
			resp := v.Handle(context.TODO(), admission.Request{})

			assert.Equal(t, tt.expectedCode, resp.Result.Code)
			assert.Contains(t, resp.Result.Message, tt.expectedMsg)
		})
	}
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

type fakeClient struct {
	client.Client
	listErr error
}

func (f *fakeClient) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	if f.listErr != nil {
		return f.listErr
	}
	return nil
}
