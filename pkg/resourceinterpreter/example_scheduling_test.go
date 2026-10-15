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

// TestInterpretSchedulingResultExample demonstrates how InterpretSchedulingResult
// can be used to customize replica distribution, addressing the scenarios
// described in GitHub issue #3459
func TestInterpretSchedulingResultExample(t *testing.T) {
	interpreter := native.NewDefaultInterpreter()

	// Create a deployment with 5 replicas to be distributed among 3 clusters
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	obj.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": int64(5),
		},
	}

	// Simulate initial scheduling result: [2, 2, 1] - one cluster gets remainder
	// This represents the current behavior where remainders always go to the same cluster
	initialSchedulingResult := []workv1alpha2.TargetCluster{
		{Name: "cluster-a", Replicas: 2},
		{Name: "cluster-b", Replicas: 2},
		{Name: "cluster-c", Replicas: 1}, // Always gets the remainder
	}

	// Test default behavior - should return the input unchanged
	result, err := interpreter.InterpretSchedulingResult(obj, initialSchedulingResult)

	assert.NoError(t, err)
	assert.Equal(t, initialSchedulingResult, result)

	// In a real implementation with custom logic, this could be modified to:
	// - Distribute remainders across different clusters in rotation
	// - Consider cluster load and capacity
	// - Sync replica changes from member clusters back to control plane

	t.Logf("Original scheduling result: %+v", initialSchedulingResult)
	t.Logf("Interpreted result (default): %+v", result)

	// Example of what custom logic could achieve:
	// Round 1: [2, 2, 1] - cluster-c gets remainder
	// Round 2: [2, 1, 2] - cluster-b gets remainder
	// Round 3: [1, 2, 2] - cluster-a gets remainder
	// This provides more balanced distribution over time
}
