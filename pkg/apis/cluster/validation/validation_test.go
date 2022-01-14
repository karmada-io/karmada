package validation

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/karmada-io/karmada/pkg/apis/cluster"
)

func TestValidateCluster(t *testing.T) {
	testCases := map[string]struct {
		cluster     api.Cluster
		expectError bool
	}{
		"zero-length name": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: ""}, Spec: api.ClusterSpec{SyncMode: api.Push}},
			expectError: true,
		},
		"invalid name": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "^Invalid"}, Spec: api.ClusterSpec{SyncMode: api.Push}},
			expectError: true,
		},
		"invalid name that is too long": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: strings.Repeat("a", 48+1)}, Spec: api.ClusterSpec{SyncMode: api.Push}},
			expectError: true,
		},
		"no sync mode": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: api.ClusterSpec{}},
			expectError: true,
		},
		"unsupported sync mode": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: api.ClusterSpec{SyncMode: api.ClusterSyncMode("^Invalid")}},
			expectError: true,
		},
		"invalid apiEndpoint": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: api.ClusterSpec{SyncMode: api.Push, APIEndpoint: "^Invalid"}},
			expectError: true,
		},
		"empty secretRef": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: api.ClusterSpec{SyncMode: api.Push, SecretRef: &api.LocalSecretReference{}}},
			expectError: true,
		},
		"empty impersonatorSecretRef": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: api.ClusterSpec{SyncMode: api.Push, ImpersonatorSecretRef: &api.LocalSecretReference{}}},
			expectError: true,
		},
		"invalid proxyURL": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: api.ClusterSpec{SyncMode: api.Push, ProxyURL: "^Invalid"}},
			expectError: true,
		},
		"unsupported taint effect": {
			cluster: api.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: api.ClusterSpec{
					SyncMode: api.Push,
					Taints: []corev1.Taint{
						{
							Key:    "foo",
							Value:  "bar",
							Effect: corev1.TaintEffect("^Invalid"),
						},
					},
				},
			},
			expectError: true,
		},
	}

	for name, testCase := range testCases {
		errs := ValidateCluster(&testCase.cluster)
		if len(errs) == 0 && testCase.expectError {
			t.Errorf("expected failure for %q, but there were none", name)
			return
		}
		if len(errs) != 0 && !testCase.expectError {
			t.Errorf("expected success for %q, but there were errors: %v", name, errs)
			return
		}
	}
}
