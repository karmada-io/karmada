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

package validation

import (
	"archive/tar"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

func TestValidateOverrideSpec(t *testing.T) {
	var tests = []struct {
		name         string
		namespace    string
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
									Operator: policyv1alpha1.OverriderOpAdd,
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
									Operator: policyv1alpha1.OverriderOpAdd,
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
		{
			name: "fieldOverrider with neither YAML nor JSON should be rejected",
			overrideSpec: policyv1alpha1.OverrideSpec{
				OverrideRules: []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster-name"},
						},
						Overriders: policyv1alpha1.Overriders{
							FieldOverrider: []policyv1alpha1.FieldOverrider{
								{
									FieldPath: "/data/config",
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "fieldOverrider with both YAML and JSON set should be rejected",
			overrideSpec: policyv1alpha1.OverrideSpec{
				OverrideRules: []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster-name"},
						},
						Overriders: policyv1alpha1.Overriders{
							FieldOverrider: []policyv1alpha1.FieldOverrider{
								{
									FieldPath: "/data/config",
									JSON: []policyv1alpha1.JSONPatchOperation{
										{
											SubPath:  "/key",
											Operator: policyv1alpha1.OverriderOpRemove,
										},
									},
									YAML: []policyv1alpha1.YAMLPatchOperation{
										{
											SubPath:  "/key",
											Operator: policyv1alpha1.OverriderOpRemove,
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "fieldOverrider with only YAML set should be allowed",
			overrideSpec: policyv1alpha1.OverrideSpec{
				OverrideRules: []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster-name"},
						},
						Overriders: policyv1alpha1.Overriders{
							FieldOverrider: []policyv1alpha1.FieldOverrider{
								{
									FieldPath: "/data/config",
									YAML: []policyv1alpha1.YAMLPatchOperation{
										{
											SubPath:  "/key",
											Operator: policyv1alpha1.OverriderOpRemove,
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "fieldOverrider with only JSON set should be allowed",
			overrideSpec: policyv1alpha1.OverrideSpec{
				OverrideRules: []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster-name"},
						},
						Overriders: policyv1alpha1.Overriders{
							FieldOverrider: []policyv1alpha1.FieldOverrider{
								{
									FieldPath: "/data/config",
									JSON: []policyv1alpha1.JSONPatchOperation{
										{
											SubPath:  "/key",
											Operator: policyv1alpha1.OverriderOpRemove,
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		tc := test
		err := ValidateOverrideSpec(&tc.overrideSpec, tc.namespace)
		if err != nil && tc.expectError != true {
			t.Fatalf("expect no error but got: %v", err)
		}
		if err == nil && tc.expectError == true {
			t.Fatalf("expect an error but got none")
		}
	}
}

func TestValidateClusterTolerations(t *testing.T) {
	tests := []struct {
		name        string
		tolerations []corev1.Toleration
		expectedErr string
	}{
		{
			name:        "nil tolerations is valid",
			tolerations: nil,
			expectedErr: "",
		},
		{
			name: "toleration with Exists operator is valid",
			tolerations: []corev1.Toleration{
				{Key: "key1", Operator: corev1.TolerationOpExists},
			},
			expectedErr: "",
		},
		{
			name: "toleration with Equal operator is valid",
			tolerations: []corev1.Toleration{
				{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "value1"},
			},
			expectedErr: "",
		},
		{
			name: "toleration with empty operator is valid (defaults to Equal)",
			tolerations: []corev1.Toleration{
				{Key: "key1", Value: "value1"},
			},
			expectedErr: "",
		},
		{
			name: "toleration with Lt operator is invalid",
			tolerations: []corev1.Toleration{
				{Key: "key1", Operator: corev1.TolerationOpLt, Value: "100"},
			},
			expectedErr: "Unsupported value",
		},
		{
			name: "toleration with Gt operator is invalid",
			tolerations: []corev1.Toleration{
				{Key: "key1", Operator: corev1.TolerationOpGt, Value: "100"},
			},
			expectedErr: "Unsupported value",
		},
		{
			name: "mixed tolerations with one invalid Lt operator",
			tolerations: []corev1.Toleration{
				{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "value1"},
				{Key: "key2", Operator: corev1.TolerationOpLt, Value: "100"},
			},
			expectedErr: "Unsupported value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateClusterTolerations(tt.tolerations, field.NewPath("spec").Child("placement").Child("clusterTolerations"))
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
						Operator:  policyv1alpha1.OverriderOpRemove,
						Value:     "fictional.registry.us",
					},
				},
				CommandOverrider: []policyv1alpha1.CommandArgsOverrider{
					{
						ContainerName: "nginx",
						Operator:      policyv1alpha1.OverriderOpAdd,
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
		namespace   string
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
			name: "clusterAffinities cannot co-exist with clusterAffinity",
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
			expectedErr: "clusterAffinities cannot co-exist with clusterAffinity",
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
			name: "spreadConstraint has two cluster spread constraints",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     2,
							MinGroups:     -2,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     5,
							MinGroups:     3,
						},
						{
							SpreadByLabel: "grouped-by-net",
							MaxGroups:     5,
							MinGroups:     3,
						},
					},
				}},
			expectedErr: "multiple cluster spread constraints are not allowed",
		},
		{
			name: "spreadConstraint has multiple region spread constraints",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldRegion,
							MaxGroups:     1,
							MinGroups:     3,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldRegion,
							MaxGroups:     4,
							MinGroups:     2,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldRegion,
							MaxGroups:     6,
							MinGroups:     5,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     10,
							MinGroups:     5,
						},
					},
				}},
			expectedErr: "multiple region spread constraints are not allowed",
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
		{
			name: "resourceSelector name is empty when preemption is enabled",
			spec: policyv1alpha1.PropagationSpec{
				ResourceSelectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
					},
				},
				Preemption: policyv1alpha1.PreemptAlways,
			},
			expectedErr: "name cannot be empty if preemption is Always, the empty name may cause unexpected resources preemption",
		},
		{
			name: "suspension dispatching cannot co-exist with dispatchingOnClusters",
			spec: policyv1alpha1.PropagationSpec{
				Suspension: &policyv1alpha1.Suspension{
					Dispatching: ptr.To(true),
					DispatchingOnClusters: &policyv1alpha1.SuspendClusters{
						ClusterNames: []string{"cluster-name"},
					},
				},
			},
			expectedErr: "suspension dispatching cannot co-exist with dispatchingOnClusters.clusterNames",
		},
		{
			name: "suspension dispatching with nil dispatchingOnClusters is valid",
			spec: policyv1alpha1.PropagationSpec{
				Suspension: &policyv1alpha1.Suspension{
					Dispatching: ptr.To(true),
				},
			},
			expectedErr: "",
		},
		{
			name: "suspension dispatching with empty dispatching clusters is valid",
			spec: policyv1alpha1.PropagationSpec{
				Suspension: &policyv1alpha1.Suspension{
					Dispatching: ptr.To(true),
					DispatchingOnClusters: &policyv1alpha1.SuspendClusters{
						ClusterNames: []string{},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "dispatchingOnClusters.clusterNames without dispatching is valid",
			spec: policyv1alpha1.PropagationSpec{
				Suspension: &policyv1alpha1.Suspension{
					DispatchingOnClusters: &policyv1alpha1.SuspendClusters{
						ClusterNames: []string{"cluster-name"},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "dispatchingOnClusters.clusterNames with dispatching false is valid",
			spec: policyv1alpha1.PropagationSpec{
				Suspension: &policyv1alpha1.Suspension{
					Dispatching: ptr.To(false),
					DispatchingOnClusters: &policyv1alpha1.SuspendClusters{
						ClusterNames: []string{"cluster-name"},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "workloadAffinity affinity and antiAffinity with same groupByLabelKey",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						Affinity: &policyv1alpha1.WorkloadAffinityTerm{
							GroupByLabelKey: "app.group",
						},
						AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{
							GroupByLabelKey: "app.group",
						},
					},
				},
			},
			expectedErr: "affinity and antiAffinity must not use the same groupByLabelKey",
		},
		{
			name: "workloadAffinity affinity and antiAffinity with different groupByLabelKey is valid",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						Affinity: &policyv1alpha1.WorkloadAffinityTerm{
							GroupByLabelKey: "app.group",
						},
						AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{
							GroupByLabelKey: "app.region",
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "workloadAffinity with only affinity is valid",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						Affinity: &policyv1alpha1.WorkloadAffinityTerm{
							GroupByLabelKey: "app.group",
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "workloadAffinity with only antiAffinity is valid",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{
							GroupByLabelKey: "app.group",
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "workloadAffinity affinity with invalid groupByLabelKey",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						Affinity: &policyv1alpha1.WorkloadAffinityTerm{
							GroupByLabelKey: "INVALID KEY!",
						},
					},
				},
			},
			expectedErr: "spec.placement.workloadAffinity.affinity.groupByLabelKey",
		},
		{
			name: "workloadAffinity antiAffinity with invalid groupByLabelKey",
			spec: policyv1alpha1.PropagationSpec{
				Placement: policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{
							GroupByLabelKey: "INVALID KEY!",
						},
					},
				},
			},
			expectedErr: "spec.placement.workloadAffinity.antiAffinity.groupByLabelKey",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidatePropagationSpec(tt.spec, tt.namespace)
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

func TestValidateApplicationFailover(t *testing.T) {
	tests := []struct {
		name                        string
		applicationFailoverBehavior *policyv1alpha1.ApplicationFailoverBehavior
		expectedErr                 string
	}{
		{
			name:                        "application failover is nil",
			applicationFailoverBehavior: nil,
			expectedErr:                 "",
		},
		{
			name: "the tolerationSeconds is less than zero",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](-100),
				},
			},
			expectedErr: "spec.failover.application.decisionConditions.tolerationSeconds: Invalid value: -100: must be greater than or equal to 0",
		},
		{
			name: "the gracePeriodSeconds is declared when purgeMode is not gracefully",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				PurgeMode:          policyv1alpha1.PurgeModeDirectly,
				GracePeriodSeconds: ptr.To[int32](100),
			},
			expectedErr: "spec.failover.application.gracePeriodSeconds: Invalid value: 100: only takes effect when purgeMode is gracefully",
		},
		{
			name: "the gracePeriodSeconds is less than 0 when purgeMode is gracefully",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				PurgeMode:          policyv1alpha1.PurgeModeGracefully,
				GracePeriodSeconds: ptr.To[int32](-100),
			},
			expectedErr: "spec.failover.application.gracePeriodSeconds: Invalid value: -100: must be greater than 0",
		},
		{
			name: "the gracePeriodSeconds is empty when purgeMode is gracefully",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				PurgeMode: policyv1alpha1.PurgeModeGracefully,
			},
			expectedErr: "spec.failover.application.gracePeriodSeconds: Invalid value: null: should not be empty when purgeMode is gracefully",
		},
		{
			name: "application behavior is correctly declared",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
			},
			expectedErr: "",
		},
		{
			name: "statePreservation is nil",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				StatePreservation: nil,
			},
			expectedErr: "",
		},
		{
			name: "statePreservation with valid rules",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				StatePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{
							AliasLabelName: "valid-label-name",
							JSONPath:       "{.availableReplicas}",
						},
						{
							AliasLabelName: "example.com/state-key",
							JSONPath:       "{.readyReplicas}",
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "statePreservation with invalid aliasLabelName - starts with dash",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				StatePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{
							AliasLabelName: "-invalid-label",
							JSONPath:       "{.availableReplicas}",
						},
					},
				},
			},
			expectedErr: "spec.failover.application.statePreservation.rules[0].aliasLabelName: Invalid value: \"-invalid-label\": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')",
		},
		{
			name: "statePreservation with invalid aliasLabelName - ends with dash",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				StatePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{
							AliasLabelName: "invalid-label-",
							JSONPath:       "{.availableReplicas}",
						},
					},
				},
			},
			expectedErr: "spec.failover.application.statePreservation.rules[0].aliasLabelName: Invalid value: \"invalid-label-\": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')",
		},
		{
			name: "statePreservation with invalid aliasLabelName - contains invalid characters",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				StatePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{
							AliasLabelName: "invalid@label",
							JSONPath:       "{.availableReplicas}",
						},
					},
				},
			},
			expectedErr: "spec.failover.application.statePreservation.rules[0].aliasLabelName: Invalid value: \"invalid@label\": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')",
		},
		{
			name: "statePreservation with too long aliasLabelName",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				StatePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{
							AliasLabelName: strings.Repeat("a", 64),
							JSONPath:       "{.availableReplicas}",
						},
					},
				},
			},
			expectedErr: "spec.failover.application.statePreservation.rules[0].aliasLabelName: Invalid value: \"" + strings.Repeat("a", 64) + "\": name part must be no more than 63 bytes",
		},
		{
			name: "statePreservation with multiple invalid rules",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				StatePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{
							AliasLabelName: "-invalid1",
							JSONPath:       "{.availableReplicas}",
						},
						{
							AliasLabelName: "invalid2@",
							JSONPath:       "{.readyReplicas}",
						},
					},
				},
			},
			expectedErr: "[spec.failover.application.statePreservation.rules[0].aliasLabelName: Invalid value: \"-invalid1\": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.failover.application.statePreservation.rules[1].aliasLabelName: Invalid value: \"invalid2@\": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')]",
		},
		{
			name: "statePreservation with valid prefix in aliasLabelName",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				StatePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{
							AliasLabelName: "example.com/my-state",
							JSONPath:       "{.availableReplicas}",
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "statePreservation with invalid prefix in aliasLabelName",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				StatePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{
							AliasLabelName: "INVALID.COM/my-state",
							JSONPath:       "{.availableReplicas}",
						},
					},
				},
			},
			expectedErr: "spec.failover.application.statePreservation.rules[0].aliasLabelName: Invalid value: \"INVALID.COM/my-state\": prefix part a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateApplicationFailover(tt.applicationFailoverBehavior, field.NewPath("spec").Child("failover").Child("application"))
			err := errs.ToAggregate()
			if err != nil && err.Error() != tt.expectedErr {
				t.Errorf("expected error:\n  %s, but got:\n  %s", tt.expectedErr, err.Error())
			} else if err == nil && tt.expectedErr != "" {
				t.Errorf("expected error:\n  %s, but got no error\n", tt.expectedErr)
			}
		})
	}
}

