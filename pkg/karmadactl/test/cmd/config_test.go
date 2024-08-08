/*
Copyright 2024 The Karmada Authors.

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

package karmadactl

import (
	"fmt"
	"strings"
	"testing"
)

func TestCmdConfigImagesList(t *testing.T) {
	var tests = []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name: "list images without any flags",
			args: []string{"config", "image", "list"},
			expected: `registry.k8s.io/kube-apiserver:v1.29.6
registry.k8s.io/kube-controller-manager:v1.29.6
registry.k8s.io/etcd:3.5.13-0
docker.io/alpine:3.19.1
docker.io/karmada/karmada-scheduler:v1.11.0-alpha.0
docker.io/karmada/karmada-controller-manager:v1.11.0-alpha.0
docker.io/karmada/karmada-webhook:v1.11.0-alpha.0
docker.io/karmada/karmada-aggregated-apiserver:v1.11.0-alpha.0`,
		},
		{
			name: "list images with private-image-registry flag",
			args: []string{"config", "image", "list", "--private-image-registry=myregistry.com"},
			expected: `myregistry.com/kube-apiserver:v1.29.6
myregistry.com/kube-controller-manager:v1.29.6
myregistry.com/etcd:3.5.13-0
myregistry.com/alpine:3.19.1
myregistry.com/karmada-scheduler:v1.11.0-alpha.0
myregistry.com/karmada-controller-manager:v1.11.0-alpha.0
myregistry.com/karmada-webhook:v1.11.0-alpha.0
myregistry.com/karmada-aggregated-apiserver:v1.11.0-alpha.0`,
		},
		{
			name: "list images with kube-image-country flag",
			args: []string{"config", "image", "list", "--kube-image-country=cn"},
			expected: `registry.cn-hangzhou.aliyuncs.com/google_containers/kube-controller-manager:v1.29.6
registry.cn-hangzhou.aliyuncs.com/google_containers/etcd:3.5.13-0
docker.io/alpine:3.19.1
docker.io/karmada/karmada-scheduler:v1.11.0-alpha.0
docker.io/karmada/karmada-controller-manager:v1.11.0-alpha.0
docker.io/karmada/karmada-webhook:v1.11.0-alpha.0
docker.io/karmada/karmada-aggregated-apiserver:v1.11.0-alpha.0`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, _, err := runKarmadactlCmd(t, tt.args...)
			if err != nil {
				t.Fatalf("failed to run 'karmadactl %s', err: %v", strings.Join(tt.args, " "), err)
			}
			output := stdout + stderr
			if !strings.Contains(output, tt.expected) {
				fmt.Println(output)
				fmt.Println(stdout)
				fmt.Println(stderr)
				fmt.Println(tt.expected)
				t.Errorf("failed %s with output: %s, expected: %s", tt.name, output, tt.expected)
			}
		})
	}
}

func TestCmdConfigImagesListWithInvalidFlags(t *testing.T) {
	var tests = []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "invalid flag --unknown-flag",
			args:     []string{"config", "image", "list", "--unknown-flag"},
			expected: "unknown flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr, _, err := runKarmadactlCmd(t, tt.args...)
			if err == nil {
				t.Fatalf("expected an error but got none for 'karmadactl %s'", strings.Join(tt.args, " "))
			}
			if !strings.Contains(stderr, tt.expected) {
				t.Errorf("failed %s with stderr: %s, expected: %s", tt.name, stderr, tt.expected)
			}
		})
	}
}
