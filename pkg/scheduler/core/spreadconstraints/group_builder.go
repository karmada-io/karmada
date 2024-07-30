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
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"sort"
	"strings"
)

const annotationSuffix = "@annotation"

// compute calculates the maximum score and the total available replicas
// from the clusters within the group,
func (group *groupCluster) compute() {
	score := int64(0)
	availableReplicas := int32(0)
	for _, cluster := range group.Clusters {
		if cluster.Score > score {
			score = cluster.Score
		}
		availableReplicas += cluster.AvailableReplicas
	}
	group.MaxScore = score
	group.AvailableReplicas = availableReplicas
	//sort groups by maxScore
	sort.Slice(group.Groups, func(i, j int) bool {
		return group.Groups[i].MaxScore > group.Groups[j].MaxScore
	})
}

// createGroupCluster groups clusters based on scores and placement policies.
// It considers topology constraints if they are enabled in the placement.
func createGroupCluster(
	clusterScores framework.ClusterScoreList,
	placement *policyv1alpha1.Placement,
	spec *workv1alpha2.ResourceBindingSpec,
	computeAvailableReplicas AvailableReplicasFunc) *groupCluster {
	root := &groupCluster{}
	root.Name = ""
	root.Clusters = createClusters(clusterScores, spec, computeAvailableReplicas)
	born(root, placement.SpreadConstraints, 0)
	return root
}

// disableSpreadConstraint checks if the spread constraints should be ignored.
// It returns true if the replica division preference is 'static weighted'.
func disableSpreadConstraint(placement *policyv1alpha1.Placement) bool {
	strategy := placement.ReplicaScheduling

	// If the replica division preference is 'static weighted', ignore the declaration specified by spread constraints.
	if strategy != nil && strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided &&
		strategy.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceWeighted &&
		(strategy.WeightPreference == nil ||
			len(strategy.WeightPreference.StaticWeightList) != 0 && strategy.WeightPreference.DynamicWeight == "") {
		return true
	}
	return false
}

// disableAvailableResource checks if the available resource information should be ignored.
// It returns true if the replica division preference is 'Duplicated'.
func disableAvailableResource(placement *policyv1alpha1.Placement) bool {
	strategy := placement.ReplicaScheduling

	// If the replica division preference is 'Duplicated', ignore the information about cluster available resource.
	if strategy == nil || strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		return true
	}

	return false
}

// born recursively groups clusters based on the provided constraints.
// It updates the parent group with its child groups and their respective clusters.
// If the current group is at the last constraint, it marks it as a leaf node.
func born(parent *groupCluster, constraints []policyv1alpha1.SpreadConstraint, index int) {
	if len(constraints) == 0 {
		return
	}
	constraint := constraints[index]
	parent.MinGroups = constraint.MinGroups
	parent.MaxGroups = constraint.MaxGroups
	children := make(map[string]*groupCluster)
	for _, desc := range parent.Clusters {
		names := getGroup(desc.Cluster, constraint)
		if names != nil {
			for _, name := range names {
				if name != "" {
					child, ok := children[name]
					if !ok {
						child = &groupCluster{}
						child.Name = name
						children[name] = child
						parent.Groups = append(parent.Groups, child)
					}
					child.Clusters = append(child.Clusters, desc)
				}
			}
		}
	}
	maxIndex := len(constraints) - 1
	for _, child := range parent.Groups {
		if index < maxIndex {
			born(child, constraints, index+1)
		} else {
			child.Leaf = true
		}
	}
	parent.compute()
}

// getGroup returns a list of group identifiers for a given cluster based on the provided constraint.
func getGroup(cluster *clusterv1alpha1.Cluster, constraint policyv1alpha1.SpreadConstraint) []string {
	if constraint.SpreadByField != "" {
		return getGroupByField(cluster, constraint.SpreadByField)
	} else if constraint.SpreadByLabel != "" {
		return getGroupByLabel(cluster, constraint.SpreadByLabel)
	} else {
		return nil
	}
}

// getGroupByLabel returns a list of group identifiers for a given cluster based on the specified label key.
func getGroupByLabel(cluster *clusterv1alpha1.Cluster, key string) []string {
	if strings.HasSuffix(key, annotationSuffix) {
		return getGroupByAnnotation(cluster, key[:len(key)-len(annotationSuffix)])
	} else if label, ok := cluster.ObjectMeta.Labels[key]; ok {
		return []string{label}
	}
	return nil
}

// getGroupByAnnotation returns a list of group identifiers for a given cluster based on the specified annotation key.
func getGroupByAnnotation(cluster *clusterv1alpha1.Cluster, key string) []string {
	if label, ok := cluster.ObjectMeta.Annotations[key]; ok {
		return strings.Split(label, ",")
	}
	return nil
}

// getGroupByField returns a list of group identifiers for a given cluster based on the specified field value.
func getGroupByField(cluster *clusterv1alpha1.Cluster, value policyv1alpha1.SpreadFieldValue) []string {
	switch value {
	case policyv1alpha1.SpreadByFieldProvider:
		return []string{cluster.Spec.Provider}
	case policyv1alpha1.SpreadByFieldRegion:
		return []string{cluster.Spec.Region}
	case policyv1alpha1.SpreadByFieldZone:
		if cluster.Spec.Zones != nil {
			return cluster.Spec.Zones
		}
		return []string{cluster.Spec.Zone}
	case policyv1alpha1.SpreadByFieldCluster:
		return []string{cluster.Name}
	default:
		return nil
	}
}

// createClusters generates a list of clusterDesc based on the given cluster scores,
// resource binding specification, and available replicas computation function.
func createClusters(clusterScores framework.ClusterScoreList,
	spec *workv1alpha2.ResourceBindingSpec,
	computeAvailableReplicas AvailableReplicasFunc) []*clusterDesc {
	var clusters []*clusterv1alpha1.Cluster
	var descs []*clusterDesc

	for _, score := range clusterScores {
		desc := &clusterDesc{}
		desc.Name = score.Cluster.Name
		desc.Score = score.Score
		desc.Cluster = score.Cluster
		descs = append(descs, desc)
		clusters = append(clusters, score.Cluster)
	}

	clustersReplicas := computeAvailableReplicas(clusters, spec)
	for i, clustersReplica := range clustersReplicas {
		desc := descs[i]
		desc.AvailableReplicas = clustersReplica.Replicas
		desc.AvailableReplicas += spec.AssignedReplicasForCluster(clustersReplica.Name)
	}

	sort.Slice(descs, func(i, j int) bool {
		if descs[i].Score > descs[j].Score {
			return true
		} else if descs[i].Score < descs[j].Score {
			return false
		} else if descs[i].AvailableReplicas > descs[j].AvailableReplicas {
			return true
		} else if descs[i].AvailableReplicas < descs[j].AvailableReplicas {
			return false
		} else {
			return clusters[i].Name < clusters[j].Name
		}
	})
	return descs
}
