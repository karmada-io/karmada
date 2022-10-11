package helper

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestValidateSpreadConstraint(t *testing.T) {
	tests := []struct {
		name             string
		spreadConstraint []policyv1alpha1.SpreadConstraint
		expectedErr      error
	}{
		{
			name: "spreadByLabel co-exist with spreadByField",
			spreadConstraint: []policyv1alpha1.SpreadConstraint{
				{
					SpreadByField: policyv1alpha1.SpreadByFieldCluster,
					SpreadByLabel: "foo",
				},
			},
			expectedErr: fmt.Errorf("invalid constraints: SpreadByLabel(foo) should not co-exist with spreadByField(cluster)"),
		},
		{
			name: "maxGroups lower than minGroups",
			spreadConstraint: []policyv1alpha1.SpreadConstraint{
				{
					MaxGroups: 1,
					MinGroups: 2,
				},
			},
			expectedErr: fmt.Errorf("maxGroups(1) lower than minGroups(2) is not allowed"),
		},
		{
			name: "spreadByFieldCluster must be included if using spreadByField",
			spreadConstraint: []policyv1alpha1.SpreadConstraint{
				{
					SpreadByField: policyv1alpha1.SpreadByFieldRegion,
				},
			},
			expectedErr: fmt.Errorf("the cluster spread constraint must be enabled in one of the constraints in case of SpreadByField is enabled"),
		},
		{
			name: "validate success",
			spreadConstraint: []policyv1alpha1.SpreadConstraint{
				{
					MaxGroups:     2,
					MinGroups:     1,
					SpreadByField: policyv1alpha1.SpreadByFieldRegion,
				},
				{
					SpreadByField: policyv1alpha1.SpreadByFieldCluster,
				},
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpreadConstraint(tt.spreadConstraint)
			if !reflect.DeepEqual(err, tt.expectedErr) {
				t.Errorf("expected: %v, but got %v", tt.expectedErr, err)
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

func TestGetFollowedResourceSelectorsWhenMatchServiceImport(t *testing.T) {
	tests := []struct {
		name              string
		resourceSelectors []policyv1alpha1.ResourceSelector
		expected          []policyv1alpha1.ResourceSelector
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
					Name:      "foo3",
					Namespace: "bar",
					Kind:      util.ServiceImportKind,
				},
			},
			expected: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: "v1",
					Kind:       util.ServiceKind,
					Namespace:  "bar",
					Name:       "derived-foo3",
				},
				{
					APIVersion: "discovery.k8s.io/v1",
					Kind:       util.EndpointSliceKind,
					Namespace:  "bar",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							discoveryv1.LabelServiceName: "derived-foo3",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := GetFollowedResourceSelectorsWhenMatchServiceImport(tt.resourceSelectors)
			if !reflect.DeepEqual(res, tt.expected) {
				t.Errorf("expected %v, but got %v", tt.expected, res)
			}
		})
	}
}

func TestGenerateResourceSelectorForServiceImport(t *testing.T) {
	tests := []struct {
		name      string
		svcImport policyv1alpha1.ResourceSelector
		expected  []policyv1alpha1.ResourceSelector
	}{
		{
			name: "generate resource selector",
			svcImport: policyv1alpha1.ResourceSelector{
				Name:      "foo",
				Namespace: "bar",
			},
			expected: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: "v1",
					Kind:       util.ServiceKind,
					Namespace:  "bar",
					Name:       "derived-foo",
				},
				{
					APIVersion: "discovery.k8s.io/v1",
					Kind:       util.EndpointSliceKind,
					Namespace:  "bar",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							discoveryv1.LabelServiceName: "derived-foo",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := GenerateResourceSelectorForServiceImport(tt.svcImport)
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
			res := IsReplicaDynamicDivided(tt.strategy)
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
