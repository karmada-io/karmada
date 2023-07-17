/*
Copyright 2015 The Kubernetes Authors.

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

// This code is directly lifted from the Kubernetes codebase in order to avoid relying on the k8s.io/kubernetes package.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/v1/helper/helpers_test.go

package lifted

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestIsNativeResource(t *testing.T) {
	testCases := []struct {
		resourceName corev1.ResourceName
		expectVal    bool
	}{
		{
			resourceName: "pod.alpha.kubernetes.io/opaque-int-resource-foo",
			expectVal:    true,
		},
		{
			resourceName: "kubernetes.io/resource-foo",
			expectVal:    true,
		},
		{
			resourceName: "foo",
			expectVal:    true,
		},
		{
			resourceName: "a/b",
			expectVal:    false,
		},
		{
			resourceName: "",
			expectVal:    true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("resourceName input=%s, expected value=%v", tc.resourceName, tc.expectVal), func(t *testing.T) {
			t.Parallel()
			v := IsNativeResource(tc.resourceName)
			if v != tc.expectVal {
				t.Errorf("Got %v but expected %v", v, tc.expectVal)
			}
		})
	}
}
