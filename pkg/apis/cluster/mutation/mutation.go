package mutation

import (
	"math"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
)

const (
	// GB is a conversion value from GB to Bytes.
	GB = 1024 * 1024 * 1024
)

// MutateCluster mutates required fields of the Cluster.
func MutateCluster(cluster *clusterapis.Cluster) {
	MutateClusterTaints(cluster.Spec.Taints)
}

// MutateClusterTaints add TimeAdded field for cluster NoExecute taints only if TimeAdded not set.
func MutateClusterTaints(taints []corev1.Taint) {
	for i := range taints {
		if taints[i].Effect == corev1.TaintEffectNoExecute && taints[i].TimeAdded == nil {
			now := metav1.Now()
			taints[i].TimeAdded = &now
		}
	}
}

// SetDefaultClusterResourceModels set default cluster resource models for cluster.
func SetDefaultClusterResourceModels(cluster *clusterapis.Cluster) {
	if cluster.Spec.ResourceModels != nil {
		return
	}
	cluster.Spec.ResourceModels = []clusterapis.ResourceModel{
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
	}
}
