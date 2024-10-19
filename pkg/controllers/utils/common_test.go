/*
Copyright 2020 The Karmada Authors.

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

package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestUpdateFailoverStatus(t *testing.T) {
	tests := []struct {
		name            string
		binding         *workv1alpha2.ResourceBinding
		supportExpected bool
	}{
		{
			name: "replicas divided amongst cluster, skip failover history",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
					Placement: &policyv1alpha1.Placement{
						ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
							ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
						},
						SpreadConstraints: []policyv1alpha1.SpreadConstraint{
							{MinGroups: 1, MaxGroups: 3, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
						},
					},
				},
			},
			supportExpected: false,
		},
		{
			name: "scheduling strategy undefined, skip failover history",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
					Placement: &policyv1alpha1.Placement{
						SpreadConstraints: []policyv1alpha1.SpreadConstraint{
							{MinGroups: 1, MaxGroups: 1, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
						},
					},
				},
			},
			supportExpected: false,
		},
		{
			name: "scheduling strategy divided and replicas constrained to single cluster, succeed",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
					Placement: &policyv1alpha1.Placement{
						ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
							ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
						},
						SpreadConstraints: []policyv1alpha1.SpreadConstraint{
							{MinGroups: 1, MaxGroups: 1, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
						},
					},
				},
			},
			supportExpected: true,
		},
		{
			name: "placement is empty, skip failover history",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "default",
				},
			},
			supportExpected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			supportActual := FailoverHistoryInfoIsSupported(tt.binding)
			if supportActual != tt.supportExpected {
				t.Errorf("Failed case, expected: %v, actual: %v", tt.supportExpected, supportActual)
			}
		})
	}
}
