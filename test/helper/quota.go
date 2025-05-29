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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// NewFederatedResourceQuota will build a demo FederatedResourceQuota object.
func NewFederatedResourceQuota(ns, name string, clusterNames []string) *policyv1alpha1.FederatedResourceQuota {
	var staticAssignments []policyv1alpha1.StaticClusterAssignment
	for i := range clusterNames {
		staticAssignments = append(staticAssignments, policyv1alpha1.StaticClusterAssignment{
			ClusterName: clusterNames[i],
			Hard: map[corev1.ResourceName]resource.Quantity{
				"cpu":    resource.MustParse("1"),
				"memory": resource.MustParse("2Gi"),
			},
		})
	}
	return &policyv1alpha1.FederatedResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: policyv1alpha1.FederatedResourceQuotaSpec{
			Overall: map[corev1.ResourceName]resource.Quantity{
				"cpu":    resource.MustParse("8"),
				"memory": resource.MustParse("16Gi"),
			},
			StaticAssignments: staticAssignments,
		},
	}
}

// NewFederatedResourceQuotaWithOverall will build a FederatedResourceQuota object with specified overall resource limits.
func NewFederatedResourceQuotaWithOverall(ns, name string, overall corev1.ResourceList) *policyv1alpha1.FederatedResourceQuota {
	return &policyv1alpha1.FederatedResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: policyv1alpha1.FederatedResourceQuotaSpec{
			Overall: overall,
		},
	}
}
