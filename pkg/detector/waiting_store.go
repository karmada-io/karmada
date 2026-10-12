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

package detector

import (
	"maps"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

// waitingObjectStore keeps the minimum object data required by ResourceSelector matching.
//
// The primary map owns stable candidate entries whose label snapshots are immutable. Secondary indexes share pointers
// to those entries instead of duplicating the full ClusterWideKey, and all indexes are updated atomically under lock.
type waitingObjectStore struct {
	lock sync.RWMutex

	// objects is the source of truth. Keeping ClusterWideKey as the map key preserves value-based direct lookup, while
	// the stable candidate pointer is reused by every secondary index and survives label snapshot replacement.
	objects map[keys.ClusterWideKey]*waitingCandidate
	// byGVK serves selectors without namespace or name constraints. Its buckets can be large, so a pointer set avoids
	// repeating ClusterWideKey values while preserving constant-time deletion.
	byGVK map[schema.GroupVersionKind]sets.Set[*waitingCandidate]
	// byScope serves namespaced selectors without an exact name. It uses the same pointer-set representation because
	// a namespace bucket can contain many waiting objects and objects are removed individually after policy matching.
	byScope map[waitingScopeKey]sets.Set[*waitingCandidate]
	// byGVKName serves name-only selectors across namespaces. These buckets are usually singletons and Match must
	// iterate all entries anyway, so a slice avoids allocating one Set map per unique name. The primary map prevents
	// duplicate insertion; deletion scans only resources sharing the same GVK and name. Cluster-scoped objects are
	// resolved directly from objects and therefore do not enter this index.
	byGVKName map[waitingGVKNameKey][]*waitingCandidate
}

type waitingObject struct {
	// labels is cloned on insertion and never mutated. Label updates replace the waitingCandidate's object pointer,
	// which makes a snapshot captured under the store lock safe to read after the lock has been released.
	labels map[string]string
}

type waitingScopeKey struct {
	gvk       schema.GroupVersionKind
	namespace string
}

type waitingGVKNameKey struct {
	gvk  schema.GroupVersionKind
	name string
}

type compiledWaitingSelector struct {
	gvk           schema.GroupVersionKind
	namespace     string
	name          string
	labelSelector labels.Selector
}

type waitingNameSelectorKey struct {
	gvk       schema.GroupVersionKind
	namespace string
	name      string
}

type waitingLabelSelectorGroup struct {
	matchAll  bool
	selectors map[string]labels.Selector
}

type waitingGVKMatchPlan struct {
	gvk         schema.GroupVersionKind
	global      *waitingLabelSelectorGroup
	byNamespace map[string]*waitingLabelSelectorGroup
}

type waitingMatchPlan struct {
	byName map[waitingNameSelectorKey]struct{}
	byGVK  map[schema.GroupVersionKind]*waitingGVKMatchPlan
}

type waitingCandidate struct {
	// key is immutable for the lifetime of this candidate. The stable candidate pointer is shared by the primary map
	// and secondary indexes, while candidates() returns value snapshots for lock-free selector evaluation.
	key    keys.ClusterWideKey
	object *waitingObject
}

func newWaitingObjectStore() *waitingObjectStore {
	return &waitingObjectStore{
		objects:   make(map[keys.ClusterWideKey]*waitingCandidate),
		byGVK:     make(map[schema.GroupVersionKind]sets.Set[*waitingCandidate]),
		byScope:   make(map[waitingScopeKey]sets.Set[*waitingCandidate]),
		byGVKName: make(map[waitingGVKNameKey][]*waitingCandidate),
	}
}

// Upsert inserts key into the primary map and every secondary index, or replaces its immutable label snapshot.
// The return values let resource reconciliation retry only when policy matching could observe new state.
func (s *waitingObjectStore) Upsert(key keys.ClusterWideKey, objectLabels map[string]string) (inserted bool, labelsChanged bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if candidate, exists := s.objects[key]; exists {
		if !maps.Equal(candidate.object.labels, objectLabels) {
			candidate.object = &waitingObject{labels: maps.Clone(objectLabels)}
			return false, true
		}
		return false, false
	}

	candidate := &waitingCandidate{
		key:    key,
		object: &waitingObject{labels: maps.Clone(objectLabels)},
	}
	s.objects[key] = candidate
	gvk := key.GroupVersionKind()
	insertWaitingIndex(s.byGVK, gvk, candidate)
	if key.Namespace != "" {
		insertWaitingIndex(s.byScope, waitingScopeKey{gvk: gvk, namespace: key.Namespace}, candidate)
		insertWaitingNameIndex(s.byGVKName, waitingGVKNameKey{gvk: gvk, name: key.Name}, candidate)
	}
	return true, false
}

func (s *waitingObjectStore) Contains(key keys.ClusterWideKey) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	_, exists := s.objects[key]
	return exists
}

