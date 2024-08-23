/*
Copyright The Karmada Authors.

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

package v1alpha1

import (
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
)

type mockIndexer struct {
	cache.Indexer
}

func (m *mockIndexer) GetByKey(string) (interface{}, bool, error) {
	return nil, false, errors.New("mock error")
}

func TestResourceRegistryLister_List(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	lister := NewResourceRegistryLister(indexer)

	registries, err := lister.List(labels.Everything())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(registries) != 0 {
		t.Fatalf("expected empty list, got %v", registries)
	}

	registry := &v1alpha1.ResourceRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}
	err = indexer.Add(registry)
	if err != nil {
		return
	}

	registries, err = lister.List(labels.Everything())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(registries) != 1 {
		t.Fatalf("expected list of length 1, got %v", len(registries))
	}
}

func TestResourceRegistryLister_Get(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	lister := NewResourceRegistryLister(indexer)

	_, err := lister.Get("non-existent")
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected NotFound error, got %v", err)
	}

	registry := &v1alpha1.ResourceRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}
	err = indexer.Add(registry)
	if err != nil {
		return
	}

	obj, err := lister.Get("default")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if obj == nil {
		t.Fatalf("expected a ResourceRegistry, got nil")
	}

	mockIndexer := &mockIndexer{Indexer: indexer}
	listerWithMock := NewResourceRegistryLister(mockIndexer)

	_, err = listerWithMock.Get("default")
	if err == nil || err.Error() != "mock error" {
		t.Fatalf("expected mock error, got %v", err)
	}
}
