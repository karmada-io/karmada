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

package cache

import (
	"sync"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
)

// Cache is an interface for scheduler internal cache.
type Cache interface {
	AddCluster(cluster *clusterv1alpha1.Cluster)
	UpdateCluster(cluster *clusterv1alpha1.Cluster)
	DeleteCluster(cluster *clusterv1alpha1.Cluster)
	Snapshot() Snapshot

	// Cache should be updated in response to RB/CRB changes
	OnResourceBindingAdd(obj interface{})
	OnResourceBindingUpdate(old, cur interface{})
	OnResourceBindingDelete(obj interface{})
}

// GroupType is a string defining the type of affinity used by a ResourceBinding or ClusterResourceBinding
type GroupType string

const (
	// GroupTypeAffinity declares the group type for affinity
	GroupTypeAffinity GroupType = "Affinity"
	// GroupTypeAntiAffinity declares the group type for anti-affinity
	GroupTypeAntiAffinity GroupType = "AntiAffinity"
)

// WorkloadGroupKey scopes an affinity group lookup.
// Namespace is used for namespaced ResourceBindings; for ClusterResourceBindings it is "".
// GroupType ensures Affinity and AntiAffinity never collide even if group strings match.
type WorkloadGroupKey struct {
	Namespace string
	Type      GroupType
	Group     string
}

func makeWorkloadGroupKey(namespace string, t GroupType, group string) WorkloadGroupKey {
	return WorkloadGroupKey{Namespace: namespace, Type: t, Group: group}
}

type schedulerCache struct {
	clusterLister clusterlister.ClusterLister
	mu            sync.RWMutex

	// bindingGroups maps bindingID -> set(WorkloadGroupKey), used as reverse index for updates / deletes
	bindingGroups map[string]sets.Set[WorkloadGroupKey]
	// clustersByBinding maps bindingID -> set(clusterName) for scheduled bindings
	clustersByBinding map[string]sets.Set[string]
	// groupPeers maps (ns, type, group) -> []bindingID
	groupMembers map[WorkloadGroupKey]sets.Set[string]
}

// NewCache instantiates a cache used only by scheduler.
func NewCache(clusterLister clusterlister.ClusterLister) Cache {
	return &schedulerCache{
		clusterLister:     clusterLister,
		groupMembers:      make(map[WorkloadGroupKey]sets.Set[string]),
		bindingGroups:     make(map[string]sets.Set[WorkloadGroupKey]),
		clustersByBinding: make(map[string]sets.Set[string]),
	}
}

// AddCluster does nothing since clusterLister would synchronize automatically
func (c *schedulerCache) AddCluster(_ *clusterv1alpha1.Cluster) {}

// UpdateCluster does nothing since clusterLister would synchronize automatically
func (c *schedulerCache) UpdateCluster(_ *clusterv1alpha1.Cluster) {}

// DeleteCluster does nothing since clusterLister would synchronize automatically
func (c *schedulerCache) DeleteCluster(_ *clusterv1alpha1.Cluster) {}

// Snapshot returns clusters' snapshot.
// TODO: optimize cloning
func (c *schedulerCache) Snapshot() Snapshot {
	out := NewEmptySnapshot()
	clusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return out
	}

	out.clusters = make([]*clusterv1alpha1.Cluster, 0, len(clusters))
	for _, cluster := range clusters {
		out.clusters = append(out.clusters, cluster.DeepCopy())
	}

	if features.FeatureGate.Enabled(features.WorkloadAffinity) {
		c.mu.RLock()
		defer c.mu.RUnlock()

		out.groupMembers = cloneMapOfSets(c.groupMembers)
		out.bindingGroups = cloneMapOfSets(c.bindingGroups)
		out.clustersByBinding = cloneMapOfSets(c.clustersByBinding)
	}

	return out
}