func TestValidateClusterFailover(t *testing.T) {
	tests := []struct {
		name                    string
		clusterFailoverBehavior *policyv1alpha1.ClusterFailoverBehavior
		expectedErr             string
	}{
		{
			name:                    "cluster failover is nil",
			clusterFailoverBehavior: nil,
			expectedErr:             "",
		},
		{
			name: "cluster failover with valid statePreservation",
			clusterFailoverBehavior: &policyv1alpha1.ClusterFailoverBehavior{
				PurgeMode: policyv1alpha1.PurgeModeGracefully,
				StatePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{
							AliasLabelName: "cluster-state",
							JSONPath:       "{.status.nodeCount}",
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "cluster failover with invalid statePreservation",
			clusterFailoverBehavior: &policyv1alpha1.ClusterFailoverBehavior{
				PurgeMode: policyv1alpha1.PurgeModeDirectly,
				StatePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{
							AliasLabelName: "-invalid-cluster-label",
							JSONPath:       "{.status.nodeCount}",
						},
					},
				},
			},
			expectedErr: "spec.failover.cluster.statePreservation.rules[0].aliasLabelName: Invalid value: \"-invalid-cluster-label\": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')",
		},
		{
			name: "cluster failover without statePreservation",
			clusterFailoverBehavior: &policyv1alpha1.ClusterFailoverBehavior{
				PurgeMode: policyv1alpha1.PurgeModeGracefully,
			},
			expectedErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateClusterFailover(tt.clusterFailoverBehavior, field.NewPath("spec").Child("failover").Child("cluster"))
			err := errs.ToAggregate()
			if err != nil && err.Error() != tt.expectedErr {
				t.Errorf("expected error:\n  %s, but got:\n  %s", tt.expectedErr, err.Error())
			} else if err == nil && tt.expectedErr != "" {
				t.Errorf("expected error:\n  %s, but got no error\n", tt.expectedErr)
			}
		})
	}
}

