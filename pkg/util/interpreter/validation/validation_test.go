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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func TestVerifyDependencies(t *testing.T) {
	type args struct {
		dependencies []configv1alpha1.DependentObjectReference
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "normal case",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Kind: "Foo", Name: "test"},
				{APIVersion: "v2", Kind: "Hu", Namespace: "default", LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"bar": "foo"}}},
			}},
			wantErr: false,
		},
		{
			name: "empty apiVersion",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{Kind: "Foo", Name: "test"},
			}},
			wantErr: true,
		},
		{
			name: "empty kind",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Name: "test"},
			}},
			wantErr: true,
		},
		{
			name: "empty Name and LabelSelector at the same time",
			args: args{dependencies: []configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Kind: "Foo"},
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := VerifyDependencies(tt.args.dependencies); (err != nil) != tt.wantErr {
				t.Errorf("VerifyDependencies() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
