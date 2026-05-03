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

func TestIsPrefixedNativeResource(t *testing.T) {
	cases := []struct {
		name corev1.ResourceName
		want bool
	}{
		{"kubernetes.io/foo", true},
		{"pod.alpha.kubernetes.io/opaque", true},
		{"example.com/gpu", false},
		{"foo", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsPrefixedNativeResource(c.name); got != c.want {
			t.Errorf("IsPrefixedNativeResource(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestIsHugePageResourceName(t *testing.T) {
	cases := []struct {
		name corev1.ResourceName
		want bool
	}{
		{corev1.ResourceName(string(corev1.ResourceHugePagesPrefix) + "2Mi"), true},
		{corev1.ResourceName(string(corev1.ResourceHugePagesPrefix) + "1Gi"), true},
		{"huge-pages-2Mi", false}, // missing the hyphenated `hugepages-` prefix
		{corev1.ResourceCPU, false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsHugePageResourceName(c.name); got != c.want {
			t.Errorf("IsHugePageResourceName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestIsAttachableVolumeResourceName(t *testing.T) {
	cases := []struct {
		name corev1.ResourceName
		want bool
	}{
		{corev1.ResourceName(string(corev1.ResourceAttachableVolumesPrefix) + "ebs"), true},
		{"attachable_volumes-foo", false}, // missing exact `attachable-volumes-` prefix
		{corev1.ResourceMemory, false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsAttachableVolumeResourceName(c.name); got != c.want {
			t.Errorf("IsAttachableVolumeResourceName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestIsExtendedResourceName(t *testing.T) {
	cases := []struct {
		name corev1.ResourceName
		want bool
	}{
		// Native resources are NOT extended.
		{corev1.ResourceCPU, false},
		{"foo", false},
		// Names prefixed with requests. are NOT extended (would clash with quota convention).
		{corev1.ResourceName(corev1.DefaultResourceRequestsPrefix + "example.com/gpu"), false},
		// Vendor-prefixed resource that satisfies IsQualifiedName once requests. is prepended IS extended.
		{"example.com/gpu", true},
		// Invalid qualified name → not extended.
		{"BAD!!", false},
	}
	for _, c := range cases {
		if got := IsExtendedResourceName(c.name); got != c.want {
			t.Errorf("IsExtendedResourceName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestIsScalarResourceName(t *testing.T) {
	cases := []struct {
		name corev1.ResourceName
		want bool
	}{
		{"example.com/gpu", true}, // extended
		{corev1.ResourceName(string(corev1.ResourceHugePagesPrefix) + "2Mi"), true},
		{corev1.ResourceName(string(corev1.ResourceAttachableVolumesPrefix) + "ebs"), true},
		{"kubernetes.io/foo", true}, // prefixed native
		{corev1.ResourceCPU, false},
		{"foo", false},
	}
	for _, c := range cases {
		if got := IsScalarResourceName(c.name); got != c.want {
			t.Errorf("IsScalarResourceName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}