func TestValidateCrdsTarBall(t *testing.T) {
	testItems := []struct {
		name        string
		header      *tar.Header
		expectedErr error
	}{
		{
			name: "unclean file dir 'crds/../'",
			header: &tar.Header{
				Name:     "crds/../",
				Typeflag: tar.TypeDir,
			},
			expectedErr: fmt.Errorf("the given file contains unclean file dir: %s", "crds/../"),
		},
		{
			name: "unclean file dir 'crds/..'",
			header: &tar.Header{
				Name:     "crds/..",
				Typeflag: tar.TypeDir,
			},
			expectedErr: fmt.Errorf("the given file contains unclean file dir: %s", "crds/.."),
		},
		{
			name: "unexpected file dir '../crds'",
			header: &tar.Header{
				Name:     "../crds",
				Typeflag: tar.TypeDir,
			},
			expectedErr: fmt.Errorf("the given file contains unexpected file dir: %s", "../crds"),
		},
		{
			name: "unexpected file dir '..'",
			header: &tar.Header{
				Name:     "..",
				Typeflag: tar.TypeDir,
			},
			expectedErr: fmt.Errorf("the given file contains unexpected file dir: %s", ".."),
		},
		{
			name: "expected file dir 'crds/'",
			header: &tar.Header{
				Name:     "crds/",
				Typeflag: tar.TypeDir,
			},
			expectedErr: nil,
		},
		{
			name: "expected file dir 'crds'",
			header: &tar.Header{
				Name:     "crds",
				Typeflag: tar.TypeDir,
			},
			expectedErr: nil,
		},
		{
			name: "unclean file path 'crds/../a.yaml'",
			header: &tar.Header{
				Name:     "crds/../a.yaml",
				Typeflag: tar.TypeReg,
			},
			expectedErr: fmt.Errorf("the given file contains unclean file path: %s", "crds/../a.yaml"),
		},
		{
			name: "unexpected file path '../crds/a.yaml'",
			header: &tar.Header{
				Name:     "../crds/a.yaml",
				Typeflag: tar.TypeReg,
			},
			expectedErr: fmt.Errorf("the given file contains unexpected file path: %s", "../crds/a.yaml"),
		},
		{
			name: "unexpected file path '../a.yaml'",
			header: &tar.Header{
				Name:     "../a.yaml",
				Typeflag: tar.TypeReg,
			},
			expectedErr: fmt.Errorf("the given file contains unexpected file path: %s", "../a.yaml"),
		},
		{
			name: "expected file path 'crds/a.yaml'",
			header: &tar.Header{
				Name:     "crds/a.yaml",
				Typeflag: tar.TypeReg,
			},
			expectedErr: nil,
		},
	}
	for _, item := range testItems {
		assert.Equal(t, item.expectedErr, ValidateCrdsTarBall(item.header))
	}
}

