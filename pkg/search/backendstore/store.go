/*
Copyright 2022 The Karmada Authors.

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

package backendstore

import (
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
)

// BackendStore define BackendStore interface
type BackendStore interface {
	ResourceEventHandlerFuncs() cache.ResourceEventHandler
	Close()
}

var (
	backendLock sync.Mutex
	backends    map[string]BackendStore
	k8sClient   *kubernetes.Clientset
)

// Init init backend store manager
func Init(cs *kubernetes.Clientset) {
	backendLock.Lock()
	backends = make(map[string]BackendStore)
	k8sClient = cs
	backendLock.Unlock()
}

// AddBackend add backend store
func AddBackend(cluster string, cfg *searchv1alpha1.BackendStoreConfig) {
	backendLock.Lock()
	defer backendLock.Unlock()

	if cfg == nil || cfg.OpenSearch == nil {
		backends[cluster] = NewDefaultBackend(cluster)
		return
	}

	bs, err := NewOpenSearch(cluster, cfg)
	if err != nil {
		klog.Errorf("cannot create backend store %s: %v, use default backend store", cluster, err)
		return
	}
	backends[cluster] = bs
}

// DeleteBackend delete backend store
func DeleteBackend(cluster string) {
	backendLock.Lock()
	defer backendLock.Unlock()

	bs, ok := backends[cluster]
	if ok {
		bs.Close()
		delete(backends, cluster)
	}
}

// GetBackend get backend store
func GetBackend(cluster string) BackendStore {
	backendLock.Lock()
	defer backendLock.Unlock()
	bs, ok := backends[cluster]
	if !ok {
		return nil
	}
	return bs
}
