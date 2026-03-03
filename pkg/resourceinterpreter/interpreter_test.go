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

package resourceinterpreter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/native"
)

func TestDefaultInterpreter_InterpretSchedulingResult(t *testing.T) {
	// Test the default interpreter directly
	interpreter := native.NewDefaultInterpreter()

	// Create test object
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")

	// Create test scheduling result
	schedulingResult := []workv1alpha2.TargetCluster{
		{Name: "cluster1", Replicas: 2},
		{Name: "cluster2", Replicas: 2},
	}

	// Test default behavior - should return the input unchanged
	result, err := interpreter.InterpretSchedulingResult(obj, schedulingResult)

	assert.NoError(t, err)
	assert.Equal(t, schedulingResult, result)
	assert.Len(t, result, 2)
	assert.Equal(t, "cluster1", result[0].Name)
	assert.Equal(t, int32(2), result[0].Replicas)
	assert.Equal(t, "cluster2", result[1].Name)
	assert.Equal(t, int32(2), result[1].Replicas)
}