func TestValidateWorkloadAffinity(t *testing.T) {
	tests := []struct {
		name             string
		workloadAffinity *policyv1alpha1.WorkloadAffinity
		expectedErr      string
	}{
		{
			name:             "nil workloadAffinity is valid",
			workloadAffinity: nil,
			expectedErr:      "",
		},
		{
			name: "only affinity set is valid",
			workloadAffinity: &policyv1alpha1.WorkloadAffinity{
				Affinity: &policyv1alpha1.WorkloadAffinityTerm{
					GroupByLabelKey: "app.group",
				},
			},
			expectedErr: "",
		},
		{
			name: "only antiAffinity set is valid",
			workloadAffinity: &policyv1alpha1.WorkloadAffinity{
				AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{
					GroupByLabelKey: "app.group",
				},
			},
			expectedErr: "",
		},
		{
			name: "affinity and antiAffinity with different groupByLabelKey is valid",
			workloadAffinity: &policyv1alpha1.WorkloadAffinity{
				Affinity: &policyv1alpha1.WorkloadAffinityTerm{
					GroupByLabelKey: "app.group",
				},
				AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{
					GroupByLabelKey: "app.region",
				},
			},
			expectedErr: "",
		},
		{
			name: "affinity and antiAffinity with same groupByLabelKey is invalid",
			workloadAffinity: &policyv1alpha1.WorkloadAffinity{
				Affinity: &policyv1alpha1.WorkloadAffinityTerm{
					GroupByLabelKey: "app.group",
				},
				AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{
					GroupByLabelKey: "app.group",
				},
			},
			expectedErr: "affinity and antiAffinity must not use the same groupByLabelKey",
		},
		{
			name: "affinity with invalid groupByLabelKey is invalid",
			workloadAffinity: &policyv1alpha1.WorkloadAffinity{
				Affinity: &policyv1alpha1.WorkloadAffinityTerm{
					GroupByLabelKey: "INVALID KEY!",
				},
			},
			expectedErr: "spec.placement.workloadAffinity.affinity.groupByLabelKey",
		},
		{
			name: "antiAffinity with invalid groupByLabelKey is invalid",
			workloadAffinity: &policyv1alpha1.WorkloadAffinity{
				AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{
					GroupByLabelKey: "INVALID KEY!",
				},
			},
			expectedErr: "spec.placement.workloadAffinity.antiAffinity.groupByLabelKey",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateWorkloadAffinity(tt.workloadAffinity, field.NewPath("spec").Child("placement").Child("workloadAffinity"))
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

func TestValidatePlacement(t *testing.T) {
	tests := []struct {
		name               string
		placement          policyv1alpha1.Placement
		expectedErrCount   int
		expectedErrStrings []string
	}{
		{
			name: "clusterAffinity and clusterAffinities cannot co-exist",
			placement: policyv1alpha1.Placement{
				ClusterAffinity:   &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
				ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{{AffinityName: "group1"}},
			},
			expectedErrCount:   1,
			expectedErrStrings: []string{"clusterAffinities cannot co-exist with clusterAffinity"},
		},
		{
			name: "overflowAffinities rejected when replicaScheduling is nil",
			placement: policyv1alpha1.Placement{
				ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
					{
						AffinityName:    "primary",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
						OverflowAffinities: []policyv1alpha1.OverflowClusterAffinity{
							{AffinityName: "overflow-1", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}}},
						},
					},
				},
			},
			expectedErrCount:   1,
			expectedErrStrings: []string{"overflowAffinities can only be used together with dynamic weight or aggregated scheduling"},
		},
		{
			name: "overflowAffinities rejected with static weight scheduling",
			placement: policyv1alpha1.Placement{
				ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
					{
						AffinityName:    "primary",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
						OverflowAffinities: []policyv1alpha1.OverflowClusterAffinity{
							{AffinityName: "overflow-1", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}}},
						},
					},
				},
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						StaticWeightList: []policyv1alpha1.StaticClusterWeight{{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}}, Weight: 1}},
					},
				},
			},
			expectedErrCount:   1,
			expectedErrStrings: []string{"overflowAffinities can only be used together with dynamic weight or aggregated scheduling"},
		},
		{
			name: "overflowAffinities allowed with dynamic weight scheduling",
			placement: policyv1alpha1.Placement{
				ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
					{
						AffinityName:    "primary",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
						OverflowAffinities: []policyv1alpha1.OverflowClusterAffinity{
							{AffinityName: "overflow-1", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}}},
						},
					},
				},
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						DynamicWeight: policyv1alpha1.DynamicWeightByAvailableReplicas,
					},
				},
			},
			expectedErrCount: 0,
		},
		{
			name: "overflowAffinities allowed with aggregated scheduling",
			placement: policyv1alpha1.Placement{
				ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
					{
						AffinityName:    "primary",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
						OverflowAffinities: []policyv1alpha1.OverflowClusterAffinity{
							{AffinityName: "overflow-1", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}}},
						},
					},
				},
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
				},
			},
			expectedErrCount: 0,
		},
		{
			name: "multiple affinities, only one has overflow rejected",
			placement: policyv1alpha1.Placement{
				ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
					{
						AffinityName:    "group1",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
					},
					{
						AffinityName:    "group2",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}},
						OverflowAffinities: []policyv1alpha1.OverflowClusterAffinity{
							{AffinityName: "overflow-1", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member3"}}},
						},
					},
				},
			},
			expectedErrCount:   1,
			expectedErrStrings: []string{"overflowAffinities can only be used together with dynamic weight or aggregated scheduling"},
		},
		{
			name: "no overflow affinities with unsupported scheduling, no error",
			placement: policyv1alpha1.Placement{
				ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
					{
						AffinityName:    "primary",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
					},
				},
			},
			expectedErrCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidatePlacement(tt.placement, field.NewPath("spec").Child("placement"))
			assert.Len(t, errs, tt.expectedErrCount)
			if len(tt.expectedErrStrings) > 0 {
				if len(errs) == 0 {
					t.Fatalf("expected errors containing:\n  %v\nbut got no error", tt.expectedErrStrings)
				}
				errStr := errs.ToAggregate().Error()
				for _, expected := range tt.expectedErrStrings {
					assert.Contains(t, errStr, expected)
				}
			}
		})
	}
}

