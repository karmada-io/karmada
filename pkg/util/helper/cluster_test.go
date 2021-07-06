package helper

import (
	"testing"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

func TestIsAPIEnabled(t *testing.T) {
	clusterEnablements := []clusterv1alpha1.APIEnablement{
		{
			GroupVersion: "foo.example.io/v1beta1",
			Resources: []clusterv1alpha1.APIResource{
				{
					Kind: "FooA",
				},
				{
					Kind: "FooB",
				},
			},
		},
		{
			GroupVersion: "bar.example.io/v1beta1",
			Resources: []clusterv1alpha1.APIResource{
				{
					Kind: "BarA",
				},
				{
					Kind: "BarB",
				},
			},
		},
	}

	tests := []struct {
		name               string
		enablements        []clusterv1alpha1.APIEnablement
		targetGroupVersion string
		targetKind         string
		expect             bool
	}{
		{
			name:               "group version not enabled",
			enablements:        clusterEnablements,
			targetGroupVersion: "notexist",
			targetKind:         "Dummy",
			expect:             false,
		},
		{
			name:               "kind not enabled",
			enablements:        clusterEnablements,
			targetGroupVersion: "foo.example.io/v1beta1",
			targetKind:         "Dummy",
			expect:             false,
		},
		{
			name:               "enabled resource",
			enablements:        clusterEnablements,
			targetGroupVersion: "bar.example.io/v1beta1",
			targetKind:         "BarA",
			expect:             true,
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			result := IsAPIEnabled(tc.enablements, tc.targetGroupVersion, tc.targetKind)
			if result != tc.expect {
				t.Errorf("expected: %v, but got: %v", tc.expect, result)
			}
		})
	}
}
