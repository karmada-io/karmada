/*
Copyright 2026 The Karmada Authors.

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
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// PreemptionResult describes the lower-priority bindings selected for eviction
// so that a higher-priority binding can be scheduled later.
type PreemptionResult struct {
	// Cluster is the target cluster where preemption should happen.
	Cluster string

	// Victims contains lower-priority bindings to evict from Cluster.
	Victims []VictimBinding
}

// VictimBinding identifies a lower-priority binding selected for preemption.
type VictimBinding struct {
	Namespace string
	Name      string
	Replicas  int32
	Priority  int32
}

type preemptionVictimCandidate struct {
	namespace         string
	name              string
	replicas          int32
	priority          int32
	creationTimestamp int64
}

func listPreemptionVictimCandidates(indexer cache.Indexer, preemptor *workv1alpha2.ResourceBindingSpec, targetCluster string) ([]preemptionVictimCandidate, error) {
	if indexer == nil {
		return nil, fmt.Errorf("resource binding indexer is nil")
	}
	if preemptor == nil {
		return nil, fmt.Errorf("preemptor binding spec is nil")
	}

	preemptorPriority := preemptor.SchedulePriorityValue()
	candidates := make([]preemptionVictimCandidate, 0)
	for _, obj := range indexer.List() {
		binding, ok := obj.(*workv1alpha2.ResourceBinding)
		if !ok {
			return nil, fmt.Errorf("object is not a ResourceBinding: %v", obj)
		}
		if binding.Spec.SchedulePriorityValue() >= preemptorPriority {
			continue
		}
		if binding.Spec.ClusterInGracefulEvictionTasks(targetCluster) {
			continue
		}
		replicas := binding.Spec.AssignedReplicasForCluster(targetCluster)
		if replicas <= 0 {
			continue
		}

		candidates = append(candidates, preemptionVictimCandidate{
			namespace:         binding.Namespace,
			name:              binding.Name,
			replicas:          replicas,
			priority:          binding.Spec.SchedulePriorityValue(),
			creationTimestamp: binding.CreationTimestamp.UnixNano(),
		})
	}
	return candidates, nil
}

func selectVictimsByReplicaCount(preemptor *workv1alpha2.ResourceBindingSpec, candidates []preemptionVictimCandidate, clusterAvailable int32) ([]VictimBinding, error) {
	if preemptor == nil {
		return nil, fmt.Errorf("preemptor binding spec is nil")
	}

	deficit := int64(preemptor.Replicas) - int64(clusterAvailable)
	if deficit <= 0 {
		return nil, nil
	}

	preemptorPriority := preemptor.SchedulePriorityValue()
	filtered := make([]preemptionVictimCandidate, 0, len(candidates))
	var totalCandidateReplicas int64
	for _, candidate := range candidates {
		if candidate.priority >= preemptorPriority || candidate.replicas <= 0 {
			continue
		}
		filtered = append(filtered, candidate)
		totalCandidateReplicas += int64(candidate.replicas)
	}
	if totalCandidateReplicas < deficit {
		return nil, fmt.Errorf("candidate replicas %d are not enough to cover deficit %d", totalCandidateReplicas, deficit)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].priority != filtered[j].priority {
			return filtered[i].priority > filtered[j].priority
		}
		if filtered[i].creationTimestamp != filtered[j].creationTimestamp {
			return filtered[i].creationTimestamp < filtered[j].creationTimestamp
		}
		return names.NamespacedKey(filtered[i].namespace, filtered[i].name) < names.NamespacedKey(filtered[j].namespace, filtered[j].name)
	})

	reprieved := make([]bool, len(filtered))
	remainingVictimReplicas := totalCandidateReplicas
	for i, candidate := range filtered {
		if remainingVictimReplicas-int64(candidate.replicas) >= deficit {
			reprieved[i] = true
			remainingVictimReplicas -= int64(candidate.replicas)
		}
	}

	victims := make([]VictimBinding, 0)
	for i, candidate := range filtered {
		if reprieved[i] {
			continue
		}
		victims = append(victims, VictimBinding{
			Namespace: candidate.namespace,
			Name:      candidate.name,
			Replicas:  candidate.replicas,
			Priority:  candidate.priority,
		})
	}
	return victims, nil
}

func (g *genericScheduler) preempt(_ context.Context, clustersScore framework.ClusterScoreList, spec *workv1alpha2.ResourceBindingSpec, status *workv1alpha2.ResourceBindingStatus) (*PreemptionResult, error) {
	if !isPreemptionApplicable(spec) || g.preemptionClaims == nil {
		return nil, nil
	}

	targets, err := g.selectClustersForPreemption(clustersScore, spec.Placement, spec, status)
	if err != nil {
		klog.V(4).Infof("Preemption skipped for %s: failed to select target cluster ignoring available replicas: %v", preemptionClaimBindingKey(spec), err)
		return nil, nil
	}
	if len(targets) != 1 {
		klog.V(4).Infof("Preemption skipped for %s: expected one target cluster, got %d", preemptionClaimBindingKey(spec), len(targets))
		return nil, nil
	}

	target := targets[0]
	if g.preemptionClaims.HasClaimOnCluster(target.Name) {
		klog.V(4).Infof("Preemption skipped for %s: cluster %q already has an active preemption claim", preemptionClaimBindingKey(spec), target.Name)
		return nil, nil
	}

	candidates, err := listPreemptionVictimCandidates(g.schedulerCache.ResourceBindingIndexer(), spec, target.Name)
	if err != nil {
		return nil, err
	}
	victims, err := selectVictimsByReplicaCount(spec, candidates, int32(target.AvailableReplicas)) // #nosec G115: available replicas are derived from int32 replica counts.
	if err != nil {
		klog.V(4).Infof("Preemption skipped for %s on cluster %q: %v", preemptionClaimBindingKey(spec), target.Name, err)
		return nil, nil
	}
	if len(victims) == 0 {
		return nil, nil
	}

	g.preemptionClaims.Set(preemptionClaim{
		bindingKey:   preemptionClaimBindingKey(spec),
		cluster:      target.Name,
		priority:     spec.SchedulePriorityValue(),
		replicas:     spec.Replicas,
		resourceNeed: preemptionResourceNeed(spec),
	})

	return &PreemptionResult{
		Cluster: target.Name,
		Victims: victims,
	}, nil
}

func isPreemptionApplicable(spec *workv1alpha2.ResourceBindingSpec) bool {
	if !features.PreemptionEnabled() || spec == nil || !spec.IsWorkload() || len(spec.Components) > 1 {
		return false
	}
	if spec.SchedulePriority == nil || spec.SchedulePriority.PreemptionPolicy != workv1alpha2.PreemptLowerPriority {
		return false
	}
	if spec.Placement == nil || spec.Placement.ClusterAffinity == nil || len(spec.Placement.ClusterAffinities) != 0 {
		return false
	}
	if !usesAggregatedSingleClusterScheduling(spec.Placement) {
		return false
	}
	return true
}

func usesAggregatedSingleClusterScheduling(placement *policyv1alpha1.Placement) bool {
	if placement.ReplicaScheduling == nil ||
		placement.ReplicaScheduling.ReplicaSchedulingType != policyv1alpha1.ReplicaSchedulingTypeDivided ||
		placement.ReplicaScheduling.ReplicaDivisionPreference != policyv1alpha1.ReplicaDivisionPreferenceAggregated {
		return false
	}
	for _, constraint := range placement.SpreadConstraints {
		if constraint.SpreadByField == policyv1alpha1.SpreadByFieldCluster && constraint.MaxGroups == 1 {
			return true
		}
	}
	return false
}

func preemptionClaimBindingKey(spec *workv1alpha2.ResourceBindingSpec) string {
	return names.NamespacedKey(spec.Resource.Namespace, spec.Resource.Name)
}

func preemptionResourceNeed(spec *workv1alpha2.ResourceBindingSpec) corev1.ResourceList {
	if spec.ReplicaRequirements == nil || spec.ReplicaRequirements.ResourceRequest == nil {
		return nil
	}
	return spec.ReplicaRequirements.ResourceRequest.DeepCopy()
}
