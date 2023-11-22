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
