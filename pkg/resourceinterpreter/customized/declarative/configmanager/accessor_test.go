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

package configmanager

import (
	"testing"

	"github.com/stretchr/testify/assert"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func TestNewResourceCustomAccessor(t *testing.T) {
	accessor := NewResourceCustomAccessor()
	assert.NotNil(t, accessor, "Expected non-nil accessor")
}

func TestGetRetentionLuaScript(t *testing.T) {
	tests := []struct {
		name     string
		accessor *resourceCustomAccessor
		want     string
	}{
		{
			name:     "nil retention",
			accessor: &resourceCustomAccessor{},
			want:     "",
		},
		{
			name: "with script",
			accessor: &resourceCustomAccessor{
				retention: &configv1alpha1.LocalValueRetention{LuaScript: "test-script"},
			},
			want: "test-script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.accessor.GetRetentionLuaScript()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetReplicaResourceLuaScript(t *testing.T) {
	tests := []struct {
		name     string
		accessor *resourceCustomAccessor
		want     string
	}{
		{
			name:     "nil replica resource",
			accessor: &resourceCustomAccessor{},
			want:     "",
		},
		{
			name: "with script",
			accessor: &resourceCustomAccessor{
				replicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "test-script"},
			},
			want: "test-script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.accessor.GetReplicaResourceLuaScript()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetReplicaRevisionLuaScript(t *testing.T) {
	tests := []struct {
		name     string
		accessor *resourceCustomAccessor
		want     string
	}{
		{
			name:     "nil replica revision",
			accessor: &resourceCustomAccessor{},
			want:     "",
		},
		{
			name: "with script",
			accessor: &resourceCustomAccessor{
				replicaRevision: &configv1alpha1.ReplicaRevision{LuaScript: "test-script"},
			},
			want: "test-script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.accessor.GetReplicaRevisionLuaScript()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetStatusReflectionLuaScript(t *testing.T) {
	tests := []struct {
		name     string
		accessor *resourceCustomAccessor
		want     string
	}{
		{
			name:     "nil status reflection",
			accessor: &resourceCustomAccessor{},
			want:     "",
		},
		{
			name: "with script",
			accessor: &resourceCustomAccessor{
				statusReflection: &configv1alpha1.StatusReflection{LuaScript: "test-script"},
			},
			want: "test-script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.accessor.GetStatusReflectionLuaScript()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetStatusAggregationLuaScript(t *testing.T) {
	tests := []struct {
		name     string
		accessor *resourceCustomAccessor
		want     string
	}{
		{
			name:     "nil status aggregation",
			accessor: &resourceCustomAccessor{},
			want:     "",
		},
		{
			name: "with script",
			accessor: &resourceCustomAccessor{
				statusAggregation: &configv1alpha1.StatusAggregation{LuaScript: "test-script"},
			},
			want: "test-script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.accessor.GetStatusAggregationLuaScript()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetHealthInterpretationLuaScript(t *testing.T) {
	tests := []struct {
		name     string
		accessor *resourceCustomAccessor
		want     string
	}{
		{
			name:     "nil health interpretation",
			accessor: &resourceCustomAccessor{},
			want:     "",
		},
		{
			name: "with script",
			accessor: &resourceCustomAccessor{
				healthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "test-script"},
			},
			want: "test-script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.accessor.GetHealthInterpretationLuaScript()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetDependencyInterpretationLuaScripts(t *testing.T) {
	tests := []struct {
		name     string
		accessor *resourceCustomAccessor
		want     []string
	}{
		{
			name:     "nil dependencies",
			accessor: &resourceCustomAccessor{},
			want:     nil,
		},
		{
			name: "empty dependencies",
			accessor: &resourceCustomAccessor{
				dependencyInterpretations: []*configv1alpha1.DependencyInterpretation{},
			},
			want: nil,
		},
		{
			name: "with scripts",
			accessor: &resourceCustomAccessor{
				dependencyInterpretations: []*configv1alpha1.DependencyInterpretation{
					{LuaScript: "script1"},
					{LuaScript: ""},
					{LuaScript: "script2"},
				},
			},
			want: []string{"script1", "script2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.accessor.GetDependencyInterpretationLuaScripts()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name    string
		initial *resourceCustomAccessor
		rules   configv1alpha1.CustomizationRules
		want    *resourceCustomAccessor
	}{
		{
			name:    "merge into empty accessor",
			initial: &resourceCustomAccessor{},
			rules: configv1alpha1.CustomizationRules{
				Retention: &configv1alpha1.LocalValueRetention{LuaScript: "script1"},
			},
			want: &resourceCustomAccessor{
				retention: &configv1alpha1.LocalValueRetention{LuaScript: "script1"},
			},
		},
		{
			name:    "merge status aggregation into empty accessor",
			initial: &resourceCustomAccessor{},
			rules: configv1alpha1.CustomizationRules{
				StatusAggregation: &configv1alpha1.StatusAggregation{LuaScript: "new-script"},
			},
			want: &resourceCustomAccessor{
				statusAggregation: &configv1alpha1.StatusAggregation{LuaScript: "new-script"},
			},
		},
		{
			name: "merge status aggregation with existing empty script",
			initial: &resourceCustomAccessor{
				statusAggregation: &configv1alpha1.StatusAggregation{LuaScript: ""},
			},
			rules: configv1alpha1.CustomizationRules{
				StatusAggregation: &configv1alpha1.StatusAggregation{LuaScript: "new-script"},
			},
			want: &resourceCustomAccessor{
				statusAggregation: &configv1alpha1.StatusAggregation{LuaScript: "new-script"},
			},
		},
		{
			name: "merge status aggregation with existing non-empty script",
			initial: &resourceCustomAccessor{
				statusAggregation: &configv1alpha1.StatusAggregation{LuaScript: "existing"},
			},
			rules: configv1alpha1.CustomizationRules{
				StatusAggregation: &configv1alpha1.StatusAggregation{LuaScript: "new-script"},
			},
			want: &resourceCustomAccessor{
				statusAggregation: &configv1alpha1.StatusAggregation{LuaScript: "existing"},
			},
		},
		{
			name:    "merge health interpretation into empty accessor",
			initial: &resourceCustomAccessor{},
			rules: configv1alpha1.CustomizationRules{
				HealthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "new-script"},
			},
			want: &resourceCustomAccessor{
				healthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "new-script"},
			},
		},
		{
			name: "merge health interpretation with existing empty script",
			initial: &resourceCustomAccessor{
				healthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: ""},
			},
			rules: configv1alpha1.CustomizationRules{
				HealthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "new-script"},
			},
			want: &resourceCustomAccessor{
				healthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "new-script"},
			},
		},
		{
			name: "merge health interpretation with existing non-empty script",
			initial: &resourceCustomAccessor{
				healthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "existing"},
			},
			rules: configv1alpha1.CustomizationRules{
				HealthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "new-script"},
			},
			want: &resourceCustomAccessor{
				healthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "existing"},
			},
		},
		{
			name: "merge multiple fields at once",
			initial: &resourceCustomAccessor{
				statusAggregation:    &configv1alpha1.StatusAggregation{LuaScript: ""},
				healthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "existing"},
			},
			rules: configv1alpha1.CustomizationRules{
				StatusAggregation:    &configv1alpha1.StatusAggregation{LuaScript: "new-agg"},
				HealthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "new-health"},
			},
			want: &resourceCustomAccessor{
				statusAggregation:    &configv1alpha1.StatusAggregation{LuaScript: "new-agg"},
				healthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "existing"},
			},
		},
		{
			name: "merge with existing values",
			initial: &resourceCustomAccessor{
				retention: &configv1alpha1.LocalValueRetention{LuaScript: "existing"},
			},
			rules: configv1alpha1.CustomizationRules{
				Retention: &configv1alpha1.LocalValueRetention{LuaScript: "new"},
			},
			want: &resourceCustomAccessor{
				retention: &configv1alpha1.LocalValueRetention{LuaScript: "existing"},
			},
		},
		{
			name: "merge with existing empty script",
			initial: &resourceCustomAccessor{
				retention: &configv1alpha1.LocalValueRetention{LuaScript: ""},
			},
			rules: configv1alpha1.CustomizationRules{
				Retention: &configv1alpha1.LocalValueRetention{LuaScript: "new"},
			},
			want: &resourceCustomAccessor{
				retention: &configv1alpha1.LocalValueRetention{LuaScript: "new"},
			},
		},
		{
			name: "merge dependency interpretation",
			initial: &resourceCustomAccessor{
				dependencyInterpretations: []*configv1alpha1.DependencyInterpretation{
					{LuaScript: "existing"},
				},
			},
			rules: configv1alpha1.CustomizationRules{
				DependencyInterpretation: &configv1alpha1.DependencyInterpretation{LuaScript: "new"},
			},
			want: &resourceCustomAccessor{
				dependencyInterpretations: []*configv1alpha1.DependencyInterpretation{
					{LuaScript: "existing"},
					{LuaScript: "new"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.Merge(tt.rules)
			assert.Equal(t, tt.want, tt.initial)
		})
	}
}

func TestSetRetain(t *testing.T) {
	tests := []struct {
		name    string
		initial *resourceCustomAccessor
		input   *configv1alpha1.LocalValueRetention
		want    string
	}{
		{
			name:    "set on nil field",
			initial: &resourceCustomAccessor{},
			input:   &configv1alpha1.LocalValueRetention{LuaScript: "script1"},
			want:    "script1",
		},
		{
			name: "set on empty script",
			initial: &resourceCustomAccessor{
				retention: &configv1alpha1.LocalValueRetention{LuaScript: ""},
			},
			input: &configv1alpha1.LocalValueRetention{LuaScript: "script1"},
			want:  "script1",
		},
		{
			name: "set on non-empty script",
			initial: &resourceCustomAccessor{
				retention: &configv1alpha1.LocalValueRetention{LuaScript: "existing"},
			},
			input: &configv1alpha1.LocalValueRetention{LuaScript: "script1"},
			want:  "existing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.setRetain(tt.input)
			assert.Equal(t, tt.want, tt.initial.GetRetentionLuaScript())
		})
	}
}

func TestSetReplicaResource(t *testing.T) {
	tests := []struct {
		name    string
		initial *resourceCustomAccessor
		input   *configv1alpha1.ReplicaResourceRequirement
		want    string
	}{
		{
			name:    "set on nil field",
			initial: &resourceCustomAccessor{},
			input:   &configv1alpha1.ReplicaResourceRequirement{LuaScript: "script1"},
			want:    "script1",
		},
		{
			name: "set on empty script",
			initial: &resourceCustomAccessor{
				replicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: ""},
			},
			input: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "script1"},
			want:  "script1",
		},
		{
			name: "set on non-empty script",
			initial: &resourceCustomAccessor{
				replicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "existing"},
			},
			input: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "script1"},
			want:  "existing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.setReplicaResource(tt.input)
			assert.Equal(t, tt.want, tt.initial.GetReplicaResourceLuaScript())
		})
	}
}

