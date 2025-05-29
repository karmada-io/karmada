/*
Copyright 2021 The Karmada Authors.

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
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// NewPropagationPolicy will build a PropagationPolicy object.
func NewPropagationPolicy(ns, name string, rsSelectors []policyv1alpha1.ResourceSelector, placement policyv1alpha1.Placement) *policyv1alpha1.PropagationPolicy {
	return &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: rsSelectors,
			Placement:         placement,
		},
	}
}

// NewLazyPropagationPolicy will build a PropagationPolicy object with Lazy activation.
func NewLazyPropagationPolicy(ns, name string, rsSelectors []policyv1alpha1.ResourceSelector, placement policyv1alpha1.Placement) *policyv1alpha1.PropagationPolicy {
	return &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ActivationPreference: policyv1alpha1.LazyActivation,
			ResourceSelectors:    rsSelectors,
			Placement:            placement,
		},
	}
}

// NewExplicitPriorityPropagationPolicy will build a PropagationPolicy object with explicit priority.
func NewExplicitPriorityPropagationPolicy(ns, name string, rsSelectors []policyv1alpha1.ResourceSelector,
	placement policyv1alpha1.Placement, priority int32) *policyv1alpha1.PropagationPolicy {
	return &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: rsSelectors,
			Priority:          &priority,
			Placement:         placement,
		},
	}
}

// NewClusterPropagationPolicy will build a ClusterPropagationPolicy object.
func NewClusterPropagationPolicy(policyName string, rsSelectors []policyv1alpha1.ResourceSelector, placement policyv1alpha1.Placement) *policyv1alpha1.ClusterPropagationPolicy {
	return &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: rsSelectors,
			Placement:         placement,
		},
	}
}

// NewExplicitPriorityClusterPropagationPolicy will build a ClusterPropagationPolicy object with explicit priority.
func NewExplicitPriorityClusterPropagationPolicy(policyName string, rsSelectors []policyv1alpha1.ResourceSelector,
	placement policyv1alpha1.Placement, priority int32) *policyv1alpha1.ClusterPropagationPolicy {
	return &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: rsSelectors,
			Priority:          &priority,
			Placement:         placement,
		},
	}
}

// NewStaticWeightPolicyStrategy create static weight policy strategy with specific weights
// e.g: @clusters=[member1, member2], @weights=[1, 1], means static weight `member1:member2=1:1`
func NewStaticWeightPolicyStrategy(clusters []string, weights []int64) *policyv1alpha1.ReplicaSchedulingStrategy {
	gomega.Expect(len(clusters)).Should(gomega.Equal(len(weights)))
	staticWeightList := make([]policyv1alpha1.StaticClusterWeight, 0)
	for i, clusterName := range clusters {
		staticWeightList = append(staticWeightList, policyv1alpha1.StaticClusterWeight{
			TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{clusterName}},
			Weight:        weights[i],
		})
	}
	return &policyv1alpha1.ReplicaSchedulingStrategy{
		ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
		ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
		WeightPreference: &policyv1alpha1.ClusterPreferences{
			StaticWeightList: staticWeightList,
		},
	}
}

// NewOverridePolicy will build a OverridePolicy object.
func NewOverridePolicy(namespace, policyName string, rsSelectors []policyv1alpha1.ResourceSelector, clusterAffinity policyv1alpha1.ClusterAffinity, overriders policyv1alpha1.Overriders) *policyv1alpha1.OverridePolicy {
	return &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      policyName,
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: rsSelectors,
			TargetCluster:     &clusterAffinity,
			Overriders:        overriders,
		},
	}
}

// NewOverridePolicyByOverrideRules will build a OverridePolicy object by OverrideRules
func NewOverridePolicyByOverrideRules(namespace, policyName string, rsSelectors []policyv1alpha1.ResourceSelector, overrideRules []policyv1alpha1.RuleWithCluster) *policyv1alpha1.OverridePolicy {
	return &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      policyName,
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: rsSelectors,
			OverrideRules:     overrideRules,
		},
	}
}

// NewClusterOverridePolicyByOverrideRules will build a ClusterOverridePolicy object by OverrideRules
func NewClusterOverridePolicyByOverrideRules(policyName string, rsSelectors []policyv1alpha1.ResourceSelector, overrideRules []policyv1alpha1.RuleWithCluster) *policyv1alpha1.ClusterOverridePolicy {
	return &policyv1alpha1.ClusterOverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: rsSelectors,
			OverrideRules:     overrideRules,
		},
	}
}
