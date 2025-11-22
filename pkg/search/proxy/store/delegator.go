/*
Copyright 2025 The Karmada Authors.

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

package store

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage"
	cacherstorage "k8s.io/apiserver/pkg/storage/cacher"
)

// NewCacheDelegator returns a CacheDelegator
func NewCacheDelegator(cacher *cacherstorage.Cacher, storage storage.Interface) *CacheDelegator {
	return &CacheDelegator{
		storage:        storage,
		CacheDelegator: cacherstorage.NewCacheDelegator(cacher, storage),
	}
}

// CacheDelegator overwrite GetList method of cacherstorage.CacheDelegator.
// APIServer in member cluster don't implement storage.RequestWatchProgress, so the cached resource version fails to
// synchronize with the member cluster in a timely manner, resulting in the error "Too large resource version" in
// some List requests. We force delegate these list requests to storage (member cluster). See ShouldDelegateList for detail.
type CacheDelegator struct {
	storage storage.Interface
	*cacherstorage.CacheDelegator
}

// GetList implements storage.Interface.
func (c *CacheDelegator) GetList(ctx context.Context, key string, opts storage.ListOptions, listObj runtime.Object) error {
	if ShouldDelegateList(opts) {
		return c.storage.GetList(ctx, key, opts, listObj)
	}
	return c.CacheDelegator.GetList(ctx, key, opts, listObj)
}

// ShouldDelegateList force delegate list request to storage when:
// * ResourceVersion is not "0"
// * Limit is set
// * Continue is set
func ShouldDelegateList(opts storage.ListOptions) bool {
	// see https://kubernetes.io/docs/reference/using-api/api-concepts/#semantics-for-get-and-list
	return opts.ResourceVersion != "0" || opts.Predicate.Limit > 0 || opts.Predicate.Continue != ""
}