func TestSetReplicaRevision(t *testing.T) {
	tests := []struct {
		name    string
		initial *resourceCustomAccessor
		input   *configv1alpha1.ReplicaRevision
		want    string
	}{
		{
			name:    "set on nil field",
			initial: &resourceCustomAccessor{},
			input:   &configv1alpha1.ReplicaRevision{LuaScript: "script1"},
			want:    "script1",
		},
		{
			name: "set on empty script",
			initial: &resourceCustomAccessor{
				replicaRevision: &configv1alpha1.ReplicaRevision{LuaScript: ""},
			},
			input: &configv1alpha1.ReplicaRevision{LuaScript: "script1"},
			want:  "script1",
		},
		{
			name: "set on non-empty script",
			initial: &resourceCustomAccessor{
				replicaRevision: &configv1alpha1.ReplicaRevision{LuaScript: "existing"},
			},
			input: &configv1alpha1.ReplicaRevision{LuaScript: "script1"},
			want:  "existing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.setReplicaRevision(tt.input)
			assert.Equal(t, tt.want, tt.initial.GetReplicaRevisionLuaScript())
		})
	}
}

func TestSetStatusReflection(t *testing.T) {
	tests := []struct {
		name    string
		initial *resourceCustomAccessor
		input   *configv1alpha1.StatusReflection
		want    string
	}{
		{
			name:    "set on nil field",
			initial: &resourceCustomAccessor{},
			input:   &configv1alpha1.StatusReflection{LuaScript: "script1"},
			want:    "script1",
		},
		{
			name: "set on empty script",
			initial: &resourceCustomAccessor{
				statusReflection: &configv1alpha1.StatusReflection{LuaScript: ""},
			},
			input: &configv1alpha1.StatusReflection{LuaScript: "script1"},
			want:  "script1",
		},
		{
			name: "set on non-empty script",
			initial: &resourceCustomAccessor{
				statusReflection: &configv1alpha1.StatusReflection{LuaScript: "existing"},
			},
			input: &configv1alpha1.StatusReflection{LuaScript: "script1"},
			want:  "existing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.setStatusReflection(tt.input)
			assert.Equal(t, tt.want, tt.initial.GetStatusReflectionLuaScript())
		})
	}
}

