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

package hpascaletargetmarker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestSetupWithManager(t *testing.T) {
	// Create a new scheme for the fake dynamic client
	scheme := runtime.NewScheme()

	tests := []struct {
		name   string
		marker *HpaScaleTargetMarker
	}{
		{
			name: "Valid setup",
			marker: &HpaScaleTargetMarker{
				DynamicClient: fake.NewSimpleDynamicClient(scheme),
				RESTMapper:    meta.NewDefaultRESTMapper([]schema.GroupVersion{}),
			},
		},
		{
			name: "Missing DynamicClient",
			marker: &HpaScaleTargetMarker{
				RESTMapper: meta.NewDefaultRESTMapper([]schema.GroupVersion{}),
			},
		},
		{
			name: "Missing RESTMapper",
			marker: &HpaScaleTargetMarker{
				DynamicClient: fake.NewSimpleDynamicClient(scheme),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new manager for each test case
			mgr, err := manager.New(&rest.Config{}, manager.Options{})
			assert.NoError(t, err)

			err = tt.marker.SetupWithManager(mgr)
			assert.NoError(t, err, "SetupWithManager should not return an error")

			assert.NotNil(t, tt.marker.scaleTargetWorker, "scaleTargetWorker should be initialized")
		})
	}
}

func TestHpaScaleTargetMarker_Reconcile(t *testing.T) {
	r := &HpaScaleTargetMarker{}

	// Call Reconcile with an empty request
	result, err := r.Reconcile(context.Background(), controllerruntime.Request{})

	assert.Equal(t, controllerruntime.Result{}, result, "Result should be empty")
	assert.NoError(t, err, "Error should be nil")
}