func (s *waitingObjectStore) Delete(key keys.ClusterWideKey) {
	s.lock.Lock()
	defer s.lock.Unlock()

	candidate, exists := s.objects[key]
	if !exists {
		return
	}

	gvk := key.GroupVersionKind()
	deleteWaitingIndex(s.byGVK, gvk, candidate)
	if key.Namespace != "" {
		deleteWaitingIndex(s.byScope, waitingScopeKey{gvk: gvk, namespace: key.Namespace}, candidate)
		deleteWaitingNameIndex(s.byGVKName, waitingGVKNameKey{gvk: gvk, name: key.Name}, candidate)
	}
	delete(s.objects, key)
}

// Match returns the union of waiting object keys selected by resourceSelectors.
// Exact-name selectors use direct or indexed lookup. Other selectors are grouped into a match plan so each applicable
// GVK or namespace bucket is snapshotted once, even when a policy contains many overlapping selectors. Matching is
// candidate-first within each snapshot and stops after the first matching selector, preserving ResourceMatches
// semantics without multiplying candidate-slice allocations by the number of selectors.
func (s *waitingObjectStore) Match(resourceSelectors []policyv1alpha1.ResourceSelector) []keys.ClusterWideKey {
	plan := buildWaitingMatchPlan(compileWaitingSelectors(resourceSelectors))
	matched := sets.New[keys.ClusterWideKey]()

	for selector := range plan.byName {
		candidates := s.candidates(compiledWaitingSelector{
			gvk: selector.gvk, namespace: selector.namespace, name: selector.name,
		})
		for _, candidate := range candidates {
			matched.Insert(candidate.key)
		}
	}
	for _, gvkPlan := range plan.byGVK {
		s.matchGVKPlan(gvkPlan, matched)
	}

	return matched.UnsortedList()
}

func (s *waitingObjectStore) matchGVKPlan(plan *waitingGVKMatchPlan, matched sets.Set[keys.ClusterWideKey]) {
	if plan.global != nil {
		// A GVK-wide selector already requires the complete GVK bucket. Namespace selectors are evaluated against the
		// same snapshot so overlapping scopes do not copy and scan their namespace buckets a second time.
		candidates := s.candidates(compiledWaitingSelector{gvk: plan.gvk})
		for _, candidate := range candidates {
			if plan.global.matches(candidate.object.labels) {
				matched.Insert(candidate.key)
				continue
			}
			if namespaceGroup := plan.byNamespace[candidate.key.Namespace]; namespaceGroup != nil && namespaceGroup.matches(candidate.object.labels) {
				matched.Insert(candidate.key)
			}
		}
		return
	}

	for namespace, selectorGroup := range plan.byNamespace {
		candidates := s.candidates(compiledWaitingSelector{gvk: plan.gvk, namespace: namespace})
		for _, candidate := range candidates {
			if selectorGroup.matches(candidate.object.labels) {
				matched.Insert(candidate.key)
			}
		}
	}
}

func (s *waitingObjectStore) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.objects)
}

func (s *waitingObjectStore) candidates(selector compiledWaitingSelector) []waitingCandidate {
	s.lock.RLock()
	defer s.lock.RUnlock()

	switch {
	case selector.name != "":
		// All name selectors first use the complete key. With a namespace, a miss is final. Without a namespace, this
		// directly resolves cluster-scoped resources before falling back to the cross-namespace name index.
		key := keys.ClusterWideKey{
			Group:     selector.gvk.Group,
			Version:   selector.gvk.Version,
			Kind:      selector.gvk.Kind,
			Namespace: selector.namespace,
			Name:      selector.name,
		}
		if candidate, exists := s.objects[key]; exists {
			return []waitingCandidate{snapshotWaitingCandidate(candidate)}
		}
		if selector.namespace != "" {
			return nil
		}

		index := s.byGVKName[waitingGVKNameKey{gvk: selector.gvk, name: selector.name}]
		candidates := make([]waitingCandidate, 0, len(index))
		for _, candidate := range index {
			candidates = append(candidates, snapshotWaitingCandidate(candidate))
		}
		return candidates
	case selector.namespace != "":
		return snapshotWaitingCandidateSet(s.byScope[waitingScopeKey{gvk: selector.gvk, namespace: selector.namespace}])
	default:
		return snapshotWaitingCandidateSet(s.byGVK[selector.gvk])
	}
}

// snapshotWaitingCandidate copies the immutable key and the current immutable object pointer while the store lock is
// held. Upsert may replace candidate.object after the lock is released, but it never mutates the captured object.
func snapshotWaitingCandidate(candidate *waitingCandidate) waitingCandidate {
	return waitingCandidate{key: candidate.key, object: candidate.object}
}

func snapshotWaitingCandidateSet(index sets.Set[*waitingCandidate]) []waitingCandidate {
	candidates := make([]waitingCandidate, 0, len(index))
	for candidate := range index {
		candidates = append(candidates, snapshotWaitingCandidate(candidate))
	}
	return candidates
}

