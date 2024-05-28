/*
Copyright 2022 The Karmada Authors.

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
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

func TestSetDefaultSpreadConstraints(t *testing.T) {
	tests := []struct {
		name                     string
		spreadConstraint         []policyv1alpha1.SpreadConstraint
		expectedSpreadConstraint []policyv1alpha1.SpreadConstraint
	}{
		{
			name: "set spreadByField",
			spreadConstraint: []policyv1alpha1.SpreadConstraint{
				{
					MinGroups: 1,
				},
			},
			expectedSpreadConstraint: []policyv1alpha1.SpreadConstraint{
				{
					SpreadByField: policyv1alpha1.SpreadByFieldCluster,
					MinGroups:     1,
				},
			},
		},
		{
			name: "set minGroups",
			spreadConstraint: []policyv1alpha1.SpreadConstraint{
				{
					SpreadByField: policyv1alpha1.SpreadByFieldCluster,
				},
			},
			expectedSpreadConstraint: []policyv1alpha1.SpreadConstraint{
				{
					SpreadByField: policyv1alpha1.SpreadByFieldCluster,
					MinGroups:     1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaultSpreadConstraints(tt.spreadConstraint)
			if !reflect.DeepEqual(tt.spreadConstraint, tt.expectedSpreadConstraint) {
				t.Errorf("expected: %v, but got %v", tt.expectedSpreadConstraint, tt.spreadConstraint)
			}
		})
	}
}

func TestIsDependentOverridesPresent(t *testing.T) {
	tests := []struct {
		name          string
		policy        *policyv1alpha1.PropagationPolicy
		policyCreated bool
		expectedExist bool
	}{
		{
			name: "dependent override policy exist",
			policy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: policyv1alpha1.PropagationSpec{
					DependentOverrides: []string{"foo"},
				},
			},
			policyCreated: true,
			expectedExist: true,
		},
		{
			name: "dependent override policy do not exist",
			policy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: policyv1alpha1.PropagationSpec{
					DependentOverrides: []string{"foo"},
				},
			},
			policyCreated: false,
			expectedExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(Schema).Build()
			if tt.policyCreated {
				testOverridePolicy := &policyv1alpha1.OverridePolicy{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: tt.policy.Namespace,
						Name:      tt.policy.Name,
					},
				}
				err := fakeClient.Create(context.TODO(), testOverridePolicy)
				if err != nil {
					t.Fatalf("failed to create overridePolicy, err is: %v", err)
				}
			}
			res, err := IsDependentOverridesPresent(fakeClient, tt.policy)
			if !reflect.DeepEqual(res, tt.expectedExist) || err != nil {
				t.Errorf("expected %v, but got %v", tt.expectedExist, res)
			}
		})
	}
}

func TestIsDependentClusterOverridesPresent(t *testing.T) {
	tests := []struct {
		name          string
		policy        *policyv1alpha1.ClusterPropagationPolicy
		policyCreated bool
		expectedExist bool
	}{
		{
			name: "dependent cluster override policy exist",
			policy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: policyv1alpha1.PropagationSpec{
					DependentOverrides: []string{"foo"},
				},
			},
			policyCreated: true,
			expectedExist: true,
		},
		{
			name: "dependent cluster override policy do not exist",
			policy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: policyv1alpha1.PropagationSpec{
					DependentOverrides: []string{"foo"},
				},
			},
			policyCreated: false,
			expectedExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(Schema).Build()
			if tt.policyCreated {
				testClusterOverridePolicy := &policyv1alpha1.ClusterOverridePolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: tt.policy.Name,
					},
				}
				err := fakeClient.Create(context.TODO(), testClusterOverridePolicy)
				if err != nil {
					t.Fatalf("failed to create clusterOverridePolicy, err is: %v", err)
				}
			}
			res, err := IsDependentClusterOverridesPresent(fakeClient, tt.policy)
			if !reflect.DeepEqual(res, tt.expectedExist) || err != nil {
				t.Errorf("expected %v, but got %v", tt.expectedExist, res)
			}
		})
	}
}

func TestCheckMatchServiceImport(t *testing.T) {
	tests := []struct {
		name              string
		resourceSelectors []policyv1alpha1.ResourceSelector
		expected          bool
	}{
		{
			name: " get followed resource selector",
			resourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					Name: "foo1",
					Kind: util.ServiceImportKind,
				},
				{
					Name:      "foo2",
					Namespace: "bar",
					Kind:      util.ServiceKind,
				},
				{
					Name:       "foo3",
					Namespace:  "bar",
					Kind:       util.ServiceImportKind,
					APIVersion: "multicluster.x-k8s.io/v1alpha1",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ContainsServiceImport(tt.resourceSelectors)
			if !reflect.DeepEqual(res, tt.expected) {
				t.Errorf("expected %v, but got %v", tt.expected, res)
			}
		})
	}
}

func TestIsReplicaDynamicDivided(t *testing.T) {
	tests := []struct {
		name     string
		strategy *policyv1alpha1.ReplicaSchedulingStrategy
		expected bool
	}{
		{
			name:     "strategy empty",
			strategy: nil,
			expected: false,
		},
		{
			name: "strategy duplicated",
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			},
			expected: false,
		},
		{
			name: "strategy division preference weighted",
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				WeightPreference: &policyv1alpha1.ClusterPreferences{
					DynamicWeight: policyv1alpha1.DynamicWeightByAvailableReplicas,
				},
			},
			expected: true,
		},
		{
			name: "strategy division preference aggregated",
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := IsReplicaDynamicDivided(&policyv1alpha1.Placement{ReplicaScheduling: tt.strategy})
			if res != tt.expected {
				t.Errorf("expected %v, but got %v", tt.expected, res)
			}
		})
	}
}

func TestGetAppliedPlacement(t *testing.T) {
	tests := []struct {
		name              string
		annotations       map[string]string
		expectedPlacement *policyv1alpha1.Placement
		expectedErr       error
	}{
		{
			name: "policy placement annotation exist",
			annotations: map[string]string{
				util.PolicyPlacementAnnotation: "{\"clusterAffinity\":{\"clusterNames\":[\"member1\",\"member2\"]}}",
			},
			expectedPlacement: &policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"member1", "member2"},
				},
			},
			expectedErr: nil,
		},
		{
			name: "policy placement annotation do not exist",
			annotations: map[string]string{
				"foo": "bar",
			},
			expectedPlacement: nil,
			expectedErr:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := GetAppliedPlacement(tt.annotations)
			if !reflect.DeepEqual(res, tt.expectedPlacement) || err != tt.expectedErr {
				t.Errorf("expected %v and %v, but got %v and %v", tt.expectedPlacement, tt.expectedErr, res, err)
			}
		})
	}
}

func TestSetReplicaDivisionPreferenceWeighted(t *testing.T) {
	tests := []struct {
		name             string
		strategy         *policyv1alpha1.ReplicaSchedulingStrategy
		expectedWeighted bool
	}{
		{
			name:             "no replica scheduling strategy declared",
			expectedWeighted: false,
		},
		{
			name: "specified aggregated division preference",
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
			expectedWeighted: false,
		},
		{
			name: "unspecified replica division preference",
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
			},
			expectedWeighted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &policyv1alpha1.Placement{ReplicaScheduling: tt.strategy}
			SetReplicaDivisionPreferenceWeighted(p)
			if (p.ReplicaScheduling != nil &&
				p.ReplicaScheduling.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceWeighted) != tt.expectedWeighted {
				t.Errorf("expectedWeighted %v, but got %v", tt.expectedWeighted, !tt.expectedWeighted)
			}
		})
	}
}

func TestSetDefaultGracePeriodSeconds(t *testing.T) {
	tests := []struct {
		name           string
		behavior       *policyv1alpha1.ApplicationFailoverBehavior
		expectBehavior *policyv1alpha1.ApplicationFailoverBehavior
	}{
		{
			name: "purgeMode is not graciously",
			behavior: &policyv1alpha1.ApplicationFailoverBehavior{
				PurgeMode: policyv1alpha1.Never,
			},
			expectBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				PurgeMode: policyv1alpha1.Never,
			},
		},
		{
			name: "purgeMode is graciously and gracePeriodSeconds is set",
			behavior: &policyv1alpha1.ApplicationFailoverBehavior{
				PurgeMode:          policyv1alpha1.Graciously,
				GracePeriodSeconds: ptr.To[int32](200),
			},
			expectBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				PurgeMode:          policyv1alpha1.Graciously,
				GracePeriodSeconds: ptr.To[int32](200),
			},
		},
		{
			name: "purgeMode is graciously and gracePeriodSeconds is not set",
			behavior: &policyv1alpha1.ApplicationFailoverBehavior{
				PurgeMode: policyv1alpha1.Graciously,
			},
			expectBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				PurgeMode:          policyv1alpha1.Graciously,
				GracePeriodSeconds: ptr.To[int32](600),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaultGracePeriodSeconds(tt.behavior)
			if !reflect.DeepEqual(tt.behavior, tt.expectBehavior) {
				t.Errorf("expectedBehavior %v, but got %v", tt.expectBehavior, tt.behavior)
			}
		})
	}
}
