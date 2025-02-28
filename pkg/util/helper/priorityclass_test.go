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

package helper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetPriorityClassByNameOrDefault(t *testing.T) {
	ctx := context.Background()

	// Create mock PriorityClasses
	priorityClass1 := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{Name: "high-priority"},
		Value:      1000,
	}

	defaultPriorityClass := &schedulingv1.PriorityClass{
		ObjectMeta:    metav1.ObjectMeta{Name: "default-priority"},
		Value:         500,
		GlobalDefault: true,
	}

	// Create a fake client with the objects
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(priorityClass1, defaultPriorityClass).Build()

	// Test case 1: Fetch by name
	pc, err := GetPriorityClassByNameOrDefault(ctx, fakeClient, "high-priority")
	assert.NoError(t, err)
	assert.NotNil(t, pc)
	assert.Equal(t, int32(1000), pc.Value)

	// Test case 2: Fetch default when named class does not exist
	pc, err = GetPriorityClassByNameOrDefault(ctx, fakeClient, "nonexistent")
	assert.NoError(t, err)
	assert.NotNil(t, pc)
	assert.True(t, pc.GlobalDefault)

	// Test case 3: No priority classes available
	emptyClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	pc, err = GetPriorityClassByNameOrDefault(ctx, emptyClient, "nonexistent")
	assert.Error(t, err)
	assert.Nil(t, pc)
	assert.Equal(t, "priority class nonexistent not found and no default priority class found", err.Error())
}
