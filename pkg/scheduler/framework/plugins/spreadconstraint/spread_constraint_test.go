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

package spreadconstraint

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

func TestSpreadConstraint_Filter(t *testing.T) {
	tests := []struct {
		name           string
		bindingSpec    *workv1alpha2.ResourceBindingSpec
		cluster        *clusterv1alpha1.Cluster
		expectedCode   framework.Code
		expectedReason string
	}{
		{
			name: "no spread constraints",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{},
			},
			cluster:      &clusterv1alpha1.Cluster{},
			expectedCode: framework.Success,
		},
		{
			name: "spread by provider - provider present",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{SpreadByField: policyv1alpha1.SpreadByFieldProvider},
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					Provider: "aws",
				},
			},
			expectedCode: framework.Success,
		},
		{
			name: "spread by provider - provider missing",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{SpreadByField: policyv1alpha1.SpreadByFieldProvider},
					},
				},
			},
			cluster:        &clusterv1alpha1.Cluster{},
			expectedCode:   framework.Unschedulable,
			expectedReason: "cluster(s) did not have provider property",
		},
		{
			name: "spread by region - region present",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{SpreadByField: policyv1alpha1.SpreadByFieldRegion},
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					Region: "us-west-2",
				},
			},
			expectedCode: framework.Success,
		},
		{
			name: "spread by region - region missing",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{SpreadByField: policyv1alpha1.SpreadByFieldRegion},
					},
				},
			},
			cluster:        &clusterv1alpha1.Cluster{},
			expectedCode:   framework.Unschedulable,
			expectedReason: "cluster(s) did not have region property",
		},
		{
			name: "spread by zone - zones present",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{SpreadByField: policyv1alpha1.SpreadByFieldZone},
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					Zones: []string{"us-west-2a"},
				},
			},
			expectedCode: framework.Success,
		},
		{
			name: "spread by zone - zones missing",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{SpreadByField: policyv1alpha1.SpreadByFieldZone},
					},
				},
			},
			cluster:        &clusterv1alpha1.Cluster{},
			expectedCode:   framework.Unschedulable,
			expectedReason: "cluster(s) did not have zones property",
		},
	}

	p := &SpreadConstraint{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Filter(context.Background(), tt.bindingSpec, nil, tt.cluster)
			assert.Equal(t, tt.expectedCode, result.Code())
			if tt.expectedReason != "" {
				assert.Contains(t, result.AsError().Error(), tt.expectedReason)
			}
		})
	}
}

func TestNew(t *testing.T) {
	plugin, err := New()
	assert.NoError(t, err)
	assert.NotNil(t, plugin)
	_, ok := plugin.(*SpreadConstraint)
	assert.True(t, ok)
}

func TestSpreadConstraint_Name(t *testing.T) {
	p := &SpreadConstraint{}
	assert.Equal(t, Name, p.Name())
}
