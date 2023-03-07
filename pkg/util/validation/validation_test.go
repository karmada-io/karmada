package validation

import (
	"strings"
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
		{
			name: "overrideSpec.targetCluster.fieldSelector has unsupported key",
			overrideSpec: policyv1alpha1.OverrideSpec{
				TargetCluster: &policyv1alpha1.ClusterAffinity{
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "foo",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"fooCloud"},
							}}}},
			},
			expectError: true,
		},
		{
			name: "overrideSpec.targetCluster.fieldSelector has unsupported operator",
			overrideSpec: policyv1alpha1.OverrideSpec{
				TargetCluster: &policyv1alpha1.ClusterAffinity{
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      util.ProviderField,
								Operator: corev1.NodeSelectorOpGt,
								Values:   []string{"fooCloud"},
							}}}},
			},
			expectError: true,
		},
		{
			name: "overrideRules.[index].targetCluster.fieldSelector has unsupported key",
			overrideSpec: policyv1alpha1.OverrideSpec{
				OverrideRules: []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							FieldSelector: &policyv1alpha1.FieldSelector{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "foo",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"fooCloud"},
									}}},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "overrideRules.[index].targetCluster.fieldSelector has unsupported operator",
			overrideSpec: policyv1alpha1.OverrideSpec{
				OverrideRules: []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							FieldSelector: &policyv1alpha1.FieldSelector{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      util.ProviderField,
										Operator: corev1.NodeSelectorOpGt,
										Values:   []string{"fooCloud"},
									}}},
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
			if got := emptyOverrides(tt.overriders); got != tt.want {
				t.Errorf("EmptyOverrides() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatePropagationSpec(t *testing.T) {
	tests := []struct {
		name        string
		spec        policyv1alpha1.PropagationSpec
		expectedErr string
	}{
		{
			name: "valid spec",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      util.ProviderField,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"fooCloud"},
								},
								{
									Key:      util.RegionField,
									Operator: corev1.NodeSelectorOpNotIn,
									Values:   []string{"fooCloud"},
								},
								{
									Key:      util.ZoneField,
									Operator: corev1.NodeSelectorOpNotIn,
									Values:   []string{"fooCloud"},
								},
							}}},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							MaxGroups:     2,
							MinGroups:     1,
							SpreadByField: policyv1alpha1.SpreadByFieldRegion,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						}}}},
			expectedErr: "",
		},
		{
			name: "clusterAffinity.fieldSelector has unsupported key",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "foo",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"fooCloud"},
								}}}}}},
			expectedErr: "unsupported key \"foo\", must be provider, region, or zone",
		},
		{
			name: "clusterAffinity.fieldSelector has unsupported operator",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						FieldSelector: &policyv1alpha1.FieldSelector{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      util.ProviderField,
									Operator: corev1.NodeSelectorOpExists,
									Values:   []string{"fooCloud"},
								}}}}}},
			expectedErr: "unsupported operator \"Exists\", must be In or NotIn",
		},
		{
			name: "clusterAffinities can not co-exist with clusterAffinity",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{"m1"},
					},
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName: "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{
								ClusterNames: []string{"m1"},
							}}}}},
			expectedErr: "clusterAffinities can not co-exist with clusterAffinity",
		},
		{
			name: "clusterAffinities different affinities have the same affinityName",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName: "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{
								ClusterNames: []string{"m1"},
							}},
						{
							AffinityName: "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{
								ClusterNames: []string{"m2"},
							}}}}},
			expectedErr: "each affinity term in a policy must have a unique name",
		},
		{
			name: "clusterAffinities.[index].clusterAffinity.fieldSelector has unsupported key",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName: "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{
								FieldSelector: &policyv1alpha1.FieldSelector{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "foo",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"fooCloud"},
										}}}}}}}},
			expectedErr: "unsupported key \"foo\", must be provider, region, or zone",
		},
		{
			name: "clusterAffinities.[index].clusterAffinity.fieldSelector has unsupported operator",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName: "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{
								FieldSelector: &policyv1alpha1.FieldSelector{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      util.ProviderField,
											Operator: corev1.NodeSelectorOpExists,
											Values:   []string{"fooCloud"},
										}}}}}}}},
			expectedErr: "unsupported operator \"Exists\", must be In or NotIn",
		},
		{
			name: "spreadConstraint spreadByLabel co-exist with spreadByField",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							SpreadByLabel: "foo",
						},
					},
				}},
			expectedErr: "spreadByLabel should not co-exist with spreadByField",
		},
		{
			name: "spreadConstraint maxGroups lower than minGroups",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							MaxGroups: 1,
							MinGroups: 2,
						},
					},
				}},
			expectedErr: "maxGroups lower than minGroups is not allowed",
		},
		{
			name: "spreadConstraint maxGroups lower than 0",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							MaxGroups: -1,
							MinGroups: 1,
						},
					},
				}},
			expectedErr: "maxGroups lower than 0 is not allowed",
		},
		{
			name: "spreadConstraint minGroups lower than 0",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							MaxGroups: 2,
							MinGroups: -2,
						},
					},
				}},
			expectedErr: "minGroups lower than 0 is not allowed",
		},
		{
			name: "spreadConstraint spreadByFieldCluster must be included if using spreadByField",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldRegion,
						},
					},
				}},
			expectedErr: "the cluster spread constraint must be enabled in one of the constraints in case of SpreadByField is enabled",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidatePropagationSpec(tt.spec)
			err := errs.ToAggregate()
			if err != nil {
				errStr := err.Error()
				if tt.expectedErr == "" {
					t.Errorf("expected no error:\n  but got:\n  %s", errStr)
				} else if !strings.Contains(errStr, tt.expectedErr) {
					t.Errorf("expected to contain:\n  %s\ngot:\n  %s", tt.expectedErr, errStr)
				}
			} else {
				if tt.expectedErr != "" {
					t.Errorf("unexpected no error, expected to contain:\n  %s", tt.expectedErr)
				}
			}
		})
	}
}
