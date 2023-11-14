package util

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

func VerifyClustersExist(input []string, clusters *clusterv1alpha1.ClusterList) error {
	clusterSet := sets.NewString()
	for _, cluster := range clusters.Items {
		clusterSet.Insert(cluster.Name)
	}

	var nonExistClusters []string
	for _, cluster := range input {
		if !clusterSet.Has(cluster) {
			nonExistClusters = append(nonExistClusters, cluster)
		}
	}
	if len(nonExistClusters) != 0 {
		return fmt.Errorf("clusters don't exist: " + strings.Join(nonExistClusters, ","))
	}

	return nil
}