func TestSetStatusAggregation(t *testing.T) {
	tests := []struct {
		name    string
		initial *resourceCustomAccessor
		input   *configv1alpha1.StatusAggregation
		want    string
	}{
		{
			name:    "set on nil field",
			initial: &resourceCustomAccessor{},
			input:   &configv1alpha1.StatusAggregation{LuaScript: "script1"},
			want:    "script1",
		},
		{
			name: "set on empty script",
			initial: &resourceCustomAccessor{
				statusAggregation: &configv1alpha1.StatusAggregation{LuaScript: ""},
			},
			input: &configv1alpha1.StatusAggregation{LuaScript: "script1"},
			want:  "script1",
		},
		{
			name: "set on non-empty script",
			initial: &resourceCustomAccessor{
				statusAggregation: &configv1alpha1.StatusAggregation{LuaScript: "existing"},
			},
			input: &configv1alpha1.StatusAggregation{LuaScript: "script1"},
			want:  "existing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.setStatusAggregation(tt.input)
			assert.Equal(t, tt.want, tt.initial.GetStatusAggregationLuaScript())
		})
	}
}

func TestSetHealthInterpretation(t *testing.T) {
	tests := []struct {
		name    string
		initial *resourceCustomAccessor
		input   *configv1alpha1.HealthInterpretation
		want    string
	}{
		{
			name:    "set on nil field",
			initial: &resourceCustomAccessor{},
			input:   &configv1alpha1.HealthInterpretation{LuaScript: "script1"},
			want:    "script1",
		},
		{
			name: "set on empty script",
			initial: &resourceCustomAccessor{
				healthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: ""},
			},
			input: &configv1alpha1.HealthInterpretation{LuaScript: "script1"},
			want:  "script1",
		},
		{
			name: "set on non-empty script",
			initial: &resourceCustomAccessor{
				healthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "existing"},
			},
			input: &configv1alpha1.HealthInterpretation{LuaScript: "script1"},
			want:  "existing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.setHealthInterpretation(tt.input)
			assert.Equal(t, tt.want, tt.initial.GetHealthInterpretationLuaScript())
		})
	}
}

func TestAppendDependencyInterpretation(t *testing.T) {
	tests := []struct {
		name    string
		initial *resourceCustomAccessor
		input   *configv1alpha1.DependencyInterpretation
		want    []string
	}{
		{
			name:    "append to nil slice",
			initial: &resourceCustomAccessor{},
			input:   &configv1alpha1.DependencyInterpretation{LuaScript: "script1"},
			want:    []string{"script1"},
		},
		{
			name: "append to existing slice",
			initial: &resourceCustomAccessor{
				dependencyInterpretations: []*configv1alpha1.DependencyInterpretation{
					{LuaScript: "existing"},
				},
			},
			input: &configv1alpha1.DependencyInterpretation{LuaScript: "script1"},
			want:  []string{"existing", "script1"},
		},
		{
			name: "append empty script",
			initial: &resourceCustomAccessor{
				dependencyInterpretations: []*configv1alpha1.DependencyInterpretation{
					{LuaScript: "existing"},
				},
			},
			input: &configv1alpha1.DependencyInterpretation{LuaScript: ""},
			want:  []string{"existing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.appendDependencyInterpretation(tt.input)
			got := tt.initial.GetDependencyInterpretationLuaScripts()
			assert.Equal(t, tt.want, got)
		})
	}
}
