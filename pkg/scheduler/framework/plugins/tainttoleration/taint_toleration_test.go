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

package tainttoleration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

func TestTaintToleration_Filter(t *testing.T) {
	tests := []struct {
		name           string
		bindingSpec    *workv1alpha2.ResourceBindingSpec
		cluster        *clusterv1alpha1.Cluster
		expectedCode   framework.Code
		expectedReason string
	}{
		{
			name: "cluster already in target clusters",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{Name: "cluster1"},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
			},
			expectedCode: framework.Success,
		},
		{
			name: "no taints",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{},
			},
			cluster:      &clusterv1alpha1.Cluster{},
			expectedCode: framework.Success,
		},
		{
			name: "tolerated taint",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					ClusterTolerations: []corev1.Toleration{
						{
							Key:      "key1",
							Operator: corev1.TolerationOpEqual,
							Value:    "value1",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:    "key1",
							Value:  "value1",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
			expectedCode: framework.Success,
		},
		{
			name: "untolerated taint",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{},
			},
			cluster: &clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: []corev1.Taint{
						{
							Key:    "key1",
							Value:  "value1",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
			expectedCode:   framework.Unschedulable,
			expectedReason: "cluster(s) had untolerated taint {key1=value1:NoSchedule}",
		},
	}

	p := &TaintToleration{}

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
	_, ok := plugin.(*TaintToleration)
	assert.True(t, ok)
}

func TestTaintToleration_Name(t *testing.T) {
	p := &TaintToleration{}
	assert.Equal(t, Name, p.Name())
}
