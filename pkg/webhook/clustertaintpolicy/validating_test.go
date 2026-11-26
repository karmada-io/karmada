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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unversionedvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func TestValidatePolicyTaints(t *testing.T) {
	tests := []struct {
		name           string
		taints         []policyv1alpha1.Taint
		allowNoExecute bool
		expected       field.ErrorList
	}{
		{
			name: "Non-repeating key-value pairs",
			taints: []policyv1alpha1.Taint{
				{Key: "key1", Effect: "NoSchedule"},
				{Key: "key2", Effect: "NoExecute"},
			},
			allowNoExecute: true,
			expected:       field.ErrorList{},
		},
		{
			name: "There are duplicate key-value pairs",
			taints: []policyv1alpha1.Taint{
				{Key: "key1", Effect: "NoSchedule"},
				{Key: "key1", Effect: "NoSchedule", Value: "value1"},
			},
			allowNoExecute: true,
			expected: field.ErrorList{
				field.Duplicate(field.NewPath("spec").Child("taints").Index(0), "Duplicate taint with the same key(key1) and effect(NoSchedule) is not allowed"),
			},
		},
		{
			name: "Some duplicate key-value pairs",
			taints: []policyv1alpha1.Taint{
				{Key: "key1", Effect: "NoSchedule"},
				{Key: "key2", Effect: "NoExecute"},
				{Key: "key1", Effect: "NoSchedule"},
			},
			allowNoExecute: true,
			expected: field.ErrorList{
				field.Duplicate(field.NewPath("spec").Child("taints").Index(0), "Duplicate taint with the same key(key1) and effect(NoSchedule) is not allowed"),
			},
		},
		{
			name: "Multi duplicate key-value pairs",
			taints: []policyv1alpha1.Taint{
				{Key: "key1", Effect: "NoSchedule"},
				{Key: "key2", Effect: "NoExecute", Value: "value1"},
				{Key: "key1", Effect: "NoSchedule"},
				{Key: "key3", Effect: "NoExecute"},
				{Key: "key2", Effect: "NoExecute"},
			},
			allowNoExecute: true,
			expected: field.ErrorList{
				field.Duplicate(field.NewPath("spec").Child("taints").Index(0), "Duplicate taint with the same key(key1) and effect(NoSchedule) is not allowed"),
				field.Duplicate(field.NewPath("spec").Child("taints").Index(1), "Duplicate taint with the same key(key2) and effect(NoExecute) is not allowed"),
			},
		},
		{
			name: "NoExecute effect is not allowed",
			taints: []policyv1alpha1.Taint{
				{Key: "key1", Effect: "NoExecute"},
			},
			allowNoExecute: false,
			expected: field.ErrorList{
				field.Forbidden(field.NewPath("spec").Child("taints").Index(0), "Configuring taint with NoExecute effect is not allowed, this capability must be explicitly enabled by administrators through command-line flags"),
			},
		},
		{
			name: "Valid taint with a qualified key and a value",
			taints: []policyv1alpha1.Taint{
				{Key: "example.com/cluster-unhealthy", Value: "v1", Effect: corev1.TaintEffectNoSchedule},
			},
			allowNoExecute: true,
			expected:       field.ErrorList{},
		},
		{
			name: "Effect that Cluster does not support is rejected",
			taints: []policyv1alpha1.Taint{
				{Key: "key1", Effect: "PreferNoSchedule"},
			},
			allowNoExecute: true,
			expected: field.ErrorList{
				field.NotSupported(field.NewPath("spec").Child("taints").Index(0).Child("effect"), corev1.TaintEffect("PreferNoSchedule"),
					[]string{string(corev1.TaintEffectNoSchedule), string(corev1.TaintEffectNoExecute)}),
			},
		},
		{
			name: "Empty effect is rejected",
			taints: []policyv1alpha1.Taint{
				{Key: "key1"},
			},
			allowNoExecute: true,
			expected: field.ErrorList{
				field.Required(field.NewPath("spec").Child("taints").Index(0).Child("effect"), ""),
			},
		},
		{
			name: "Invalid key is rejected",
			taints: []policyv1alpha1.Taint{
				{Key: "not a valid key", Effect: corev1.TaintEffectNoSchedule},
			},
			allowNoExecute: true,
			expected: unversionedvalidation.ValidateLabelName("not a valid key",
				field.NewPath("spec").Child("taints").Index(0).Child("key")),
		},
		{
			name: "Empty key is rejected",
			taints: []policyv1alpha1.Taint{
				{Effect: corev1.TaintEffectNoSchedule},
			},
			allowNoExecute: true,
			expected: unversionedvalidation.ValidateLabelName("",
				field.NewPath("spec").Child("taints").Index(0).Child("key")),
		},
		{
			name: "Invalid value is rejected",
			taints: []policyv1alpha1.Taint{
				{Key: "key1", Value: "not a valid value!", Effect: corev1.TaintEffectNoSchedule},
			},
			allowNoExecute: true,
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("taints").Index(0).Child("value"), "not a valid value!",
					strings.Join(validation.IsValidLabelValue("not a valid value!"), ";")),
			},
		},
		{
			name: "Invalid taints are reported once, together with duplicates",
			taints: []policyv1alpha1.Taint{
				{Key: "key1", Effect: "PreferNoSchedule"},
				{Key: "key1", Effect: "PreferNoSchedule"},
			},
			allowNoExecute: true,
			expected: field.ErrorList{
				field.NotSupported(field.NewPath("spec").Child("taints").Index(0).Child("effect"), corev1.TaintEffect("PreferNoSchedule"),
					[]string{string(corev1.TaintEffectNoSchedule), string(corev1.TaintEffectNoExecute)}),
				field.NotSupported(field.NewPath("spec").Child("taints").Index(1).Child("effect"), corev1.TaintEffect("PreferNoSchedule"),
					[]string{string(corev1.TaintEffectNoSchedule), string(corev1.TaintEffectNoExecute)}),
				field.Duplicate(field.NewPath("spec").Child("taints").Index(0), "Duplicate taint with the same key(key1) and effect(PreferNoSchedule) is not allowed"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := validatePolicyTaints(tt.taints, field.NewPath("spec").Child("taints"), tt.allowNoExecute)
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

func TestHandle(t *testing.T) {
	tests := []struct {
		name         string
		decodeErr    error
		decodeObj    runtime.Object
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
			name: "Validation success",
			decodeObj: &policyv1alpha1.ClusterTaintPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: policyv1alpha1.ClusterTaintPolicySpec{
					Taints: []policyv1alpha1.Taint{
						{Key: "key1", Effect: "NoSchedule"},
						{Key: "key2", Effect: "NoSchedule"},
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
						{Key: "key2", Effect: "NoSchedule"},
						{Key: "key1", Effect: "NoSchedule"},
					},
				},
			},
			expectedCode: http.StatusForbidden,
			expectedMsg:  "spec.taints[0]: Duplicate value: \"Duplicate taint with the same key(key1) and effect(NoSchedule) is not allowed\"",
		},
		{
			name: "Validation failed for a taint that Cluster would reject",
			decodeObj: &policyv1alpha1.ClusterTaintPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: policyv1alpha1.ClusterTaintPolicySpec{
					Taints: []policyv1alpha1.Taint{
						{Key: "key1", Effect: "PreferNoSchedule"},
					},
				},
			},
			expectedCode: http.StatusForbidden,
			expectedMsg:  "spec.taints[0].effect: Unsupported value: \"PreferNoSchedule\": supported values: \"NoSchedule\", \"NoExecute\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ValidatingAdmission{
				Decoder: &fakeValidationDecoder{err: tt.decodeErr, obj: tt.decodeObj},
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
