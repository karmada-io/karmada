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
package ctrlutil

import (
	"testing"

	"github.com/stretchr/testify/assert"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

func TestWithSuspendDispatching(t *testing.T) {
	tests := []struct {
		name               string
		suspendDispatching bool
	}{
		{
			name:               "WithSuspendDispatching: true",
			suspendDispatching: true,
		},
		{
			name:               "WithSuspendDispatching: false",
			suspendDispatching: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			work := &workv1alpha1.Work{}
			applyWorkOptions(work, []WorkOption{
				WithSuspendDispatching(tt.suspendDispatching),
			})

			assert.NotNilf(t, work.Spec.SuspendDispatching, "WithSuspendDispatching(%v)", tt.suspendDispatching)
			assert.Equalf(t, tt.suspendDispatching, *work.Spec.SuspendDispatching, "WithSuspendDispatching(%v)", tt.suspendDispatching)
		})
	}
}

func TestWithPreserveResourcesOnDeletion(t *testing.T) {
	tests := []struct {
		name                        string
		preserveResourcesOnDeletion bool
	}{
		{
			name:                        "PreserveResourcesOnDeletion: true",
			preserveResourcesOnDeletion: true,
		},
		{
			name:                        "PreserveResourcesOnDeletion: false",
			preserveResourcesOnDeletion: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			work := &workv1alpha1.Work{}
			applyWorkOptions(work, []WorkOption{
				WithPreserveResourcesOnDeletion(tt.preserveResourcesOnDeletion),
			})

			assert.NotNilf(t, work.Spec.PreserveResourcesOnDeletion, "WithPreserveResourcesOnDeletion(%v)", tt.preserveResourcesOnDeletion)
			assert.Equalf(t, tt.preserveResourcesOnDeletion, *work.Spec.PreserveResourcesOnDeletion, "WithPreserveResourcesOnDeletion(%v)", tt.preserveResourcesOnDeletion)
		})
	}
}
