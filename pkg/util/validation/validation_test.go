package validation

import (
	"testing"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
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
