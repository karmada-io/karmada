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
func NewCache(clusterLister clusterlister.ClusterLister, resourceBindingIndexer cache.Indexer) Cache {
	return &schedulerCache{
		clusterLister:          clusterLister,
		resourceBindingIndexer: resourceBindingIndexer,
		assigningResourceBindings: &AssigningResourceBindingCache{
			items: make(map[string]*workv1alpha2.ResourceBinding),
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

	delete(b.items, names.NamespacedKey(binding.Namespace, binding.Name))
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
