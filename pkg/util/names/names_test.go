package names

import (
	"fmt"
	"hash/fnv"
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
			args:    args{clusterName: "member-cluster-normal"},
			want:    "karmada-es-member-cluster-normal",
			wantErr: false,
		},
		{name: "empty member cluster name",
			args:    args{clusterName: ""},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateExecutionSpaceName(tt.args.clusterName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateExecutionSpaceName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
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
		if result := fmt.Sprintf("%s-%s", test.name, rand.SafeEncodeString(fmt.Sprint(hash.Sum32()))); result != got {
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

func TestGenerateEstimatorServiceName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		expected    string
	}{
		{
			name:        "",
			clusterName: "cluster",
			expected:    "karmada-scheduler-estimator-cluster",
		},
	}
	for _, test := range tests {
		got := GenerateEstimatorServiceName(test.clusterName)
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
			name:      "kube-",
			namespace: KubernetesReservedNSPrefix,
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
