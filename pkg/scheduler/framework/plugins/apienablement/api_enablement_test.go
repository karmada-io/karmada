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

package apienablement

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

func TestAPIEnablement_Filter(t *testing.T) {
	tests := []struct {
		name         string
		bindingSpec  *workv1alpha2.ResourceBindingSpec
		cluster      *clusterv1alpha1.Cluster
		expectedCode framework.Code
		expectError  bool
	}{
		{
			name: "API is enabled in cluster",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
				Status: clusterv1alpha1.ClusterStatus{
					APIEnablements: []clusterv1alpha1.APIEnablement{
						{
							GroupVersion: "apps/v1",
							Resources: []clusterv1alpha1.APIResource{
								{
									Kind: "Deployment",
								},
							},
						},
					},
				},
			},
			expectedCode: framework.Success,
			expectError:  false,
		},
		{
			name: "API is not enabled in cluster",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{
					APIVersion: "custom.io/v1",
					Kind:       "CustomResource",
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
				Status: clusterv1alpha1.ClusterStatus{
					APIEnablements: []clusterv1alpha1.APIEnablement{
						{
							GroupVersion: "apps/v1",
							Resources: []clusterv1alpha1.APIResource{
								{
									Kind: "Deployment",
								},
							},
						},
					},
				},
			},
			expectedCode: framework.Unschedulable,
			expectError:  true,
		},
		{
			name: "cluster in target list with API not enabled",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{
					APIVersion: "custom.io/v1",
					Kind:       "CustomResource",
				},
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name: "cluster1",
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
				Status: clusterv1alpha1.ClusterStatus{
					APIEnablements: []clusterv1alpha1.APIEnablement{
						{
							GroupVersion: "apps/v1",
							Resources: []clusterv1alpha1.APIResource{
								{
									Kind: "Deployment",
								},
							},
						},
					},
				},
			},
			expectedCode: framework.Success,
			expectError:  false,
		},
	}

	p := &APIEnablement{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Filter(context.Background(), tt.bindingSpec, nil, tt.cluster)
			assert.Equal(t, tt.expectedCode, result.Code())
			assert.Equal(t, tt.expectError, result.AsError() != nil)
		})
	}
}

func TestNew(t *testing.T) {
	plugin, err := New()
	assert.NoError(t, err)
	assert.NotNil(t, plugin)
	_, ok := plugin.(*APIEnablement)
	assert.True(t, ok)
}

func TestAPIEnablement_Name(t *testing.T) {
	p := &APIEnablement{}
	assert.Equal(t, Name, p.Name())
}
