package mutation

import (
	"math"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
)

func TestMutateCluster(t *testing.T) {
	type args struct {
		cluster *clusterapis.Cluster
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test mutate cluster",
			args: args{
				cluster: &clusterapis.Cluster{
					Spec: clusterapis.ClusterSpec{
						Taints: []corev1.Taint{
							{
								Key:    "foo",
								Value:  "abc",
								Effect: corev1.TaintEffectNoSchedule,
							},
							{
								Key:    "bar",
								Effect: corev1.TaintEffectNoExecute,
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MutateCluster(tt.args.cluster)
			for i := range tt.args.cluster.Spec.Taints {
				if tt.args.cluster.Spec.Taints[i].Effect == corev1.TaintEffectNoExecute && tt.args.cluster.Spec.Taints[i].TimeAdded == nil {
					t.Error("failed to mutate cluster, taints TimeAdded should not be nil")
				}
			}
		})
	}
}

func TestStandardizeClusterResourceModels(t *testing.T) {
	testCases := map[string]struct {
		models         []clusterapis.ResourceModel
		expectedModels []clusterapis.ResourceModel
	}{
		"sort models": {
			models: []clusterapis.ResourceModel{
				{
					Grade: 2,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(2, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(2, resource.DecimalSI),
						},
					},
				},
			},
			expectedModels: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(2, resource.DecimalSI),
						},
					},
				},
				{
					Grade: 2,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(2, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
			},
		},
		"start with 0": {
			models: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(1, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
			},
			expectedModels: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
			},
		},
		"end with MaxInt64": {
			models: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(2, resource.DecimalSI),
						},
					},
				},
			},
			expectedModels: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	for name, testCase := range testCases {
		StandardizeClusterResourceModels(testCase.models)
		if !reflect.DeepEqual(testCase.models, testCase.expectedModels) {
			t.Errorf("expected sorted resource models for %q, but it did not work", name)
			return
		}
	}
}

func TestSetDefaultClusterResourceModels(t *testing.T) {
	type args struct {
		cluster *clusterapis.Cluster
	}
	tests := []struct {
		name       string
		args       args
		wantModels []clusterapis.ResourceModel
	}{
		{
			name: "test set default Cluster",
			args: args{
				cluster: &clusterapis.Cluster{},
			},
			wantModels: []clusterapis.ResourceModel{
				{
					Grade: 0,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(1, resource.DecimalSI),
						},
						{
							Name: clusterapis.ResourceMemory,
							Min:  *resource.NewQuantity(0, resource.BinarySI),
							Max:  *resource.NewQuantity(4*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(1, resource.DecimalSI),
							Max:  *resource.NewQuantity(2, resource.DecimalSI),
						},
						{
							Name: clusterapis.ResourceMemory,
							Min:  *resource.NewQuantity(4*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(16*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 2,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(2, resource.DecimalSI),
							Max:  *resource.NewQuantity(4, resource.DecimalSI),
						},
						{
							Name: clusterapis.ResourceMemory,
							Min:  *resource.NewQuantity(16*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(32*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 3,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(4, resource.DecimalSI),
							Max:  *resource.NewQuantity(8, resource.DecimalSI),
						},
						{
							Name: clusterapis.ResourceMemory,
							Min:  *resource.NewQuantity(32*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(64*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 4,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(8, resource.DecimalSI),
							Max:  *resource.NewQuantity(16, resource.DecimalSI),
						},
						{
							Name: clusterapis.ResourceMemory,
							Min:  *resource.NewQuantity(64*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(128*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 5,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(16, resource.DecimalSI),
							Max:  *resource.NewQuantity(32, resource.DecimalSI),
						},
						{
							Name: clusterapis.ResourceMemory,
							Min:  *resource.NewQuantity(128*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(256*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 6,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(32, resource.DecimalSI),
							Max:  *resource.NewQuantity(64, resource.DecimalSI),
						},
						{
							Name: clusterapis.ResourceMemory,
							Min:  *resource.NewQuantity(256*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(512*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 7,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(64, resource.DecimalSI),
							Max:  *resource.NewQuantity(128, resource.DecimalSI),
						},
						{
							Name: clusterapis.ResourceMemory,
							Min:  *resource.NewQuantity(512*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(1024*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 8,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(128, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
						{
							Name: clusterapis.ResourceMemory,
							Min:  *resource.NewQuantity(1024*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.BinarySI),
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaultClusterResourceModels(tt.args.cluster)
		})
		if !reflect.DeepEqual(tt.args.cluster.Spec.ResourceModels, tt.wantModels) {
			t.Errorf("SetDefaultClusterResourceModels expected resourceModels %+v, bud get %+v", tt.wantModels, tt.args.cluster.Spec.ResourceModels)
			return
		}
	}
}
