/*
Copyright 2016 The Kubernetes Authors.

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

// This code is directly lifted from the Kubernetes codebase.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.26/staging/src/k8s.io/kubectl/pkg/cmd/taint/utils_test.go#L372-L533

package lifted

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/staging/src/k8s.io/kubectl/pkg/cmd/taint/utils_test.go#L372-L533

func TestParseTaints(t *testing.T) {
	cases := []struct {
		name                   string
		spec                   []string
		expectedTaints         []corev1.Taint
		expectedTaintsToRemove []corev1.Taint
		expectedErr            bool
	}{
		{
			name:        "invalid spec format",
			spec:        []string{""},
			expectedErr: true,
		},
		{
			name:        "invalid spec format",
			spec:        []string{"foo=abc"},
			expectedErr: true,
		},
		{
			name:        "invalid spec format",
			spec:        []string{"foo=abc=xyz:NoSchedule"},
			expectedErr: true,
		},
		{
			name:        "invalid spec format",
			spec:        []string{"foo=abc:xyz:NoSchedule"},
			expectedErr: true,
		},
		{
			name:        "invalid spec format for adding taint",
			spec:        []string{"foo"},
			expectedErr: true,
		},
		{
			name:        "invalid spec effect for adding taint",
			spec:        []string{"foo=abc:invalid_effect"},
			expectedErr: true,
		},
		{
			name:        "invalid spec effect for deleting taint",
			spec:        []string{"foo:invalid_effect-"},
			expectedErr: true,
		},
		{
			name: "add new taints",
			spec: []string{"foo=abc:NoSchedule", "bar=abc:NoSchedule", "baz:NoSchedule", "qux:NoSchedule", "foobar=:NoSchedule"},
			expectedTaints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "abc",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "bar",
					Value:  "abc",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "baz",
					Value:  "",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "qux",
					Value:  "",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "foobar",
					Value:  "",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectedErr: false,
		},
		{
			name: "delete taints",
			spec: []string{"foo:NoSchedule-", "bar:NoSchedule-", "qux=:NoSchedule-", "dedicated-"},
			expectedTaintsToRemove: []corev1.Taint{
				{
					Key:    "foo",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "qux",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key: "dedicated",
				},
			},
			expectedErr: false,
		},
		{
			name: "add taints and delete taints",
			spec: []string{"foo=abc:NoSchedule", "bar=abc:NoSchedule", "baz:NoSchedule", "qux:NoSchedule", "foobar=:NoSchedule", "foo:NoSchedule-", "bar:NoSchedule-", "baz=:NoSchedule-"},
			expectedTaints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "abc",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "bar",
					Value:  "abc",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "baz",
					Value:  "",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "qux",
					Value:  "",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "foobar",
					Value:  "",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectedTaintsToRemove: []corev1.Taint{
				{
					Key:    "foo",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "baz",
					Value:  "",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectedErr: false,
		},
	}

	for _, c := range cases {
		taints, taintsToRemove, err := ParseTaints(c.spec)
		if c.expectedErr && err == nil {
			t.Errorf("[%s] expected error for spec %s, but got nothing", c.name, c.spec)
		}
		if !c.expectedErr && err != nil {
			t.Errorf("[%s] expected no error for spec %s, but got: %v", c.name, c.spec, err)
		}
		if !reflect.DeepEqual(c.expectedTaints, taints) {
			t.Errorf("[%s] expected returen taints as %v, but got: %v", c.name, c.expectedTaints, taints)
		}
		if !reflect.DeepEqual(c.expectedTaintsToRemove, taintsToRemove) {
			t.Errorf("[%s] expected return taints to be removed as %v, but got: %v", c.name, c.expectedTaintsToRemove, taintsToRemove)
		}
	}
}
