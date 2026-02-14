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
	TargetClusterReplicaStatus []*ClusterReplicaStatus
}

// NewSchedulingResultHelper returns a new SchedulingResultHelper based on ResourceBinding.
func NewSchedulingResultHelper(binding *workv1alpha2.ResourceBinding) *SchedulingResultHelper {
	h := &SchedulingResultHelper{ResourceBinding: binding}
	readyReplicas := getReadyReplicas(binding)
	for i := range binding.Spec.Clusters {
		targetCluster := &binding.Spec.Clusters[i]
		targetClusterReplicaStatus := &ClusterReplicaStatus{
			ClusterName: targetCluster.Name,
			Spec:        targetCluster.Replicas,
		}
		if ready, exist := readyReplicas[targetCluster.Name]; exist {
			targetClusterReplicaStatus.Ready = ready
		} else {
			targetClusterReplicaStatus.Ready = estimatorclient.UnauthenticReplica
		}
		h.TargetClusterReplicaStatus = append(h.TargetClusterReplicaStatus, targetClusterReplicaStatus)
	}
	return h
}

// FillUnschedulableReplicas will detect the unschedulable replicas of member cluster by calling
// unschedulable replica estimators and fill the unschedulable field of ClusterReplicaStatus.
func (h *SchedulingResultHelper) FillUnschedulableReplicas(unschedulableThreshold time.Duration) {
	reference := &h.Spec.Resource
	undesiredClusters, undesiredClusterNames := h.GetUndesiredClusters()
	// Set the boundary.
	for i := range undesiredClusters {
		undesiredClusters[i].Unschedulable = math.MaxInt32
	}
	// Get the minimum value of MaxAvailableReplicas in terms of all estimators.
	estimators := estimatorclient.GetUnschedulableReplicaEstimators()
	ctx := context.WithValue(context.TODO(), util.ContextKeyObject,
		fmt.Sprintf("kind=%s, name=%s/%s", reference.Kind, reference.Namespace, reference.Name))
	for _, estimator := range estimators {
		res, err := estimator.GetUnschedulableReplicas(ctx, undesiredClusterNames, reference, unschedulableThreshold)
		if err != nil {
			klog.Errorf("Max cluster unschedulable replicas error: %v", err)
			continue
		}
		for i := range res {
			if res[i].Replicas == estimatorclient.UnauthenticReplica {
				continue
			}
			if undesiredClusters[i].ClusterName == res[i].Name && undesiredClusters[i].Unschedulable > res[i].Replicas {
				undesiredClusters[i].Unschedulable = res[i].Replicas
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
func (h *SchedulingResultHelper) GetUndesiredClusters() ([]*ClusterReplicaStatus, []string) {
	var clusters []*ClusterReplicaStatus
	var names []string
	for _, cluster := range h.TargetClusterReplicaStatus {
		if cluster.Ready < cluster.Spec {
			clusters = append(clusters, cluster)
			names = append(names, cluster.ClusterName)
		}
	}
	return clusters, names
}

// ClusterReplicaStatus represents the replicas status of a workload on a particular cluster.
type ClusterReplicaStatus struct {
	// ClusterName is the name of the member cluster targeted by ResourceBinding.
	ClusterName string

	// Spec is the desired replicas assigned to this cluster (from ResourceBinding.spec.clusters).
	Spec int32

	// Ready is the number of replicas currently reported as ready in this cluster.
	Ready int32

	// Unschedulable is the estimated count of replicas that cannot be scheduled in this cluster.
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

		workloadStatus := make(map[string]any)
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
