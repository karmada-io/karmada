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

package validation

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func TestVerifyDependencies(t *testing.T) {
	type args struct {
		dependencies []configv1alpha1.DependentObjectReference
	}
	tests := []struct {
		name            string
		args            args
		wantErr         bool
		wantErrContains string
	}{
		{
			name: "normal case with name only",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Kind: "Foo", Name: "test"},
			}},
			wantErr: false,
		},
		{
			name: "normal case with labelSelector only",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Kind: "Foo", LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}}},
			}},
			wantErr: false,
		},
		{
			name: "normal case with both name and labelSelector",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{APIVersion: "v2", Kind: "Hu", Namespace: "default", Name: "test", LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"bar": "foo"}}},
			}},
			wantErr: false,
		},
		{
			name: "normal case with matchExpressions",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "env", Operator: metav1.LabelSelectorOpIn, Values: []string{"prod", "staging"}},
						},
					},
				},
			}},
			wantErr: false,
		},
		{
			name: "empty apiVersion",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{Kind: "Foo", Name: "test"},
			}},
			wantErr:         true,
			wantErrContains: "missing required apiVersion",
		},
		{
			name: "empty kind",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Name: "test"},
			}},
			wantErr:         true,
			wantErrContains: "missing required kind",
		},
		{
			name: "empty Name and LabelSelector at the same time",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Kind: "Foo"},
			}},
			wantErr:         true,
			wantErrContains: "dependency can not leave name and labelSelector all empty",
		},
		{
			name: "invalid label key in matchLabels",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"-invalid-key": "value",
						},
					},
				},
			}},
			wantErr:         true,
			wantErrContains: "dependencies[0].labelSelector.matchLabels",
		},
		{
			name: "invalid label value in matchLabels",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "invalid@value",
						},
					},
				},
			}},
			wantErr:         true,
			wantErrContains: "dependencies[0].labelSelector.matchLabels",
		},
		{
			name: "invalid operator in matchExpressions",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "env", Operator: "InvalidOp", Values: []string{"prod"}},
						},
					},
				},
			}},
			wantErr:         true,
			wantErrContains: "not a valid selector operator",
		},
		{
			name: "matchExpressions with In operator but no values",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "env", Operator: metav1.LabelSelectorOpIn, Values: []string{}},
						},
					},
				},
			}},
			wantErr:         true,
			wantErrContains: "must be specified when `operator` is 'In' or 'NotIn'",
		},
		{
			name: "matchExpressions with Exists operator and values",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "env", Operator: metav1.LabelSelectorOpExists, Values: []string{"prod"}},
						},
					},
				},
			}},
			wantErr:         true,
			wantErrContains: "may not be specified when `operator` is 'Exists' or 'DoesNotExist'",
		},
		{
			name: "invalid label key in matchExpressions",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "@invalid", Operator: metav1.LabelSelectorOpIn, Values: []string{"value"}},
						},
					},
				},
			}},
			wantErr:         true,
			wantErrContains: "dependencies[0].labelSelector.matchExpressions[0].key",
		},
		{
			name: "multiple dependencies with mixed errors",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Kind: "Foo", Name: "valid"},
				{Kind: "Bar", Name: "missing-apiversion"},
			}},
			wantErr:         true,
			wantErrContains: "dependencies[1].apiVersion",
		},
		{
			name: "multiple errors in single dependency",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{
					// Missing both apiVersion and kind
					Name: "test",
				},
			}},
			wantErr:         true,
			wantErrContains: "missing required",
		},
		{
			name: "label value too long",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "this-is-a-very-long-label-value-that-exceeds-the-maximum-allowed-length-of-63-characters-for-kubernetes-labels",
						},
					},
				},
			}},
			wantErr:         true,
			wantErrContains: "must be no more than 63 bytes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyDependencies(tt.args.dependencies)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.wantErrContains != "" {
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("VerifyDependencies() error = %v, want error containing %q", err, tt.wantErrContains)
				}
			}
		})
	}
}