// compileWaitingSelectors parses selector fields once per Match call.
// ResourceMatches treats Name as authoritative when it is present, so a label selector accompanying Name is
// intentionally ignored to preserve the existing matching semantics.
func compileWaitingSelectors(resourceSelectors []policyv1alpha1.ResourceSelector) []compiledWaitingSelector {
	selectors := make([]compiledWaitingSelector, 0, len(resourceSelectors))
	for _, resourceSelector := range resourceSelectors {
		groupVersion, err := schema.ParseGroupVersion(resourceSelector.APIVersion)
		if err != nil {
			continue
		}

		selector := compiledWaitingSelector{
			gvk:       groupVersion.WithKind(resourceSelector.Kind),
			namespace: resourceSelector.Namespace,
			name:      resourceSelector.Name,
		}
		if resourceSelector.Name == "" && resourceSelector.LabelSelector != nil {
			selector.labelSelector, err = metav1.LabelSelectorAsSelector(resourceSelector.LabelSelector)
			if err != nil {
				continue
			}
		}
		selectors = append(selectors, selector)
	}
	return selectors
}

// buildWaitingMatchPlan groups selectors by the candidate indexes they use. Name selectors remain independent indexed
// lookups, while label and match-all selectors sharing a GVK or namespace reuse one candidate snapshot. Equivalent
// selectors are deduplicated, and a match-all selector subsumes every other selector in the same group.
func buildWaitingMatchPlan(selectors []compiledWaitingSelector) waitingMatchPlan {
	plan := waitingMatchPlan{
		byName: make(map[waitingNameSelectorKey]struct{}),
		byGVK:  make(map[schema.GroupVersionKind]*waitingGVKMatchPlan),
	}
	for _, selector := range selectors {
		if selector.name != "" {
			plan.byName[waitingNameSelectorKey{
				gvk: selector.gvk, namespace: selector.namespace, name: selector.name,
			}] = struct{}{}
			continue
		}

		gvkPlan := plan.byGVK[selector.gvk]
		if gvkPlan == nil {
			gvkPlan = &waitingGVKMatchPlan{
				gvk:         selector.gvk,
				byNamespace: make(map[string]*waitingLabelSelectorGroup),
			}
			plan.byGVK[selector.gvk] = gvkPlan
		}

		if selector.namespace == "" {
			if gvkPlan.global == nil {
				gvkPlan.global = &waitingLabelSelectorGroup{}
			}
			gvkPlan.global.add(selector.labelSelector)
			continue
		}

		selectorGroup := gvkPlan.byNamespace[selector.namespace]
		if selectorGroup == nil {
			selectorGroup = &waitingLabelSelectorGroup{}
			gvkPlan.byNamespace[selector.namespace] = selectorGroup
		}
		selectorGroup.add(selector.labelSelector)
	}
	return plan
}

func (g *waitingLabelSelectorGroup) add(selector labels.Selector) {
	if g.matchAll {
		return
	}
	if selector == nil || selector.Empty() {
		g.matchAll = true
		g.selectors = nil
		return
	}
	if g.selectors == nil {
		g.selectors = make(map[string]labels.Selector)
	}
	g.selectors[selector.String()] = selector
}

func (g *waitingLabelSelectorGroup) matches(objectLabels map[string]string) bool {
	if g.matchAll {
		return true
	}
	labelSet := labels.Set(objectLabels)
	for _, selector := range g.selectors {
		if selector.Matches(labelSet) {
			return true
		}
	}
	return false
}

func insertWaitingIndex[T comparable](index map[T]sets.Set[*waitingCandidate], bucket T, candidate *waitingCandidate) {
	candidatesInBucket, exists := index[bucket]
	if !exists {
		candidatesInBucket = sets.New[*waitingCandidate]()
		index[bucket] = candidatesInBucket
	}
	candidatesInBucket.Insert(candidate)
}

func deleteWaitingIndex[T comparable](index map[T]sets.Set[*waitingCandidate], bucket T, candidate *waitingCandidate) {
	candidatesInBucket, exists := index[bucket]
	if !exists {
		return
	}
	candidatesInBucket.Delete(candidate)
	if len(candidatesInBucket) == 0 {
		delete(index, bucket)
	}
}

func insertWaitingNameIndex(index map[waitingGVKNameKey][]*waitingCandidate, bucket waitingGVKNameKey, candidate *waitingCandidate) {
	index[bucket] = append(index[bucket], candidate)
}

func deleteWaitingNameIndex(index map[waitingGVKNameKey][]*waitingCandidate, bucket waitingGVKNameKey, candidate *waitingCandidate) {
	candidatesInBucket := index[bucket]
	for i, indexedCandidate := range candidatesInBucket {
		if indexedCandidate != candidate {
			continue
		}

		last := len(candidatesInBucket) - 1
		candidatesInBucket[i] = candidatesInBucket[last]
		candidatesInBucket[last] = nil
		candidatesInBucket = candidatesInBucket[:last]
		if len(candidatesInBucket) == 0 {
			delete(index, bucket)
		} else {
			index[bucket] = candidatesInBucket
		}
		return
	}
}
