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

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/kr/pretty"
	"k8s.io/klog/v2"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/util"
)

// SchedulingResultHelper is a helper to wrap the ResourceBinding and its target cluster result.
type SchedulingResultHelper struct {
	*workv1alpha2.ResourceBinding
	TargetClusters []*TargetClusterWrapper
}

// NewSchedulingResultHelper returns a new SchedulingResultHelper based on ResourceBinding.
func NewSchedulingResultHelper(binding *workv1alpha2.ResourceBinding) *SchedulingResultHelper {
	h := &SchedulingResultHelper{ResourceBinding: binding}
	readyReplicas := getReadyReplicas(binding)
	for i := range binding.Spec.Clusters {
		targetCluster := &binding.Spec.Clusters[i]
		targetClusterHelper := &TargetClusterWrapper{
			ClusterName: targetCluster.Name,
			Spec:        targetCluster.Replicas,
		}
		if ready, exist := readyReplicas[targetCluster.Name]; exist {
			targetClusterHelper.Ready = ready
		} else {
			targetClusterHelper.Ready = estimatorclient.UnauthenticReplica
		}
		h.TargetClusters = append(h.TargetClusters, targetClusterHelper)
	}
	return h
}

// FillUnschedulableReplicas will detect the unschedulable replicas of member cluster by calling
// unschedulable replica estimators and fill the unschedulable field of TargetClusterWrapper.
func (h *SchedulingResultHelper) FillUnschedulableReplicas(unschedulableThreshold time.Duration) {
	reference := &h.Spec.Resource
	undesiredClusters, undesiredClusterNames := h.GetUndesiredClusters()
	// clusterIndexToEstimatorPriority key refers to index of cluster slice,
	// value refers to the EstimatorPriority of who gave its estimated result.
	clusterIndexToEstimatorPriority := make(map[int]estimatorclient.EstimatorPriority)

	// Set the boundary.
	for i := range undesiredClusters {
		undesiredClusters[i].Unschedulable = math.MaxInt32
	}

	// Get all replicaEstimators, which are stored in TreeMap.
	estimators := estimatorclient.GetUnschedulableReplicaEstimators()
	ctx := context.WithValue(context.TODO(), util.ContextKeyObject,
		fmt.Sprintf("kind=%s, name=%s/%s", reference.Kind, reference.Namespace, reference.Name))

	// List all unschedulableReplicaEstimators in order of descending priority. The estimators are grouped with different
	// priorities, e.g: [priority:20, {estimators:[es1, es3]}, {priority:10, estimators:[es2, es4]}, ...]
	estimatorGroups := estimators.Values()

	// Iterate the estimator groups in order of descending priority
	for _, estimatorGroup := range estimatorGroups {
		// if higher-priority estimators have formed a full result of member clusters, no longer to call lower-priority estimator.
		if len(clusterIndexToEstimatorPriority) == len(undesiredClusterNames) {
			break
		}
		estimatorsWithSamePriority := estimatorGroup.(map[string]estimatorclient.UnschedulableReplicaEstimator)
		// iterate through these estimators with the same priority.
		for _, estimator := range estimatorsWithSamePriority {
			res, err := estimator.GetUnschedulableReplicas(ctx, undesiredClusterNames, reference, unschedulableThreshold)
			if err != nil {
				klog.Errorf("Max cluster unschedulable replicas error: %v", err)
				continue
			}
			for i := range res {
				// the result of this cluster estimated failed, ignore the corresponding result
				if res[i].Replicas == estimatorclient.UnauthenticReplica {
					continue
				}
				// the cluster name not match, ignore, which hardly ever happens
				if res[i].Name != undesiredClusters[i].ClusterName {
					klog.Errorf("unexpected cluster name in the result of estimator with %d priority, "+
						"expected: %s, got: %s", estimator.Priority(), undesiredClusters[i].ClusterName, res[i].Name)
					continue
				}
				// the result of this cluster has already been estimated by higher-priority estimator,
				// ignore the corresponding result by this estimator
				if priority, ok := clusterIndexToEstimatorPriority[i]; ok && estimator.Priority() < priority {
					continue
				}
				// if multiple estimators are called, choose the minimum value of each estimated result,
				// record the priority of result provider.
				if res[i].Replicas < undesiredClusters[i].Unschedulable {
					undesiredClusters[i].Unschedulable = res[i].Replicas
					clusterIndexToEstimatorPriority[i] = estimator.Priority()
				}
			}
		}
	}

	for i := range undesiredClusters {
		if undesiredClusters[i].Unschedulable == math.MaxInt32 {
			undesiredClusters[i].Unschedulable = 0
		}
	}

	klog.V(4).Infof("Target undesired cluster of unschedulable replica result: %s", pretty.Sprint(undesiredClusters))
}

// GetUndesiredClusters returns the cluster which of ready replicas are not reach the ready ones.
func (h *SchedulingResultHelper) GetUndesiredClusters() ([]*TargetClusterWrapper, []string) {
	var clusters []*TargetClusterWrapper
	var names []string
	for _, cluster := range h.TargetClusters {
		if cluster.Ready < cluster.Spec {
			clusters = append(clusters, cluster)
			names = append(names, cluster.ClusterName)
		}
	}
	return clusters, names
}

// TargetClusterWrapper is a wrapper to wrap the target cluster name, spec replicas,
// ready replicas and unschedulable replicas.
type TargetClusterWrapper struct {
	ClusterName   string
	Spec          int32
	Ready         int32
	Unschedulable int32
}

func getReadyReplicas(binding *workv1alpha2.ResourceBinding) map[string]int32 {
	aggregatedStatus := binding.Status.AggregatedStatus
	res := make(map[string]int32, len(aggregatedStatus))
	for i := range aggregatedStatus {
		item := aggregatedStatus[i]
		if item.Status == nil {
			continue
		}

		workloadStatus := make(map[string]interface{})
		if err := json.Unmarshal(item.Status.Raw, &workloadStatus); err != nil {
			klog.ErrorS(err, "Failed to unmarshal workload status when get ready replicas", "ResourceBinding", klog.KObj(binding))
			continue
		}
		readyReplicas := int32(0)
		// TODO(Garrybest): cooperate with custom resource interpreter
		if r, ok := workloadStatus[util.ReadyReplicasField]; ok {
			readyReplicas = int32(r.(float64))
			res[item.ClusterName] = readyReplicas
		}
	}
	return res
}
