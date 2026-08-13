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
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
)

const maxPreemptionVictimCandidates = 1000
const defaultSchedulerName = "default-scheduler"

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
	schedulerName     string
	resourceRequest   corev1.ResourceList
	creationTimestamp int64
}

func listPreemptionVictimCandidates(indexer cache.Indexer, preemptor *workv1alpha2.ResourceBindingSpec, targetCluster, schedulerName string) ([]preemptionVictimCandidate, error) {
	if indexer == nil {
		return nil, fmt.Errorf("resource binding indexer is nil")
	}
	if preemptor == nil {
		return nil, fmt.Errorf("preemptor binding spec is nil")
	}

	preemptorPriority := preemptor.SchedulePriorityValue()
	candidates := make([]preemptionVictimCandidate, 0)
	objs, err := indexer.ByIndex(indexregistry.ResourceBindingIndexByFieldCluster, targetCluster)
	if err != nil {
		return nil, fmt.Errorf("failed to list ResourceBindings by cluster index %q: %w", targetCluster, err)
	}
	if len(objs) > maxPreemptionVictimCandidates {
		return nil, fmt.Errorf("too many preemption victim candidates on cluster %q: %d exceeds cap %d", targetCluster, len(objs), maxPreemptionVictimCandidates)
	}
	for _, obj := range objs {
		binding, ok := obj.(*workv1alpha2.ResourceBinding)
		if !ok {
			return nil, fmt.Errorf("object is not a ResourceBinding: %v", obj)
		}
		if !schedulerNameMatches(schedulerName, binding.Spec.SchedulerName) {
			continue
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
			schedulerName:     binding.Spec.SchedulerName,
			resourceRequest:   bindingResourceRequest(&binding.Spec),
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
	deficitResources := resourceListForReplicas(bindingResourceRequest(preemptor), deficit)
	if len(deficitResources) == 0 {
		return nil, fmt.Errorf("preemptor resource request is empty")
	}

	filtered := make([]preemptionVictimCandidate, 0, len(candidates))
	totalCandidateResources := corev1.ResourceList{}
	for _, candidate := range candidates {
		if candidate.priority >= preemptorPriority || candidate.replicas <= 0 || len(candidate.resourceRequest) == 0 {
			continue
		}
		filtered = append(filtered, candidate)
		addResourceList(totalCandidateResources, resourceListForReplicas(candidate.resourceRequest, int64(candidate.replicas)))
	}
	if !resourceListCovers(totalCandidateResources, deficitResources) {
		return nil, fmt.Errorf("candidate resources are not enough to cover deficit resources")
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].priority != filtered[j].priority {
			return filtered[i].priority > filtered[j].priority
		}
		if filtered[i].creationTimestamp != filtered[j].creationTimestamp {
			return filtered[i].creationTimestamp < filtered[j].creationTimestamp
		}
		return filtered[i].namespace+"/"+filtered[i].name < filtered[j].namespace+"/"+filtered[j].name
	})

	reprieved := make([]bool, len(filtered))
	remainingVictimResources := cloneResourceList(totalCandidateResources)
	for i, candidate := range filtered {
		candidateResources := resourceListForReplicas(candidate.resourceRequest, int64(candidate.replicas))
		afterReprieve := cloneResourceList(remainingVictimResources)
		subtractResourceList(afterReprieve, candidateResources)
		if resourceListCovers(afterReprieve, deficitResources) {
			reprieved[i] = true
			remainingVictimResources = afterReprieve
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

func (g *genericScheduler) preempt(_ context.Context, clustersScore framework.ClusterScoreList, spec *workv1alpha2.ResourceBindingSpec, status *workv1alpha2.ResourceBindingStatus, option *ScheduleAlgorithmOption) (*PreemptionResult, error) {
	if !isPreemptionApplicable(spec, option) || g.preemptionClaims == nil {
		return nil, nil
	}

	targets, err := g.selectClustersForPreemption(clustersScore, spec.Placement, spec, status)
	if err != nil {
		klog.V(4).Infof("Preemption skipped for %s: failed to select target cluster ignoring available replicas: %v", option.BindingIdentity.Key(), err)
		return nil, nil
	}
	if len(targets) != 1 {
		klog.V(4).Infof("Preemption skipped for %s: expected one target cluster, got %d", option.BindingIdentity.Key(), len(targets))
		return nil, nil
	}

	target := targets[0]
	if g.preemptionClaims.HasClaimOnCluster(target.Name) {
		klog.V(4).Infof("Preemption skipped for %s: cluster %q already has an active preemption claim", option.BindingIdentity.Key(), target.Name)
		return nil, nil
	}

	candidates, err := listPreemptionVictimCandidates(g.schedulerCache.ResourceBindingIndexer(), spec, target.Name, option.SchedulerName)
	if err != nil {
		return nil, err
	}
	victims, err := selectVictimsByReplicaCount(spec, candidates, int32(target.AvailableReplicas)) // #nosec G115: available replicas are derived from int32 replica counts.
	if err != nil {
		klog.V(4).Infof("Preemption skipped for %s on cluster %q: %v", option.BindingIdentity.Key(), target.Name, err)
		return nil, nil
	}
	if len(victims) == 0 {
		return nil, nil
	}

	g.preemptionClaims.Set(preemptionClaim{
		bindingKey:   option.BindingIdentity.Key(),
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

func isPreemptionApplicable(spec *workv1alpha2.ResourceBindingSpec, option *ScheduleAlgorithmOption) bool {
	if !features.PreemptionEnabled() || spec == nil || !spec.IsWorkload() || len(spec.Components) > 1 {
		return false
	}
	if option == nil || !option.BindingIdentity.IsResourceBinding() {
		return false
	}
	if spec.SchedulePriority == nil || spec.SchedulePriority.PreemptionPolicy != workv1alpha2.PreemptLowerPriority {
		return false
	}
	if len(bindingResourceRequest(spec)) == 0 {
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

func preemptionResourceNeed(spec *workv1alpha2.ResourceBindingSpec) corev1.ResourceList {
	return bindingResourceRequest(spec)
}

func bindingResourceRequest(spec *workv1alpha2.ResourceBindingSpec) corev1.ResourceList {
	if spec == nil || spec.ReplicaRequirements == nil || len(spec.ReplicaRequirements.ResourceRequest) == 0 {
		return nil
	}
	return spec.ReplicaRequirements.ResourceRequest.DeepCopy()
}

func schedulerNameMatches(expected, actual string) bool {
	return normalizeSchedulerName(expected) == normalizeSchedulerName(actual)
}

func normalizeSchedulerName(name string) string {
	if name == "" {
		return defaultSchedulerName
	}
	return name
}

func resourceListForReplicas(request corev1.ResourceList, replicas int64) corev1.ResourceList {
	if replicas <= 0 || len(request) == 0 {
		return nil
	}
	out := make(corev1.ResourceList, len(request))
	for name, quantity := range request {
		if !positiveQuantity(quantity) {
			continue
		}
		q := quantity.DeepCopy()
		q.Mul(replicas)
		out[name] = q
	}
	return out
}

func addResourceList(dst, add corev1.ResourceList) {
	for name, quantity := range add {
		existing := dst[name]
		existing.Add(quantity)
		dst[name] = existing
	}
}

func subtractResourceList(dst, sub corev1.ResourceList) {
	for name, quantity := range sub {
		existing := dst[name]
		existing.Sub(quantity)
		dst[name] = existing
	}
}

func resourceListCovers(available, needed corev1.ResourceList) bool {
	if len(needed) == 0 {
		return false
	}
	for name, neededQuantity := range needed {
		if !positiveQuantity(neededQuantity) {
			continue
		}
		availableQuantity := available[name]
		if availableQuantity.Cmp(neededQuantity) < 0 {
			return false
		}
	}
	return true
}

func cloneResourceList(in corev1.ResourceList) corev1.ResourceList {
	if in == nil {
		return nil
	}
	return in.DeepCopy()
}
