/*
Copyright 2023 The Karmada Authors.

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

package util

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

// VerifyClustersExist verifies the clusters exist.
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
		return fmt.Errorf("clusters don't exist: %s", strings.Join(nonExistClusters, ","))
	}

	return nil
}
