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

// waitingObjectSelector is a policy resource selector prepared for matching.
type waitingObjectSelector struct {
	gvk           schema.GroupVersionKind
	namespace     string
	name          string
	labelSelector labels.Selector
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
	selectors := compileWaitingObjectSelectors(resourceSelectors)
	matched := sets.New[keys.ClusterWideKey]()

	for _, selector := range selectors {
		s.matchSelector(selector, matched)
	}

	return matched.UnsortedList()
}

// matchSelector adds the waiting objects that match selector to matched.
func (s *waitingObjectStore) matchSelector(selector waitingObjectSelector, matched sets.Set[keys.ClusterWideKey]) {
	// A namespaced selector only needs to inspect that namespace's bucket.
	if selector.namespace != "" {
		s.matchBucket(selector, selector.namespace, matched)
		return
	}

	// All objects of a GVK share the same scope. A waiting object in the
	// empty-namespace bucket therefore means the GVK is cluster-scoped, and the
	// namespace buckets cannot hold anything this selector may match.
	if objects := s.getObjectsBucket(selector.gvk, ""); notEmpty(objects) {
		s.matchBucket(selector, "", matched)
		return
	}

	// The GVK is namespaced (or has no waiting object yet). A selector without
	// a namespace matches objects in any namespace, so every namespace bucket
	// has to be inspected. Even then, a named selector resolves with a point
	// lookup per namespace instead of scanning the objects.
	namespaces, exists := s.cache.Load(selector.gvk)
	if !exists {
		return
	}

	namespaces.(*sync.Map).Range(func(namespace, _ any) bool {
		s.matchBucket(selector, namespace.(string), matched)
		return true
	})
}

// matchBucket adds the objects of the given namespace bucket that match selector to matched.
func (s *waitingObjectStore) matchBucket(
	selector waitingObjectSelector,
	namespace string,
	matched sets.Set[keys.ClusterWideKey],
) {
	objects := s.getObjectsBucket(selector.gvk, namespace)
	if objects == nil {
		return
	}

	// The bucket is already scoped by GVK and namespace, so the name is the
	// only remaining part of an object's identity and is used as the index key.
	if selector.name != "" {
		if _, exists := objects.Load(selector.name); exists {
			matched.Insert(waitingObjectKey(selector.gvk, namespace, selector.name))
		}
		return
	}

	// Range can observe an entry concurrently removed from or replaced in the
	// indexes. Matching only schedules another resource reconciliation, which
	// validates the current object and policy state before taking action.
	objects.Range(func(name, objectValue any) bool {
		if selector.labelSelector != nil &&
			!selector.labelSelector.Matches(labels.Set(objectValue.(map[string]string))) {
			return true
		}
		matched.Insert(waitingObjectKey(selector.gvk, namespace, name.(string)))
		return true
	})
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

// compileWaitingObjectSelectors converts policy resource selectors into the
// form used for matching, dropping invalid ones.
func compileWaitingObjectSelectors(resourceSelectors []policyv1alpha1.ResourceSelector) []waitingObjectSelector {
	selectors := make([]waitingObjectSelector, 0, len(resourceSelectors))
	for _, resourceSelector := range resourceSelectors {
		groupVersion, err := schema.ParseGroupVersion(resourceSelector.APIVersion)
		if err != nil {
			continue
		}

		selector := waitingObjectSelector{
			gvk:       groupVersion.WithKind(resourceSelector.Kind),
			namespace: resourceSelector.Namespace,
			name:      resourceSelector.Name,
		}
		if selector.name == "" && resourceSelector.LabelSelector != nil {
			selector.labelSelector, err = metav1.LabelSelectorAsSelector(resourceSelector.LabelSelector)
			if err != nil {
				continue
			}
		}
		selectors = append(selectors, selector)
	}
	return selectors
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
