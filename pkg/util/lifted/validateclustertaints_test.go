/*
Copyright 2014 The Kubernetes Authors.

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

package lifted

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateClusterTaintEffect(t *testing.T) {
	tests := []struct {
		name       string
		effect     corev1.TaintEffect
		allowEmpty bool
		wantErrors int
		errorMsg   string
	}{
		{
			name:       "valid NoSchedule effect",
			effect:     corev1.TaintEffectNoSchedule,
			allowEmpty: false,
			wantErrors: 0,
		},
		{
			name:       "valid NoExecute effect",
			effect:     corev1.TaintEffectNoExecute,
			allowEmpty: false,
			wantErrors: 0,
		},
		{
			name:       "invalid effect",
			effect:     corev1.TaintEffect("InvalidEffect"),
			allowEmpty: false,
			wantErrors: 1,
			errorMsg:   "test: Unsupported value: \"InvalidEffect\": supported values: \"NoSchedule\", \"NoExecute\"",
		},
		{
			name:       "empty effect not allowed",
			effect:     corev1.TaintEffect(""),
			allowEmpty: false,
			wantErrors: 1,
			errorMsg:   "test: Required value",
		},
		{
			name:       "empty effect with allowEmpty true",
			effect:     corev1.TaintEffect(""),
			allowEmpty: true,
			wantErrors: 1,
			errorMsg:   "test: Unsupported value: \"\": supported values: \"NoSchedule\", \"NoExecute\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateClusterTaintEffect(&tt.effect, tt.allowEmpty, field.NewPath("test"))
			assert.Len(t, errors, tt.wantErrors)

			if tt.wantErrors == 0 {
				assert.Empty(t, errors)
			} else {
				assert.NotEmpty(t, errors)
				assert.Equal(t, tt.errorMsg, errors[0].Error())
			}
		})
	}
}
func TestValidateClusterTaints(t *testing.T) {
	tests := []struct {
		name       string
		taints     []corev1.Taint
		wantErrors int
	}{
		{
			name: "valid taints",
			taints: []corev1.Taint{
				{Key: "key1", Value: "value1", Effect: corev1.TaintEffectNoSchedule},
				{Key: "key2", Value: "value2", Effect: corev1.TaintEffectNoExecute},
			},
			wantErrors: 0,
		},
		{
			name: "invalid taint key",
			taints: []corev1.Taint{
				{Key: "invalid key", Value: "value1", Effect: corev1.TaintEffectNoSchedule},
			},
			wantErrors: 1,
		},
		{
			name: "invalid taint value",
			taints: []corev1.Taint{
				{Key: "key1", Value: "invalid value!", Effect: corev1.TaintEffectNoSchedule},
			},
			wantErrors: 1,
		},
		{
			name: "invalid taint effect",
			taints: []corev1.Taint{
				{Key: "key1", Value: "value1", Effect: corev1.TaintEffect("InvalidEffect")},
			},
			wantErrors: 1,
		},
		{
			name: "duplicate taints",
			taints: []corev1.Taint{
				{Key: "key1", Value: "value1", Effect: corev1.TaintEffectNoSchedule},
				{Key: "key1", Value: "value2", Effect: corev1.TaintEffectNoSchedule},
			},
			wantErrors: 1,
		},
		{
			name: "multiple errors",
			taints: []corev1.Taint{
				{Key: "invalid key", Value: "invalid value!", Effect: corev1.TaintEffect("InvalidEffect")},
				{Key: "key1", Value: "value1", Effect: corev1.TaintEffectNoSchedule},
				{Key: "key1", Value: "value2", Effect: corev1.TaintEffectNoSchedule},
			},
			wantErrors: 4,
		},
		{
			name: "empty effect",
			taints: []corev1.Taint{
				{Key: "key1", Value: "value1", Effect: corev1.TaintEffect("")},
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateClusterTaints(tt.taints, field.NewPath("test"))
			assert.Len(t, errors, tt.wantErrors)

			if tt.wantErrors == 0 {
				assert.Empty(t, errors)
			} else {
				assert.NotEmpty(t, errors)
			}
		})
	}
}
