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
	"slices"
	"sync"
	"sync/atomic"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

// waitingObjectStore keeps the identity and labels of the objects that are
// waiting for a propagation policy, so they can be matched against resource
// selectors. Its zero value is ready for use.
type waitingObjectStore struct {
	// count is the total number of objects across all buckets.
	count atomic.Int64
	// cache maps GVK -> namespace -> name -> labels. Empty buckets are kept so
	// readers and writers get stable map pointers without locking; the retained
	// structure stays around 600 KiB even with 1,000 GVK/namespace pairs.
	cache sync.Map
}

// waitingObjectSelector is a group of compiled selectors that share a GVK and
// namespace. Merging them lets one walk over the objects handle all of them.
type waitingObjectSelector struct {
	gvk       schema.GroupVersionKind
	namespace string
	// matchAll means the merged selectors select every object in scope, so
	// names and labels cannot narrow the result any further.
	matchAll bool
	names    []string
	labels   []labels.Selector
}

// Store adds the object identified by key or updates its labels, and returns
// whether the object was added and the total number of waiting objects.
func (s *waitingObjectStore) Store(key keys.ClusterWideKey, object *unstructured.Unstructured) (bool, int) {
	objects := s.getOrCreateObjectsBucket(key.GroupVersionKind(), key.Namespace)
	if _, exists := objects.Swap(key.Name, object.GetLabels()); exists {
		return false, int(s.count.Load())
	}
	return true, int(s.count.Add(1))
}

// Delete removes the object identified by key from the store.
func (s *waitingObjectStore) Delete(key keys.ClusterWideKey) {
	objects := s.getObjectsBucket(key.GroupVersionKind(), key.Namespace)
	if objects == nil {
		return
	}
	if _, exists := objects.LoadAndDelete(key.Name); !exists {
		return
	}
	s.count.Add(-1)
}

// Len returns the number of objects in the store.
func (s *waitingObjectStore) Len() int {
	return int(s.count.Load())
}

// notEmpty reports whether the bucket contains at least one object.
// Empty buckets are retained to keep bucket pointers stable, so a non-nil
// bucket is not enough to tell whether it has entries.
func notEmpty(objects *sync.Map) bool {
	if objects == nil {
		return false
	}

	found := false
	objects.Range(func(_, _ any) bool {
		found = true
		return false
	})
	return found
}

// Match returns the keys of the waiting objects that match any of the given
// resource selectors.
func (s *waitingObjectStore) Match(resourceSelectors []policyv1alpha1.ResourceSelector) []keys.ClusterWideKey {
	matched := sets.New[keys.ClusterWideKey]()
	for _, selector := range compileWaitingObjectSelectors(resourceSelectors) {
		s.matchSelector(selector, matched)
	}

	return matched.UnsortedList()
}

// matchSelector adds the waiting objects selected by matcher to matched.
func (s *waitingObjectStore) matchSelector(selector waitingObjectSelector, matched sets.Set[keys.ClusterWideKey]) {
	for _, name := range selector.names {
		s.matchName(selector.gvk, selector.namespace, name, matched)
	}
	if !selector.matchAll && len(selector.labels) == 0 {
		return
	}

	// A namespaced matcher only needs to inspect that namespace's bucket.
	if selector.namespace != "" {
		s.matchLabelBucket(selector, selector.namespace, matched)
		return
	}

	// All objects of a GVK share the same scope, so a waiting object in the
	// empty-namespace bucket means the GVK is cluster-scoped.
	if objects := s.getObjectsBucket(selector.gvk, ""); notEmpty(objects) {
		s.matchLabelBucket(selector, "", matched)
		return
	}

	// The GVK is namespaced (or has no waiting object yet), so a matcher
	// without a namespace has to inspect every namespace bucket.
	namespaces, exists := s.cache.Load(selector.gvk)
	if !exists {
		return
	}
	namespaces.(*sync.Map).Range(func(namespace, _ any) bool {
		s.matchLabelBucket(selector, namespace.(string), matched)
		return true
	})
}

// matchName adds the waiting object of the given name to matched.
func (s *waitingObjectStore) matchName(gvk schema.GroupVersionKind, namespace, name string, matched sets.Set[keys.ClusterWideKey]) {
	if namespace != "" {
		objects := s.getObjectsBucket(gvk, namespace)
		if objects == nil {
			return
		}
		if _, exists := objects.Load(name); exists {
			matched.Insert(waitingObjectKey(gvk, namespace, name))
		}
		return
	}

	// A waiting object in the empty-namespace bucket means the GVK is
	// cluster-scoped, so the namespace buckets cannot hold a match.
	if objects := s.getObjectsBucket(gvk, ""); notEmpty(objects) {
		if _, exists := objects.Load(name); exists {
			matched.Insert(waitingObjectKey(gvk, "", name))
		}
		return
	}

	namespaces, exists := s.cache.Load(gvk)
	if !exists {
		return
	}
	namespaces.(*sync.Map).Range(func(namespace, objects any) bool {
		if _, exists := objects.(*sync.Map).Load(name); exists {
			matched.Insert(waitingObjectKey(gvk, namespace.(string), name))
		}
		return true
	})
}

