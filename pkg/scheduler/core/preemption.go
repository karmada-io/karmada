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
	"fmt"
	"sort"

	"k8s.io/client-go/tools/cache"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
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
