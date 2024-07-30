/*
Copyright 2022 The Karmada Authors.

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

package spreadconstraint

import (
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// SelectBestClusters selects the cluster set based the spread constraint
func SelectBestClusters(clusterScores framework.ClusterScoreList,
	placement *policyv1alpha1.Placement,
	spec *workv1alpha2.ResourceBindingSpec,
	availableReplicasFunc func(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster,
) ([]*clusterv1alpha1.Cluster, error) {
	if len(placement.SpreadConstraints) == 0 || disableSpreadConstraint(placement) {
		var clusters []*clusterv1alpha1.Cluster
		for _, score := range clusterScores {
			clusters = append(clusters, score.Cluster)
		}
		klog.V(4).Infof("Select all clusters")
		return clusters, nil
	}

	replicas := spec.Replicas
	if disableAvailableResource(placement) {
		replicas = InvalidReplicas
	}

	group := createGroupCluster(clusterScores, placement, spec, availableReplicasFunc)
	return group.elect(replicas)
}
