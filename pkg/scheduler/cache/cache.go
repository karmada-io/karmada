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

type GroupType string

const (
	GroupTypeAffinity     GroupType = "Affinity"
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

	// groupPeers maps (ns, type, group) -> []bindingID
	groupPeers map[WorkloadGroupKey][]string

	// clustersByBinding maps bindingID -> set(clusterName) for scheduled bindings
	clustersByBinding map[string]sets.Set[string]
}

// NewCache instantiates a cache used only by scheduler.
func NewCache(clusterLister clusterlister.ClusterLister) Cache {
	return &schedulerCache{
		clusterLister:     clusterLister,
		groupPeers:        make(map[WorkloadGroupKey][]string),
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

		out.groupPeers = make(map[WorkloadGroupKey][]string, len(c.groupPeers))
		for k, v := range c.groupPeers {
			vv := make([]string, len(v))
			copy(vv, v)
			out.groupPeers[k] = vv
		}

		out.clustersByBinding = make(map[string]sets.Set[string], len(c.clustersByBinding))
		for id, set := range c.clustersByBinding {
			s := sets.New[string]()
			for item := range set {
				s.Insert(item)
			}
			out.clustersByBinding[id] = s
		}
	}

	return out
}

func (c *schedulerCache) OnResourceBindingAdd(obj interface{}) {
	info := getBindingInfoFromObj(obj)
	if info == nil {
		return
	}
	c.indexBinding(info)
}

func (c *schedulerCache) OnResourceBindingUpdate(oldObj, newObj interface{}) {
	oldInfo := getBindingInfoFromObj(oldObj)
	newInfo := getBindingInfoFromObj(newObj)
	if oldInfo == nil || newInfo == nil {
		return
	}

	c.unindexBinding(oldInfo)
	c.indexBinding(newInfo)
}

func (c *schedulerCache) OnResourceBindingDelete(obj interface{}) {
	info := getBindingInfoFromObj(obj)
	if info == nil {
		return
	}
	c.unindexBinding(info)
}

type bindingInfo struct {
	id                string
	namespace         string // "" for CRB
	clusters          sets.Set[string]
	affinityGroup     string
	antiAffinityGroup string
}

// indexBinding indexes BOTH affinity and anti-affinity groups (if present).
// Only indexes bindings that are already scheduled (spec.clusters non-empty).
func (c *schedulerCache) indexBinding(b *bindingInfo) {
	if b.clusters.Len() == 0 {
		return
	}

	// If controller didn't populate groups, nothing to index.
	if b.affinityGroup == "" && b.antiAffinityGroup == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Record clusters for this binding ID (used by scheduler when resolving peers).
	c.clustersByBinding[b.id] = b.clusters

	if b.affinityGroup != "" {
		k := makeWorkloadGroupKey(b.namespace, GroupTypeAffinity, b.affinityGroup)
		c.groupPeers[k] = append(c.groupPeers[k], b.id)
	}

	if b.antiAffinityGroup != "" {
		k := makeWorkloadGroupKey(b.namespace, GroupTypeAntiAffinity, b.antiAffinityGroup)
		c.groupPeers[k] = append(c.groupPeers[k], b.id)
	}
}

func (c *schedulerCache) unindexBinding(b *bindingInfo) {
	// If controller didn't populate groups, nothing to unindex.
	if b.affinityGroup == "" && b.antiAffinityGroup == "" {
		// still remove cluster mapping in case it existed
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.clustersByBinding, b.id)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if b.affinityGroup != "" {
		k := makeWorkloadGroupKey(b.namespace, GroupTypeAffinity, b.affinityGroup)
		c.groupPeers[k] = removeString(c.groupPeers[k], b.id)
		if len(c.groupPeers[k]) == 0 {
			delete(c.groupPeers, k)
		}
	}

	if b.antiAffinityGroup != "" {
		k := makeWorkloadGroupKey(b.namespace, GroupTypeAntiAffinity, b.antiAffinityGroup)
		c.groupPeers[k] = removeString(c.groupPeers[k], b.id)
		if len(c.groupPeers[k]) == 0 {
			delete(c.groupPeers, k)
		}
	}

	delete(c.clustersByBinding, b.id)
}

func removeString(slice []string, target string) []string {
	if len(slice) == 0 {
		return slice
	}
	out := slice[:0]
	for _, s := range slice {
		if s != target {
			out = append(out, s)
		}
	}
	return out
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
