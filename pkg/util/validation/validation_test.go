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
			name: "the gracePeriodSeconds is declared when purgeMode is not graciously",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				PurgeMode:          policyv1alpha1.Immediately,
				GracePeriodSeconds: ptr.To[int32](100),
			},
			expectedErr: "spec.failover.application.gracePeriodSeconds: Invalid value: 100: only takes effect when purgeMode is graciously",
		},
		{
			name: "the gracePeriodSeconds is less than 0 when purgeMode is graciously",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				PurgeMode:          policyv1alpha1.Graciously,
				GracePeriodSeconds: ptr.To[int32](-100),
			},
			expectedErr: "spec.failover.application.gracePeriodSeconds: Invalid value: -100: must be greater than 0",
		},
		{
			name: "the gracePeriodSeconds is empty when purgeMode is graciously",
			applicationFailoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
				DecisionConditions: policyv1alpha1.DecisionConditions{
					TolerationSeconds: ptr.To[int32](100),
				},
				PurgeMode: policyv1alpha1.Graciously,
			},
			expectedErr: "spec.failover.application.gracePeriodSeconds: Invalid value: \"null\": should not be empty when purgeMode is graciously",
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
