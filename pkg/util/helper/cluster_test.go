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
