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
	"maps"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/resourceversion"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// DefaultAssumptionTTL is the default duration a reservation is kept alive after it is created.
// Once a workload's AggregatedStatus becomes Healthy the reservation is released immediately;
// this TTL serves as a safety net for workloads that take unusually long to start or whose
// health signal never arrives.
const DefaultAssumptionTTL = 5 * time.Minute

// AssumedWorkload holds the resource footprint of an in-flight workload set that has been
// assigned to a specific cluster by the scheduler but whose pods have not yet reached a Healthy
// state.  The estimator deducts this footprint from the cluster's available capacity to avoid
// over-commitment when multiple workloads are scheduled in rapid succession.
type AssumedWorkload struct {
	// Namespace is the namespace of the reserved workload.
	Namespace string
	// Components lists the component types and their per-replica resource requirements
	// that are reserved on the target cluster.
	Components []workv1alpha2.Component
}

// BindingAssumption groups all per-cluster assumptions belonging to a single
// ResourceBinding and carries the expiry time for the entire binding entry.
type BindingAssumption struct {
	// entries maps clusterName → reserved workload for that cluster.
	entries map[string]AssumedWorkload
	// ExpiresAt is the wall-clock time after which this reservation is considered stale
	// and will be removed by the GC sweep.
	ExpiresAt time.Time
}

// Cache is an interface for scheduler internal cache.
type Cache interface {
	AddCluster(cluster *clusterv1alpha1.Cluster)
	UpdateCluster(cluster *clusterv1alpha1.Cluster)
	DeleteCluster(cluster *clusterv1alpha1.Cluster)
	// Snapshot returns a snapshot of the current clusters info
	Snapshot() Snapshot

	// ResourceBindingIndexer returns the indexer for ResourceBindings, used for advanced scheduling logic.
	ResourceBindingIndexer() cache.Indexer

	// AssigningResourceBindings returns a cache for ResourceBindings that are in the "assigning" state.
	// After the control plane generates scheduling results and commits them to the API server,
	// there is a period before these results are fully propagated to and reflected in the member clusters.
	// This cache stores these "assigning" bindings to provide the scheduler with a more accurate and
	// immediate view of the intended state, enabling more precise decision-making during subsequent scheduling cycles.
	AssigningResourceBindings() *AssigningResourceBindingCache
}

type schedulerCache struct {
	clusterLister          clusterlister.ClusterLister
	resourceBindingIndexer cache.Indexer

	assigningResourceBindings *AssigningResourceBindingCache
}

// Check if our schedulerCache implements necessary interface
var _ Cache = &schedulerCache{}

// NewCache instantiates a cache used only by scheduler.
func NewCache(clusterLister clusterlister.ClusterLister, resourceBindingIndexer cache.Indexer, assumptionTTL time.Duration) Cache {
	if assumptionTTL <= 0 {
		assumptionTTL = DefaultAssumptionTTL
	}
	return &schedulerCache{
		clusterLister:          clusterLister,
		resourceBindingIndexer: resourceBindingIndexer,
		assigningResourceBindings: &AssigningResourceBindingCache{
			items:       make(map[string]*workv1alpha2.ResourceBinding),
			assumptions: make(map[string]*BindingAssumption),
			ttl:         assumptionTTL,
		},
	}
}

// AddCluster does nothing since clusterLister would synchronize automatically
func (c *schedulerCache) AddCluster(_ *clusterv1alpha1.Cluster) {
}

// UpdateCluster does nothing since clusterLister would synchronize automatically
func (c *schedulerCache) UpdateCluster(_ *clusterv1alpha1.Cluster) {
}

// DeleteCluster does nothing since clusterLister would synchronize automatically
func (c *schedulerCache) DeleteCluster(_ *clusterv1alpha1.Cluster) {
}

// Snapshot returns clusters' snapshot.
// TODO: need optimization, only clone when necessary
func (c *schedulerCache) Snapshot() Snapshot {
	out := NewEmptySnapshot()
	clusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return out
	}

	out.clusterInfoList = make([]*framework.ClusterInfo, 0, len(clusters))
	for _, cluster := range clusters {
		cloned := cluster.DeepCopy()
		out.clusterInfoList = append(out.clusterInfoList, framework.NewClusterInfo(cloned))
	}

	return out
}

// ResourceBindingIndexer returns the indexer for ResourceBindings.
func (c *schedulerCache) ResourceBindingIndexer() cache.Indexer {
	return c.resourceBindingIndexer
}

// AssigningResourceBindings returns the cache of ResourceBindings that are in the "assigning" state.
func (c *schedulerCache) AssigningResourceBindings() *AssigningResourceBindingCache {
	return c.assigningResourceBindings
}

// AssigningResourceBindingCache acts as a temporary buffer for ResourceBindings that have new scheduling
// decisions ("assigning" state) but are not yet fully reflected in the local Informer cache or member clusters.
type AssigningResourceBindingCache struct {
	sync.RWMutex
	items map[string]*workv1alpha2.ResourceBinding

	// assumptions stores per-binding, per-cluster resource assumptions for in-flight workloads.
	// The outer key is the binding key (namespace/name); the inner key (inside BindingAssumption.entries)
	// is the cluster name.
	assumptions map[string]*BindingAssumption
	// ttl is the duration after which a reservation is considered stale if no Healthy signal arrives.
	ttl time.Duration
}

