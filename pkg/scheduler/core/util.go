/*
Copyright 2021 The Karmada Authors.

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

package core

import (
	"context"
	"fmt"
	"math"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

type calculator func([]*clusterv1alpha1.Cluster, *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster

func getDefaultWeightPreference(clusters []*clusterv1alpha1.Cluster) *policyv1alpha1.ClusterPreferences {
	staticWeightLists := make([]policyv1alpha1.StaticClusterWeight, 0)
	for _, cluster := range clusters {
		staticWeightList := policyv1alpha1.StaticClusterWeight{
			TargetCluster: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
			},
			Weight: 1,
		}
		staticWeightLists = append(staticWeightLists, staticWeightList)
	}

	return &policyv1alpha1.ClusterPreferences{
		StaticWeightList: staticWeightLists,
	}
}

func calAvailableReplicas(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster {
	// availableTargetClusters stores the result of estimated replicas for each clusters
	availableTargetClusters := make([]workv1alpha2.TargetCluster, len(clusters))
	// clusterIndexToEstimatorPriority key refers to index of cluster slice,
	// value refers to the EstimatorPriority of who gave its estimated result.
	clusterIndexToEstimatorPriority := make(map[int]estimatorclient.EstimatorPriority)

	// Set the boundary.
	for i := range availableTargetClusters {
		availableTargetClusters[i].Name = clusters[i].Name
		availableTargetClusters[i].Replicas = math.MaxInt32
	}

	// For non-workload, like ServiceAccount, ConfigMap, Secret and etc, it's unnecessary to calculate available replicas in member clusters.
	// See issue: https://github.com/karmada-io/karmada/issues/3743.
	if spec.Replicas == 0 {
		klog.V(4).Infof("Do not calculate available replicas for non-workload(%s, kind=%s, %s).", spec.Resource.APIVersion,
			spec.Resource.Kind, names.NamespacedKey(spec.Resource.Namespace, spec.Resource.Name))
		return availableTargetClusters
	}

	// Get all replicaEstimators, which are stored in TreeMap.
	replicaEstimators := estimatorclient.GetReplicaEstimators()
	ctx := context.WithValue(context.TODO(), util.ContextKeyObject,
		fmt.Sprintf("kind=%s, name=%s/%s", spec.Resource.Kind, spec.Resource.Namespace, spec.Resource.Name))

	// List all replicaEstimators in order of descending priority. The estimators are grouped with different priorities,
	// e.g: [priority:20, {estimators:[es1, es3]}, {priority:10, estimators:[es2, es4]}, ...]
	estimatorGroups := replicaEstimators.Values()

	// Iterate the estimator groups in order of descending priority
	for _, estimatorGroup := range estimatorGroups {
		// if higher-priority estimators have formed a full result of member clusters, no longer to call lower-priority estimator.
		if len(clusterIndexToEstimatorPriority) == len(clusters) {
			break
		}
		estimatorsWithSamePriority := estimatorGroup.(map[string]estimatorclient.ReplicaEstimator)
		// iterate through these estimators with the same priority.
		for _, estimator := range estimatorsWithSamePriority {
			res, err := estimator.MaxAvailableReplicas(ctx, clusters, spec.ReplicaRequirements)
			if err != nil {
				klog.Errorf("Max cluster available replicas error: %v", err)
				continue
			}
			for i := range res {
				// the result of this cluster estimated failed, ignore the corresponding result
				if res[i].Replicas == estimatorclient.UnauthenticReplica {
					continue
				}
				// the cluster name not match, ignore, which hardly ever happens
				if res[i].Name != availableTargetClusters[i].Name {
					klog.Errorf("unexpected cluster name in the result of estimator with %d priority, "+
						"expected: %s, got: %s", estimator.Priority(), availableTargetClusters[i].Name, res[i].Name)
					continue
				}
				// the result of this cluster has already been estimated by higher-priority estimator,
				// ignore the corresponding result by this estimator
				if priority, ok := clusterIndexToEstimatorPriority[i]; ok && estimator.Priority() < priority {
					continue
				}
				// if multiple estimators are called, choose the minimum value of each estimated result,
				// record the priority of result provider.
				if res[i].Replicas < availableTargetClusters[i].Replicas {
					availableTargetClusters[i].Replicas = res[i].Replicas
					clusterIndexToEstimatorPriority[i] = estimator.Priority()
				}
			}
		}
	}

	// In most cases, the target cluster max available replicas should not be MaxInt32 unless the workload is best-effort
	// and the scheduler-estimator has not been enabled. So we set the replicas to spec.Replicas for avoiding overflow.
	for i := range availableTargetClusters {
		if availableTargetClusters[i].Replicas == math.MaxInt32 {
			availableTargetClusters[i].Replicas = spec.Replicas
		}
	}

	klog.V(4).Infof("Target cluster calculated by estimators (available cluster && maxAvailableReplicas): %v", availableTargetClusters)
	return availableTargetClusters
}

// attachZeroReplicasCluster  attach cluster in clusters into targetCluster
// The purpose is to avoid workload not appeared in rb's spec.clusters field
func attachZeroReplicasCluster(clusters []*clusterv1alpha1.Cluster, targetClusters []workv1alpha2.TargetCluster) []workv1alpha2.TargetCluster {
	targetClusterSet := sets.NewString()
	for i := range targetClusters {
		targetClusterSet.Insert(targetClusters[i].Name)
	}
	for i := range clusters {
		if !targetClusterSet.Has(clusters[i].Name) {
			targetClusters = append(targetClusters, workv1alpha2.TargetCluster{Name: clusters[i].Name, Replicas: 0})
		}
	}
	return targetClusters
}

// removeZeroReplicasCLuster remove the cluster with 0 replicas in assignResults
func removeZeroReplicasCluster(assignResults []workv1alpha2.TargetCluster) []workv1alpha2.TargetCluster {
	targetClusters := make([]workv1alpha2.TargetCluster, 0, len(assignResults))
	for _, cluster := range assignResults {
		if cluster.Replicas > 0 {
			targetClusters = append(targetClusters, workv1alpha2.TargetCluster{Name: cluster.Name, Replicas: cluster.Replicas})
		}
	}
	return targetClusters
}
