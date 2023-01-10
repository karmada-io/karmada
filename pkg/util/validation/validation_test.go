package validation

import (
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

func TestValidateOverrideSpec(t *testing.T) {
	var tests = []struct {
		name         string
		overrideSpec policyv1alpha1.OverrideSpec
		expectError  bool
	}{
		{
			name: "overrideRules is set, overriders and targetCluster aren't set",
			overrideSpec: policyv1alpha1.OverrideSpec{
				OverrideRules: []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster-name"},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "overriders and targetCluster are set, overrideRules isn't set",
			overrideSpec: policyv1alpha1.OverrideSpec{
				TargetCluster: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"cluster-name"},
				},
				Overriders: policyv1alpha1.Overriders{
					Plaintext: []policyv1alpha1.PlaintextOverrider{
						{
							Path: "spec/image",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "overrideRules and targetCluster can't co-exist",
			overrideSpec: policyv1alpha1.OverrideSpec{
				OverrideRules: []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster-name"},
						},
					},
				},
				TargetCluster: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"cluster-name"},
				},
			},
			expectError: true,
		},
		{
			name: "overrideRules and overriders can't co-exist",
			overrideSpec: policyv1alpha1.OverrideSpec{
				OverrideRules: []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster-name"},
						},
					},
				},
				Overriders: policyv1alpha1.Overriders{
					Plaintext: []policyv1alpha1.PlaintextOverrider{
						{
							Path: "spec/image",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "overrideRules, targetCluster and overriders can't co-exist",
			overrideSpec: policyv1alpha1.OverrideSpec{
				OverrideRules: []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster-name"},
						},
					},
				},
				TargetCluster: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"cluster-name"},
				},
				Overriders: policyv1alpha1.Overriders{
					Plaintext: []policyv1alpha1.PlaintextOverrider{
						{
							Path: "spec/image",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid annotation should not be allowed",
			overrideSpec: policyv1alpha1.OverrideSpec{
				OverrideRules: []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster-name"},
						},
						Overriders: policyv1alpha1.Overriders{
							AnnotationsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
								{
									Operator: "add",
									Value:    map[string]string{"testannotation~projectId": "c-m-lfx9lk92p-v86cf"},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid label should not be allowed",
			overrideSpec: policyv1alpha1.OverrideSpec{
				OverrideRules: []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster-name"},
						},
						Overriders: policyv1alpha1.Overriders{
							LabelsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
								{
									Operator: "add",
									Value:    map[string]string{"testannotation~projectId": "c-m-lfx9lk92p-v86cf"},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		tc := test
		err := ValidateOverrideSpec(&tc.overrideSpec)
		if err != nil && tc.expectError != true {
			t.Fatalf("expect no error but got: %v", err)
		}
		if err == nil && tc.expectError == true {
			t.Fatalf("expect an error but got none")
		}
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

func TestValidatePolicyFieldSelector(t *testing.T) {
	fakeProvider := []string{"fooCloud"}
	fakeRegion := []string{"fooRegion"}
	fakeZone := []string{"fooZone"}

	tests := []struct {
		name          string
		filedSelector *policyv1alpha1.FieldSelector
		expectError   bool
	}{
		{
			name: "supported key",
			filedSelector: &policyv1alpha1.FieldSelector{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      util.ProviderField,
						Operator: corev1.NodeSelectorOpIn,
						Values:   fakeProvider,
					},
					{
						Key:      util.RegionField,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   fakeRegion,
					},
					{
						Key:      util.ZoneField,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   fakeZone,
					},
				},
			},
			expectError: false,
		},
		{
			name: "unsupported key",
			filedSelector: &policyv1alpha1.FieldSelector{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "foo",
						Operator: corev1.NodeSelectorOpIn,
						Values:   fakeProvider,
					},
					{
						Key:      util.RegionField,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   fakeRegion,
					},
				},
			},
			expectError: true,
		},
		{
			name: "supported operator",
			filedSelector: &policyv1alpha1.FieldSelector{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      util.ProviderField,
						Operator: corev1.NodeSelectorOpIn,
						Values:   fakeProvider,
					},
					{
						Key:      util.RegionField,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   fakeRegion,
					},
				},
			},
			expectError: false,
		},
		{
			name: "unsupported operator",
			filedSelector: &policyv1alpha1.FieldSelector{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      util.ProviderField,
						Operator: corev1.NodeSelectorOpExists,
						Values:   fakeProvider,
					},
					{
						Key:      util.RegionField,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   fakeRegion,
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePolicyFieldSelector(tt.filedSelector)
			if err != nil && tt.expectError != true {
				t.Fatalf("expect no error but got: %v", err)
			}
			if err == nil && tt.expectError == true {
				t.Fatalf("expect an error but got none")
			}
		})
	}
}

func TestEmptyOverrides(t *testing.T) {
	tests := []struct {
		name       string
		overriders policyv1alpha1.Overriders
		want       bool
	}{
		{
			name:       "empty overrides",
			overriders: policyv1alpha1.Overriders{},
			want:       true,
		},
		{
			name: "non-empty overrides",
			overriders: policyv1alpha1.Overriders{
				Plaintext: []policyv1alpha1.PlaintextOverrider{
					{
						Path: "spec/image",
					},
				},
			},
			want: false,
		},
		{
			name: "non-empty overrides",
			overriders: policyv1alpha1.Overriders{
				Plaintext: []policyv1alpha1.PlaintextOverrider{
					{
						Path: "spec/image",
					},
				},
				ImageOverrider: []policyv1alpha1.ImageOverrider{
					{
						Component: "Registry",
						Operator:  "remove",
						Value:     "fictional.registry.us",
					},
				},
				CommandOverrider: []policyv1alpha1.CommandArgsOverrider{
					{
						ContainerName: "nginx",
						Operator:      "add",
						Value:         []string{"echo 'hello karmada'"},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EmptyOverrides(tt.overriders); got != tt.want {
				t.Errorf("EmptyOverrides() = %v, want %v", got, tt.want)
			}
		})
	}
}