// OnBindingUpdate is called when an update event for a ResourceBinding is received from the Informer.
// It removes the binding from this temporary cache if the Informer's version has caught up
// to or surpassed the version we recorded, indicating that the scheduling decision is now
// reflected in the Informer's state. This means the binding is no longer in the "assigning" state
// from the cache's perspective.
func (b *AssigningResourceBindingCache) OnBindingUpdate(binding *workv1alpha2.ResourceBinding) {
	b.Lock()
	defer b.Unlock()

	ca, ok := b.items[names.NamespacedKey(binding.Namespace, binding.Name)]
	if !ok {
		return
	}

	compare, err := resourceversion.CompareResourceVersion(ca.GetResourceVersion(), binding.GetResourceVersion())
	if err != nil {
		// should not happen since Kubernetes generates the resource versions, but log just in case
		klog.Errorf("Failed to compare resource versions for ResourceBinding %s/%s: %v", binding.Namespace, binding.Name, err)
		return
	}

	if compare == 1 {
		return
	}

	if meta.IsStatusConditionTrue(binding.Status.Conditions, workv1alpha2.FullyApplied) {
		delete(b.items, names.NamespacedKey(binding.Namespace, binding.Name))
		return
	}

	b.items[names.NamespacedKey(binding.Namespace, binding.Name)] = binding
}

// OnBindingDelete is called when a delete event for a ResourceBinding is received from the Informer.
func (b *AssigningResourceBindingCache) OnBindingDelete(binding *workv1alpha2.ResourceBinding) {
	b.Lock()
	defer b.Unlock()

	key := names.NamespacedKey(binding.Namespace, binding.Name)
	delete(b.items, key)
}

// Add records a ResourceBinding that has a new scheduling decision committed to the API server.
// This binding is considered to be in an "assigning" state and will be served by the scheduler
// as the intended state until its full reflection in the Informer cache and member clusters.
func (b *AssigningResourceBindingCache) Add(binding *workv1alpha2.ResourceBinding) {
	b.Lock()
	defer b.Unlock()

	b.items[names.NamespacedKey(binding.Namespace, binding.Name)] = binding
}

// GetBindings returns all currently cached ResourceBindings that are in the "assigning" state,
// providing the scheduler with the most up-to-date view of pending assignments.
func (b *AssigningResourceBindingCache) GetBindings() map[string]*workv1alpha2.ResourceBinding {
	b.RLock()
	defer b.RUnlock()

	return maps.Clone(b.items)
}

// Assume records (or replaces) the resource footprint for the given binding+cluster pair.
// Calling Assume again for the same pair overwrites the previous entry and resets the TTL,
// reflecting the latest scheduling decision.
// entry.Components is deep-copied before storage to prevent callers from unintentionally
// mutating the cache's internal state via shared slice backing arrays.
func (b *AssigningResourceBindingCache) Assume(bindingKey, clusterName string, entry AssumedWorkload) {
	b.Lock()
	defer b.Unlock()

	br, ok := b.assumptions[bindingKey]
	if !ok {
		br = &BindingAssumption{entries: make(map[string]AssumedWorkload)}
		b.assumptions[bindingKey] = br
	}
	copied := make([]workv1alpha2.Component, len(entry.Components))
	for i := range entry.Components {
		copied[i] = *entry.Components[i].DeepCopy()
	}
	entry.Components = copied
	br.entries[clusterName] = entry
	br.ExpiresAt = time.Now().Add(b.ttl)
}

// ReleaseClusterAssumption removes the reservation for a specific cluster within a binding.
// It is called when the workload on that cluster has reached a Healthy state.
// If the binding has no remaining cluster assumptions after the removal, the binding entry
// itself is also deleted.
func (b *AssigningResourceBindingCache) ReleaseClusterAssumption(bindingKey, clusterName string) {
	b.Lock()
	defer b.Unlock()

	br, ok := b.assumptions[bindingKey]
	if !ok {
		return
	}
	delete(br.entries, clusterName)
	if len(br.entries) == 0 {
		delete(b.assumptions, bindingKey)
	}
}

// ReleaseAssumption removes all cluster assumptions for a given binding at once.
// It is used when a binding is fully rescheduled or deleted.
func (b *AssigningResourceBindingCache) ReleaseAssumption(bindingKey string) {
	b.Lock()
	defer b.Unlock()

	delete(b.assumptions, bindingKey)
}

// GetAssumedWorkloads returns the reserved workloads for a given cluster across all bindings.
// Only assumptions that have not yet expired are included; expired entries are skipped
// (they will be cleaned up by the next GC sweep).
// Each returned AssumedWorkload is a deep copy, so callers may safely read or modify
// the result without affecting the cache's internal state.
func (b *AssigningResourceBindingCache) GetAssumedWorkloads(clusterName string) []AssumedWorkload {
	b.RLock()
	defer b.RUnlock()

	now := time.Now()
	var result []AssumedWorkload
	for _, br := range b.assumptions {
		if now.After(br.ExpiresAt) {
			continue
		}
		if entry, ok := br.entries[clusterName]; ok {
			copied := make([]workv1alpha2.Component, len(entry.Components))
			for i := range entry.Components {
				copied[i] = *entry.Components[i].DeepCopy()
			}
			result = append(result, AssumedWorkload{
				Namespace:  entry.Namespace,
				Components: copied,
			})
		}
	}
	return result
}

// GC removes all assumptions whose TTL has expired.
// It is intended to be called periodically (e.g., every minute) as a safety net for
// assumptions whose normal release path (Healthy signal) was never triggered.
// Returns the number of binding entries that were removed.
func (b *AssigningResourceBindingCache) GC() int {
	b.Lock()
	defer b.Unlock()

	now := time.Now()
	removed := 0
	for key, br := range b.assumptions {
		if now.After(br.ExpiresAt) {
			delete(b.assumptions, key)
			removed++
		}
	}
	return removed
}
