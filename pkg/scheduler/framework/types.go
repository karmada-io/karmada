/*
Copyright The Karmada Authors.

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

package framework

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

// ClusterInfo is cluster level aggregated information.
type ClusterInfo struct {
	// Overall cluster information.
	cluster *clusterv1alpha1.Cluster
}

// NewClusterInfo creates a ClusterInfo object.
func NewClusterInfo(cluster *clusterv1alpha1.Cluster) *ClusterInfo {
	return &ClusterInfo{
		cluster: cluster,
	}
}

// Cluster returns overall information about this cluster.
func (n *ClusterInfo) Cluster() *clusterv1alpha1.Cluster {
	if n == nil {
		return nil
	}
	return n.cluster
}
