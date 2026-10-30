/*
Copyright 2026 The Karmada Authors.

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

package storage

import (
	"context"
	"errors"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestValidateDeletionProtection(t *testing.T) {
	var nilCluster *clusterapis.Cluster

	tests := []struct {
		name          string
		object        runtime.Object
		wantErr       bool
		wantForbidden bool
	}{
		{
			name:   "cluster without labels is allowed",
			object: &clusterapis.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member1"}},
		},
		{
			name: "cluster with unrelated label is allowed",
			object: &clusterapis.Cluster{ObjectMeta: metav1.ObjectMeta{
				Name:   "member1",
				Labels: map[string]string{"example.karmada.io/label": "value"},
			}},
		},
		{
			name: "cluster with non Always protection value is allowed",
			object: &clusterapis.Cluster{ObjectMeta: metav1.ObjectMeta{
				Name:   "member1",
				Labels: map[string]string{workv1alpha2.DeletionProtectionLabelKey: "Never"},
			}},
		},
		{
			name: "cluster with empty protection value is allowed",
			object: &clusterapis.Cluster{ObjectMeta: metav1.ObjectMeta{
				Name:   "member1",
				Labels: map[string]string{workv1alpha2.DeletionProtectionLabelKey: ""},
			}},
		},
		{
			name: "protected cluster is forbidden",
			object: &clusterapis.Cluster{ObjectMeta: metav1.ObjectMeta{
				Name:   "member1",
				Labels: map[string]string{workv1alpha2.DeletionProtectionLabelKey: workv1alpha2.DeletionProtectionAlways},
			}},
			wantErr:       true,
			wantForbidden: true,
		},
		{
			name:    "unexpected object type returns an error",
			object:  &corev1.ConfigMap{},
			wantErr: true,
		},
		{
			name:    "nil cluster returns an error",
			object:  nilCluster,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDeletionProtection(context.Background(), tt.object)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDeletionProtection() error = %v, wantErr %t", err, tt.wantErr)
			}
			if apierrors.IsForbidden(err) != tt.wantForbidden {
				t.Errorf("validateDeletionProtection() forbidden = %t, want %t", apierrors.IsForbidden(err), tt.wantForbidden)
			}
		})
	}
}

func TestComposeDeleteValidations(t *testing.T) {
	protectionErr := errors.New("protected")
	originalErr := errors.New("original validation failed")
	type validationResult struct {
		name string
		err  error
	}

	tests := []struct {
		name        string
		validations []*validationResult
		wantErr     error
		wantCalls   []string
	}{
		{
			name: "all successful validations run in order",
			validations: []*validationResult{
				{name: "protection"},
				nil,
				{name: "original"},
			},
			wantCalls: []string{"protection", "original"},
		},
		{
			name: "original validation error is returned after protection succeeds",
			validations: []*validationResult{
				{name: "protection"},
				{name: "original", err: originalErr},
			},
			wantErr:   originalErr,
			wantCalls: []string{"protection", "original"},
		},
		{
			name: "protection error prevents original validation",
			validations: []*validationResult{
				{name: "protection", err: protectionErr},
				{name: "original", err: originalErr},
			},
			wantErr:   protectionErr,
			wantCalls: []string{"protection"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := make([]string, 0)
			validations := make([]rest.ValidateObjectFunc, 0, len(tt.validations))
			for _, result := range tt.validations {
				if result == nil {
					validations = append(validations, nil)
					continue
				}
				result := result
				validations = append(validations, func(_ context.Context, _ runtime.Object) error {
					calls = append(calls, result.name)
					return result.err
				})
			}

			err := composeDeleteValidations(validations...)(context.Background(), &clusterapis.Cluster{})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("composeDeleteValidations() error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Errorf("composeDeleteValidations() calls = %v, want %v", calls, tt.wantCalls)
			}
		})
	}
}
