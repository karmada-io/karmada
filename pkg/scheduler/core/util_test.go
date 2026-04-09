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

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestFindClusterReplicas(t *testing.T) {
	clusters := []workv1alpha2.TargetCluster{
		{Name: "member-a", Replicas: 5},
		{Name: "member-b", Replicas: 3},
	}

	tests := []struct {
		name     string
		cluster  string
		expected int32
	}{
		{"found member-a", "member-a", 5},
		{"found member-b", "member-b", 3},
		{"not found", "member-c", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, findClusterReplicas(clusters, tt.cluster))
		})
	}
}