// matchLabelBucket adds the objects of the namespace bucket selected by the
// matcher to matched.
func (s *waitingObjectStore) matchLabelBucket(matcher waitingObjectSelector, namespace string, matched sets.Set[keys.ClusterWideKey]) {
	objects := s.getObjectsBucket(matcher.gvk, namespace)
	if objects == nil {
		return
	}

	// Range can observe an entry concurrently removed from or replaced in the
	// indexes. Matching only schedules another resource reconciliation, which
	// validates the current object and policy state before taking action.
	objects.Range(func(name, objectValue any) bool {
		if matcher.matchAll || matcher.matchesLabels(objectValue.(map[string]string)) {
			matched.Insert(waitingObjectKey(matcher.gvk, namespace, name.(string)))
		}
		return true
	})
}

// matchesLabels reports whether any label selector of the matcher matches.
func (m waitingObjectSelector) matchesLabels(objectLabels map[string]string) bool {
	labelSet := labels.Set(objectLabels)
	for _, selector := range m.labels {
		if selector.Matches(labelSet) {
			return true
		}
	}
	return false
}

// waitingObjectKey builds the key of an object inside the bucket of the given
// GVK and namespace.
func waitingObjectKey(gvk schema.GroupVersionKind, namespace, name string) keys.ClusterWideKey {
	return keys.ClusterWideKey{
		Group:     gvk.Group,
		Version:   gvk.Version,
		Kind:      gvk.Kind,
		Namespace: namespace,
		Name:      name,
	}
}

// add merges a compiled selector into the group. A match-all selector absorbs
// everything of the same scope, so later selectors cannot narrow the result.
func (m *waitingObjectSelector) add(name string, labelSelector labels.Selector) {
	switch {
	case m.matchAll:
		// The scope is already fully selected.
	case name != "":
		if !slices.Contains(m.names, name) {
			m.names = append(m.names, name)
		}
	case labelSelector == nil || labelSelector.Empty():
		m.matchAll = true
		m.names = nil
		m.labels = nil
	default:
		if !slices.ContainsFunc(m.labels, func(existing labels.Selector) bool {
			return existing.String() == labelSelector.String()
		}) {
			m.labels = append(m.labels, labelSelector)
		}
	}
}

// compileWaitingObjectSelectors compiles resource selectors and merges the ones
// that share a GVK and namespace. A match-all selector absorbs every other
// selector of the same scope, so matching only has to walk each group of
// objects once.
func compileWaitingObjectSelectors(resourceSelectors []policyv1alpha1.ResourceSelector) []waitingObjectSelector {
	byGVK := make(map[schema.GroupVersionKind]map[string]*waitingObjectSelector, len(resourceSelectors))
	for _, resourceSelector := range resourceSelectors {
		groupVersion, err := schema.ParseGroupVersion(resourceSelector.APIVersion)
		if err != nil {
			continue
		}

		gvk := groupVersion.WithKind(resourceSelector.Kind)
		namespace := resourceSelector.Namespace
		name := resourceSelector.Name
		var labelSelector labels.Selector
		if name == "" && resourceSelector.LabelSelector != nil {
			labelSelector, err = metav1.LabelSelectorAsSelector(resourceSelector.LabelSelector)
			if err != nil {
				continue
			}
		}

		namespaces := byGVK[gvk]
		if namespaces == nil {
			namespaces = make(map[string]*waitingObjectSelector)
			byGVK[gvk] = namespaces
		}
		selector := namespaces[namespace]
		if selector == nil {
			selector = &waitingObjectSelector{gvk: gvk, namespace: namespace}
			namespaces[namespace] = selector
		}

		selector.add(name, labelSelector)
	}

	matchers := make([]waitingObjectSelector, 0, len(resourceSelectors))
	for _, namespaces := range byGVK {
		if global := namespaces[""]; global != nil && global.matchAll {
			matchers = append(matchers, *global)
			continue
		}
		for _, matcher := range namespaces {
			matchers = append(matchers, *matcher)
		}
	}
	return matchers
}

// getObjectsBucket returns the bucket of the given GVK and namespace, or nil
// if no object has been stored for them.
func (s *waitingObjectStore) getObjectsBucket(gvk schema.GroupVersionKind, namespace string) *sync.Map {
	namespaces, exists := s.cache.Load(gvk)
	if !exists {
		return nil
	}
	objects, exists := namespaces.(*sync.Map).Load(namespace)
	if !exists {
		return nil
	}
	return objects.(*sync.Map)
}

// getOrCreateObjectsBucket returns the bucket of the given GVK and namespace,
// creating it if necessary.
func (s *waitingObjectStore) getOrCreateObjectsBucket(gvk schema.GroupVersionKind, namespace string) *sync.Map {
	namespaces, _ := s.cache.LoadOrStore(gvk, &sync.Map{})
	objects, _ := namespaces.(*sync.Map).LoadOrStore(namespace, &sync.Map{})
	return objects.(*sync.Map)
}