// OnResourceBindingAdd indexes a newly observed ResourceBinding or ClusterResourceBinding.
// Binding will only be indexed if work has been scheduled to clusters and the binding has relevant affinity groups.
// Operation is atomic (single lock).
func (c *schedulerCache) OnResourceBindingAdd(obj interface{}) {
	info := getBindingInfoFromObj(obj)
	if info == nil {
		return
	}

	// If binding is not scheduled anywhere, or does not have any affinity groups, do not update cache.
	if info.clusters == nil || info.clusters.Len() == 0 || (info.affinityGroup == "" && info.antiAffinityGroup == "") {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// For each non-empty group, index the binding. indexBindingLocked will also set clustersByBinding.
	if info.affinityGroup != "" {
		k := makeWorkloadGroupKey(info.namespace, GroupTypeAffinity, info.affinityGroup)
		c.indexBindingLocked(k, info.id, info.clusters)
	}
	if info.antiAffinityGroup != "" {
		k := makeWorkloadGroupKey(info.namespace, GroupTypeAntiAffinity, info.antiAffinityGroup)
		c.indexBindingLocked(k, info.id, info.clusters)
	}
}

// OnResourceBindingUpdate updates cache indexes only when relevant fields changed:
// - If clusters has changed
// - If affinity/anti-affinity group strings have changed
// The update is atomic (single lock)
func (c *schedulerCache) OnResourceBindingUpdate(oldObj, newObj interface{}) {
	oldInfo := getBindingInfoFromObj(oldObj)
	newInfo := getBindingInfoFromObj(newObj)
	if oldInfo == nil || newInfo == nil {
		return
	}

	// If clusters and affinity groups have not checked, skip cache update
	if setsEqualString(oldInfo.clusters, newInfo.clusters) &&
		oldInfo.affinityGroup == newInfo.affinityGroup &&
		oldInfo.antiAffinityGroup == newInfo.antiAffinityGroup &&
		oldInfo.id == newInfo.id {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// If the binding id unexpectedly changed, remove everything associated with the old id first.
	if oldInfo.id != newInfo.id {
		c.deleteByIDLocked(oldInfo.id)
	}

	// Remove old memberships
	if oldInfo.affinityGroup != "" {
		oldK := makeWorkloadGroupKey(oldInfo.namespace, GroupTypeAffinity, oldInfo.affinityGroup)
		c.unindexBindingLocked(oldK, oldInfo.id)
	}
	if oldInfo.antiAffinityGroup != "" {
		oldK := makeWorkloadGroupKey(oldInfo.namespace, GroupTypeAntiAffinity, oldInfo.antiAffinityGroup)
		c.unindexBindingLocked(oldK, oldInfo.id)
	}

	// Add new memberships only if newInfo is scheduled somewhere and has groups.
	if newInfo.clusters != nil && newInfo.clusters.Len() > 0 {
		if newInfo.affinityGroup != "" {
			newK := makeWorkloadGroupKey(newInfo.namespace, GroupTypeAffinity, newInfo.affinityGroup)
			c.indexBindingLocked(newK, newInfo.id, newInfo.clusters)
		}
		if newInfo.antiAffinityGroup != "" {
			newK := makeWorkloadGroupKey(newInfo.namespace, GroupTypeAntiAffinity, newInfo.antiAffinityGroup)
			c.indexBindingLocked(newK, newInfo.id, newInfo.clusters)
		}
	} else {
		// If new binding has no clusters, ensure clustersByBinding is removed for the new ID
		// (in case it was previously present)
		delete(c.clustersByBinding, newInfo.id)
	}
}

// OnResourceBindingDelete removes the binding's clusters mapping and group membership.
// It attempts a targeted removal using affinity fields on the tombstone; to be robust
// against tombstones that may lack group info, it falls back to a full reverse-index
// cleanup if necessary.
func (c *schedulerCache) OnResourceBindingDelete(obj interface{}) {
	info := getBindingInfoFromObj(obj)
	if info == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove group memberships if group info present on the tombstone.
	removedAny := false
	if info.affinityGroup != "" {
		k := makeWorkloadGroupKey(info.namespace, GroupTypeAffinity, info.affinityGroup)
		c.unindexBindingLocked(k, info.id)
		removedAny = true
	}
	if info.antiAffinityGroup != "" {
		k := makeWorkloadGroupKey(info.namespace, GroupTypeAntiAffinity, info.antiAffinityGroup)
		c.unindexBindingLocked(k, info.id)
		removedAny = true
	}

	// If tombstone lacked group info, or we suspect there may still be a membership,
	// use the reverse-index cleanup.
	if !removedAny || c.isPossiblyStillMemberLocked(info.id) {
		c.deleteByIDLocked(info.id)
	}
}

// indexBindingLocked adds bindingID to the specified group key and ensures the reverse index
// and cluster mapping are updated. Caller MUST hold c.mu.
func (c *schedulerCache) indexBindingLocked(k WorkloadGroupKey, bindingID string, clusters sets.Set[string]) {
	// add to groupMembers
	members := c.groupMembers[k]
	if members == nil {
		members = sets.New[string]()
		c.groupMembers[k] = members
	}
	members.Insert(bindingID)

	// add to bindingGroups (reverse index)
	bg := c.bindingGroups[bindingID]
	if bg == nil {
		bg = sets.New[WorkloadGroupKey]()
		c.bindingGroups[bindingID] = bg
	}
	bg.Insert(k)

	// ensure clusters mapping exists if clusters provided
	if clusters != nil && clusters.Len() > 0 {
		c.clustersByBinding[bindingID] = clusters
	}
}

// unindexBindingLocked removes bindingID from the specified group key and updates the
// reverse index. If that was the last group for this binding, it also removes the
// clustersByBinding entry. Caller MUST hold c.mu.
func (c *schedulerCache) unindexBindingLocked(k WorkloadGroupKey, bindingID string) {
	// remove from groupMembers
	if members := c.groupMembers[k]; members != nil {
		members.Delete(bindingID)
		if members.Len() == 0 {
			delete(c.groupMembers, k)
		}
	}

	// remove from bindingGroups, and if no groups left for this binding, remove clusters mapping
	if bg := c.bindingGroups[bindingID]; bg != nil {
		bg.Delete(k)
		if bg.Len() == 0 {
			// no more groups reference this binding
			delete(c.bindingGroups, bindingID)
			delete(c.clustersByBinding, bindingID)
		} else {
			// keep updated set (not strictly necessary because bg is the same object)
			c.bindingGroups[bindingID] = bg
		}
	}
}

// deleteByIDLocked removes all traces of bindingID using the reverse index.
// Caller must hold c.mu.
func (c *schedulerCache) deleteByIDLocked(bindingID string) {
	// Remove cluster mapping
	delete(c.clustersByBinding, bindingID)

	// Use bindingGroups to remove bindingID from all groups efficiently
	if bg := c.bindingGroups[bindingID]; bg != nil {
		for g := range bg {
			if members := c.groupMembers[g]; members != nil {
				members.Delete(bindingID)
				if members.Len() == 0 {
					delete(c.groupMembers, g)
				}
			}
		}
		delete(c.bindingGroups, bindingID)
	}
}

// Helpers

// bindingInfo contains metadata on te ResourceBinding being indexed or unindexed by this cache
type bindingInfo struct {
	id                string
	namespace         string // "" for CRB
	clusters          sets.Set[string]
	affinityGroup     string
	antiAffinityGroup string
}

func getBindingInfoFromObj(obj interface{}) *bindingInfo {
	switch t := obj.(type) {
	case *workv1alpha2.ResourceBinding:
		return bindingInfoFromRB(t)
	case *workv1alpha2.ClusterResourceBinding:
		return bindingInfoFromCRB(t)
	case cache.DeletedFinalStateUnknown:
		return getBindingInfoFromObj(t.Obj)
	default:
		return nil
	}
}

func bindingInfoFromRB(rb *workv1alpha2.ResourceBinding) *bindingInfo {
	id := "rb:" + rb.Namespace + "/" + rb.Name

	clusters := sets.New[string]()
	for _, target := range rb.Spec.Clusters {
		clusters.Insert(target.Name)
	}

	var aff, anti string
	if rb.Spec.WorkloadAffinityGroups != nil {
		aff = rb.Spec.WorkloadAffinityGroups.AffinityGroup
		anti = rb.Spec.WorkloadAffinityGroups.AntiAffinityGroup
	}

	return &bindingInfo{
		id:                id,
		namespace:         rb.Namespace,
		clusters:          clusters,
		affinityGroup:     aff,
		antiAffinityGroup: anti,
	}
}

func bindingInfoFromCRB(crb *workv1alpha2.ClusterResourceBinding) *bindingInfo {
	id := "crb:" + crb.Name

	clusters := sets.New[string]()
	for _, target := range crb.Spec.Clusters {
		clusters.Insert(target.Name)
	}

	var aff, anti string
	if crb.Spec.WorkloadAffinityGroups != nil {
		aff = crb.Spec.WorkloadAffinityGroups.AffinityGroup
		anti = crb.Spec.WorkloadAffinityGroups.AntiAffinityGroup
	}

	return &bindingInfo{
		id:                id,
		namespace:         "", // cluster-scoped
		clusters:          clusters,
		affinityGroup:     aff,
		antiAffinityGroup: anti,
	}
}

// isPossiblyStillMemberLocked returns true if bindingID still appears in any group.
// Caller must hold c.mu.
func (c *schedulerCache) isPossiblyStillMemberLocked(bindingID string) bool {
	for _, m := range c.groupMembers {
		if m.Has(bindingID) {
			return true
		}
	}
	return false
}

// setsEqualString checks if sets a, b are equal
func setsEqualString(a, b sets.Set[string]) bool {
	if a == nil && b == nil {
		return true
	}
	if (a == nil) != (b == nil) {
		return false
	}
	return a.Equal(b)
}

// cloneSet returns a deep copy of s (nil-safe).
func cloneSet[T comparable](s sets.Set[T]) sets.Set[T] {
	if s == nil {
		return nil
	}
	cp := sets.New[T]()
	for v := range s {
		cp.Insert(v)
	}
	return cp
}

// cloneMapOfSets returns a deep copy of a map whose values are sets (nil-safe values are skipped).
func cloneMapOfSets[K comparable, V comparable](in map[K]sets.Set[V]) map[K]sets.Set[V] {
	if in == nil {
		return nil
	}
	out := make(map[K]sets.Set[V], len(in))
	for k, s := range in {
		if s == nil {
			continue
		}
		out[k] = cloneSet[V](s)
	}
	return out
}
