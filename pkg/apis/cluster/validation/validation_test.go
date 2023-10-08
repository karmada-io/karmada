package validation

import (
	"math"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
		"invalid provider": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: api.ClusterSpec{SyncMode: api.Push, Provider: "Invalid Provider"}},
			expectError: true,
		},
		"invalid region": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: api.ClusterSpec{SyncMode: api.Push, Region: "Invalid Region"}},
			expectError: true,
		},
		"invalid zone": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: api.ClusterSpec{SyncMode: api.Push, Zone: "Invalid Zone"}},
			expectError: true,
		},
		"invalid zones": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: api.ClusterSpec{SyncMode: api.Push, Zones: []string{"Invalid Zone", "Zone2"}}},
			expectError: true,
		},
		"co-exist zones and zone": {
			cluster:     api.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: api.ClusterSpec{SyncMode: api.Push, Zone: "Zone", Zones: []string{"Zones"}}},
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
		"invalid cluster resource models with the same grade": {
			cluster: api.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: api.ClusterSpec{
					ResourceModels: []api.ResourceModel{
						{
							Grade: 1,
							Ranges: []api.ResourceModelRange{
								{
									Name: api.ResourceCPU,
									Min:  *resource.NewQuantity(0, resource.DecimalSI),
									Max:  *resource.NewQuantity(2, resource.DecimalSI),
								},
							},
						},
						{
							Grade: 2,
							Ranges: []api.ResourceModelRange{
								{
									Name: api.ResourceCPU,
									Min:  *resource.NewQuantity(2, resource.DecimalSI),
									Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		"invalid cluster resource models with different numbers of resource types": {
			cluster: api.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: api.ClusterSpec{
					ResourceModels: []api.ResourceModel{
						{
							Grade: 1,
							Ranges: []api.ResourceModelRange{
								{
									Name: api.ResourceCPU,
									Min:  *resource.NewQuantity(0, resource.DecimalSI),
									Max:  *resource.NewQuantity(2, resource.DecimalSI),
								},
							},
						},
						{
							Grade: 2,
							Ranges: []api.ResourceModelRange{
								{
									Name: api.ResourceCPU,
									Min:  *resource.NewQuantity(2, resource.DecimalSI),
									Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
								},
								{
									Name: api.ResourceMemory,
									Min:  *resource.NewQuantity(2, resource.DecimalSI),
									Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		"invalid cluster resource models with unreasonable ranges": {
			cluster: api.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: api.ClusterSpec{
					ResourceModels: []api.ResourceModel{
						{
							Grade: 1,
							Ranges: []api.ResourceModelRange{
								{
									Name: api.ResourceCPU,
									Min:  *resource.NewQuantity(2, resource.DecimalSI),
									Max:  *resource.NewQuantity(0, resource.DecimalSI),
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		"invalid cluster resource models which the min value of each resource in the first model is not 0": {
			cluster: api.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: api.ClusterSpec{
					ResourceModels: []api.ResourceModel{
						{
							Grade: 1,
							Ranges: []api.ResourceModelRange{
								{
									Name: api.ResourceCPU,
									Min:  *resource.NewQuantity(1, resource.DecimalSI),
									Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		"invalid cluster resource models which the max value of each resource in the last model is not MaxInt64": {
			cluster: api.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: api.ClusterSpec{
					ResourceModels: []api.ResourceModel{
						{
							Grade: 1,
							Ranges: []api.ResourceModelRange{
								{
									Name: api.ResourceCPU,
									Min:  *resource.NewQuantity(0, resource.DecimalSI),
									Max:  *resource.NewQuantity(2, resource.DecimalSI),
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		"invalid cluster resource models which the resource types of each models are different": {
			cluster: api.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: api.ClusterSpec{
					ResourceModels: []api.ResourceModel{
						{
							Grade: 1,
							Ranges: []api.ResourceModelRange{
								{
									Name: api.ResourceCPU,
									Min:  *resource.NewQuantity(0, resource.DecimalSI),
									Max:  *resource.NewQuantity(2, resource.DecimalSI),
								},
							},
						},
						{
							Grade: 2,
							Ranges: []api.ResourceModelRange{
								{
									Name: api.ResourceMemory,
									Min:  *resource.NewQuantity(2, resource.DecimalSI),
									Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		"invalid cluster resource models with contiguous and non-overlapping": {
			cluster: api.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: api.ClusterSpec{
					ResourceModels: []api.ResourceModel{
						{
							Grade: 1,
							Ranges: []api.ResourceModelRange{
								{
									Name: api.ResourceCPU,
									Min:  *resource.NewQuantity(0, resource.DecimalSI),
									Max:  *resource.NewQuantity(2, resource.DecimalSI),
								},
							},
						},
						{
							Grade: 2,
							Ranges: []api.ResourceModelRange{
								{
									Name: api.ResourceCPU,
									Min:  *resource.NewQuantity(1, resource.DecimalSI),
									Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
								},
							},
						},
					},
				},
			},
			expectError: true,
		},
		"invalid cluster resource models with invalid resource name": {
			cluster: api.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: api.ClusterSpec{
					ResourceModels: []api.ResourceModel{
						{
							Grade: 1,
							Ranges: []api.ResourceModelRange{
								{
									Name: "test",
									Min:  *resource.NewQuantity(0, resource.DecimalSI),
									Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
								},
							},
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