func TestValidateOverflowAffinities(t *testing.T) {
	tests := []struct {
		name               string
		affinity           policyv1alpha1.ClusterAffinityTerm
		expectedErrCount   int
		expectedErrStrings []string
	}{
		{
			name: "no overflow affinities, no error",
			affinity: policyv1alpha1.ClusterAffinityTerm{
				AffinityName:    "primary",
				ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
			},
			expectedErrCount: 0,
		},
		{
			name: "valid overflow affinities",
			affinity: policyv1alpha1.ClusterAffinityTerm{
				AffinityName:    "primary",
				ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
				OverflowAffinities: []policyv1alpha1.OverflowClusterAffinity{
					{AffinityName: "overflow-1", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}}},
					{AffinityName: "overflow-2", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member3"}}},
				},
			},
			expectedErrCount: 0,
		},
		{
			name: "overflow affinities without ClusterAffinity",
			affinity: policyv1alpha1.ClusterAffinityTerm{
				AffinityName: "primary",
				OverflowAffinities: []policyv1alpha1.OverflowClusterAffinity{
					{AffinityName: "overflow-1", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}}},
				},
			},
			expectedErrCount:   1,
			expectedErrStrings: []string{"overflowAffinities can only be used together with the inline ClusterAffinity"},
		},
		{
			name: "duplicate overflow affinity names",
			affinity: policyv1alpha1.ClusterAffinityTerm{
				AffinityName:    "primary",
				ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
				OverflowAffinities: []policyv1alpha1.OverflowClusterAffinity{
					{AffinityName: "overflow-1", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}}},
					{AffinityName: "overflow-1", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member3"}}},
				},
			},
			expectedErrCount:   1,
			expectedErrStrings: []string{"overflow affinity name must be unique and must not duplicate the primary group's affinityName"},
		},
		{
			name: "overflow affinity name duplicates primary group's affinityName",
			affinity: policyv1alpha1.ClusterAffinityTerm{
				AffinityName:    "primary",
				ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
				OverflowAffinities: []policyv1alpha1.OverflowClusterAffinity{
					{AffinityName: "primary", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}}},
				},
			},
			expectedErrCount:   1,
			expectedErrStrings: []string{"overflow affinity name must be unique and must not duplicate the primary group's affinityName"},
		},
		{
			name: "multiple errors: empty ClusterAffinity and duplicate names",
			affinity: policyv1alpha1.ClusterAffinityTerm{
				AffinityName: "primary",
				OverflowAffinities: []policyv1alpha1.OverflowClusterAffinity{
					{AffinityName: "overflow-1", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}}},
					{AffinityName: "overflow-1", ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member3"}}},
				},
			},
			expectedErrCount: 2,
			expectedErrStrings: []string{
				"overflowAffinities can only be used together with the inline ClusterAffinity",
				"overflow affinity name must be unique and must not duplicate the primary group's affinityName",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateOverflowAffinities(tt.affinity, field.NewPath("spec").Child("placement").Child("clusterAffinities").Index(0))
			assert.Len(t, errs, tt.expectedErrCount)
			if len(tt.expectedErrStrings) > 0 {
				if len(errs) == 0 {
					t.Fatalf("expected errors containing:\n  %v\nbut got no error", tt.expectedErrStrings)
				}
				errStr := errs.ToAggregate().Error()
				for _, expected := range tt.expectedErrStrings {
					assert.Contains(t, errStr, expected)
				}
			}
		})
	}
}
