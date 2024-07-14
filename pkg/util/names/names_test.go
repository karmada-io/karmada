/*
Copyright 2020 The Karmada Authors.

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

package names

import (
	"fmt"
	"hash/fnv"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/rand"

	hashutil "github.com/karmada-io/karmada/pkg/util/hash"
)

func TestGenerateExecutionSpaceName(t *testing.T) {
	type args struct {
		clusterName string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "normal cluster name",
			args: args{clusterName: "member-cluster-normal"},
			want: "karmada-es-member-cluster-normal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateExecutionSpaceName(tt.args.clusterName)
			if got != tt.want {
				t.Errorf("GenerateExecutionSpaceName() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetClusterName(t *testing.T) {
	type args struct {
		executionSpaceName string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "normal execution space name",
			args:    args{executionSpaceName: "karmada-es-member-cluster-normal"},
			want:    "member-cluster-normal",
			wantErr: false,
		},
		{name: "invalid member cluster",
			args:    args{executionSpaceName: "invalid"},
			want:    "",
			wantErr: true,
		},
		{name: "empty execution space name",
			args:    args{executionSpaceName: ""},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetClusterName(tt.args.executionSpaceName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetClusterName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetClusterName() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateBindingReferenceKey(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
	}{
		{
			name:      "mytest-deployment",
			namespace: "prod-xxx",
		},
		{
			name:      "mytest-deployment",
			namespace: "qa-xxx",
		},
		{
			name:      "mytest-deployment",
			namespace: "",
		},
		{
			name:      "b-c",
			namespace: "a",
		},
		{
			name:      "c",
			namespace: "a-b",
		},
	}
	result := map[string]struct{}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateBindingReferenceKey(tt.namespace, tt.name)
			if _, ok := result[got]; ok {
				t.Errorf("duplicate key found %v", got)
			}
			result[got] = struct{}{}
		})
	}
}

func TestGenerateBindingName(t *testing.T) {
	tests := []struct {
		testCase string
		kind     string
		name     string
		expect   string
	}{
		{
			testCase: "uppercase kind",
			kind:     "Pods",
			name:     "pod",
			expect:   "pod-pods",
		},
		{
			testCase: "uppercase name",
			kind:     "pods",
			name:     "Pod",
			expect:   "pod-pods",
		},
		{
			testCase: "lowercase name",
			kind:     "pods",
			name:     "Pod",
			expect:   "pod-pods",
		},
		// RBAC kind resource test with colon character in the name
		{
			testCase: "role kind resource",
			kind:     "Role",
			name:     "system:controller:tom",
			expect:   "system.controller.tom-role",
		},
		{
			testCase: "clusterRole kind resource",
			kind:     "ClusterRole",
			name:     "system::tom",
			expect:   "system..tom-clusterrole",
		},
		{
			testCase: "roleBinding kind resource",
			kind:     "RoleBinding",
			name:     "system::tt:tom",
			expect:   "system..tt.tom-rolebinding",
		},
		{
			testCase: "clusterRoleBinding kind resource",
			kind:     "ClusterRoleBinding",
			name:     "system:tt:tom",
			expect:   "system.tt.tom-clusterrolebinding",
		},
	}
	for _, test := range tests {
		if result := GenerateBindingName(test.kind, test.name); result != test.expect {
			t.Errorf("Test %s failed: expected %v, but got %v", test.testCase, test.expect, result)
		}
	}
}

func TestGenerateWorkName(t *testing.T) {
	tests := []struct {
		testCase  string
		kind      string
		name      string
		namespace string
		workname  string
	}{
		{
			testCase:  "empty namespace",
			kind:      "Pod",
			name:      "pod",
			namespace: "",
			workname:  "pod-pod",
		},
		{
			testCase:  "non nil namespace",
			kind:      "pods",
			name:      "Pod",
			namespace: "default",
			workname:  "default-pod-pods",
		},
	}

	for _, test := range tests {
		got := GenerateWorkName(test.kind, test.name, test.namespace)

		hash := fnv.New32a()
		hashutil.DeepHashObject(hash, test.workname)
		if result := fmt.Sprintf("%s-%s", strings.ToLower(test.name), rand.SafeEncodeString(fmt.Sprint(hash.Sum32()))); result != got {
			t.Errorf("Test %s failed: expected %v, but got %v", test.testCase, result, got)
		}
	}
}

func TestGenerateServiceAccountName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		expected    string
	}{
		{
			name:        "non empty clusterName",
			clusterName: "cluster1",
			expected:    "karmada-cluster1",
		},
	}
	for _, test := range tests {
		got := GenerateServiceAccountName(test.clusterName)
		if got != test.expected {
			t.Errorf("Test %s failed: expected %v, but got %v", test.name, test.expected, got)
		}
	}
}

func TestGenerateRoleName(t *testing.T) {
	tests := []struct {
		name               string
		serviceAccountName string
		expected           string
	}{
		{
			name:               "non empty serviceAccountName",
			serviceAccountName: "account",
			expected:           "karmada-controller-manager:account",
		},
	}
	for _, test := range tests {
		got := GenerateRoleName(test.serviceAccountName)
		if got != test.expected {
			t.Errorf("Test %s failed: expected %v, but got %v", test.name, test.expected, got)
		}
	}
}

func TestGenerateEndpointSliceName(t *testing.T) {
	tests := []struct {
		name              string
		endpointSliceName string
		cluster           string
		expected          string
	}{
		{
			name:              "",
			endpointSliceName: "endpoint",
			cluster:           "cluster",
			expected:          "imported-cluster-endpoint",
		},
	}
	for _, test := range tests {
		got := GenerateEndpointSliceName(test.endpointSliceName, test.cluster)
		if got != test.expected {
			t.Errorf("Test %s failed: expected %v, but got %v", test.name, test.expected, got)
		}
	}
}

func TestGenerateDerivedServiceName(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		expected    string
	}{
		{
			name:        "",
			serviceName: "service",
			expected:    "derived-service",
		},
	}
	for _, test := range tests {
		got := GenerateDerivedServiceName(test.serviceName)
		if got != test.expected {
			t.Errorf("Test %s failed: expected %v, but got %v", test.name, test.expected, got)
		}
	}
}

func TestGenerateEstimatorDeploymentName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		expected    string
	}{
		{
			name:        "generate estimator deployment name",
			clusterName: "cluster",
			expected:    "karmada-scheduler-estimator-cluster",
		},
	}
	for _, test := range tests {
		got := GenerateEstimatorDeploymentName(test.clusterName)
		if got != test.expected {
			t.Errorf("Test %s failed: expected %v, but got %v", test.name, test.expected, got)
		}
	}
}

func TestGenerateEstimatorServiceName(t *testing.T) {
	tests := []struct {
		name                   string
		clusterName            string
		estimatorServicePrefix string
		expected               string
	}{
		{
			name:                   "",
			clusterName:            "cluster",
			estimatorServicePrefix: "karmada-scheduler-estimator",
			expected:               "karmada-scheduler-estimator-cluster",
		},
		{
			name:                   "",
			clusterName:            "cluster",
			estimatorServicePrefix: "demo-karmada-scheduler-estimator",
			expected:               "demo-karmada-scheduler-estimator-cluster",
		},
	}
	for _, test := range tests {
		got := GenerateEstimatorServiceName(test.estimatorServicePrefix, test.clusterName)
		if got != test.expected {
			t.Errorf("Test %s failed: expected %v, but got %v", test.name, test.expected, got)
		}
	}
}

func TestIsReservedNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		expected  bool
	}{
		{
			name:      "karmada-system",
			namespace: NamespaceKarmadaSystem,
			expected:  true,
		},
		{
			name:      "karmada-cluster",
			namespace: NamespaceKarmadaCluster,
			expected:  true,
		},
		{
			name:      "karmada-es-",
			namespace: ExecutionSpacePrefix,
			expected:  true,
		},
		{
			name:      "not reserved namespace",
			namespace: "test-A",
			expected:  false,
		},
	}
	for _, test := range tests {
		got := IsReservedNamespace(test.namespace)
		if got != test.expected {
			t.Errorf("Test %s failed: expected %v, but got %v", test.name, test.expected, got)
		}
	}
}

func TestGenerateImpersonationSecretName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		expected    string
	}{
		{
			name:        "impersonator",
			clusterName: "clusterA",
			expected:    "clusterA-impersonator",
		},
	}
	for _, test := range tests {
		got := GenerateImpersonationSecretName(test.clusterName)
		if got != test.expected {
			t.Errorf("Test %s failed: expected %v, but got %v", test.name, test.expected, got)
		}
	}
}

func TestGeneratePolicyName(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		resourcename string
		gvk          string
		expected     string
	}{
		{
			name:         "generate policy name",
			namespace:    "ns-foo",
			resourcename: "foo",
			gvk:          "rand",
			expected:     "foo-b4978784",
		},
		{
			name:         "generate policy name with :",
			namespace:    "ns-foo",
			resourcename: "system:foo",
			gvk:          "rand",
			expected:     "system.foo-b4978784",
		},
	}
	for _, test := range tests {
		got := GeneratePolicyName(test.namespace, test.resourcename, test.gvk)
		if got != test.expected {
			t.Errorf("Test %s failed: expected %v, but got %v", test.name, test.expected, got)
		}
	}
}
