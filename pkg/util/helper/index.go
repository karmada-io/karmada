/*
Copyright 2024 The Karmada Authors.

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

package helper

import (
	"context"
	"sync"

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	// IndexNameForResourceBindingClusters used for building indexer for resourcebinding object based on the field .spec.clusters
	IndexNameForResourceBindingClusters = "resourcebinding.spec.clusters"
	// IndexNameForClusterResourceBindingClusters used for building indexer for clusterresourcebinding object based on the field .spec.clusters
	IndexNameForClusterResourceBindingClusters = "clusterresourcebinding.spec.clusters"
)

var (
	registerOnce sync.Once
)

// Options Optional when defining the registration indexer
type Options struct {
	// EnableTaintManager If set to true will register resourcebinding.spec.clusters and clusterresourcebinding.spec.clusters indexer
	EnableTaintManager bool
}

// RegisterFieldIndexes register all field indexes
func RegisterFieldIndexes(ctx context.Context, mgr ctrl.Manager, opt *Options) error {
	var err error
	registerOnce.Do(func() {
		if err = IndexWork(ctx, mgr); err != nil {
			return
		}
		if opt.EnableTaintManager {
			if err = IndexResourceBinding(ctx, mgr); err != nil {
				return
			}
			if err = IndexClusterResourceBinding(ctx, mgr); err != nil {
				return
			}
		}
	})
	return err
}

// IndexWork creates index for Work.
func IndexWork(ctx context.Context, mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha1.Work{}, workv1alpha2.ResourceBindingPermanentIDLabel,
		IndexerFuncBasedOnLabel(workv1alpha2.ResourceBindingPermanentIDLabel))
	if err != nil {
		klog.Errorf("failed to create index for work, err: %v", err)
		return err
	}
	err = mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha1.Work{}, workv1alpha2.ClusterResourceBindingPermanentIDLabel,
		IndexerFuncBasedOnLabel(workv1alpha2.ClusterResourceBindingPermanentIDLabel))
	if err != nil {
		klog.Errorf("failed to create index for work, err: %v", err)
		return err
	}
	return nil
}

// IndexerFuncBasedOnLabel returns an IndexerFunc used to index resource with the given key as label key.
func IndexerFuncBasedOnLabel(key string) client.IndexerFunc {
	return func(obj client.Object) []string {
		val, ok := obj.GetLabels()[key]
		if !ok {
			return nil
		}
		return []string{val}
	}
}

// IndexResourceBinding creates index for ResourceBinding.
func IndexResourceBinding(ctx context.Context, mgr ctrl.Manager) error {
	rbIndexerFunc := func(obj client.Object) []string {
		rb, ok := obj.(*workv1alpha2.ResourceBinding)
		if !ok {
			return nil
		}
		return util.GetBindingClusterNames(&rb.Spec)
	}
	err := mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha2.ResourceBinding{}, IndexNameForResourceBindingClusters, rbIndexerFunc)
	if err != nil {
		klog.Errorf("failed to create index for resourcebinding, err: %v", err)
		return err
	}
	return nil
}

// IndexClusterResourceBinding creates index for ResourceBinding.
func IndexClusterResourceBinding(ctx context.Context, mgr ctrl.Manager) error {
	crbIndexerFunc := func(obj client.Object) []string {
		crb, ok := obj.(*workv1alpha2.ClusterResourceBinding)
		if !ok {
			return nil
		}
		return util.GetBindingClusterNames(&crb.Spec)
	}
	err := mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha2.ClusterResourceBinding{}, IndexNameForClusterResourceBindingClusters, crbIndexerFunc)
	if err != nil {
		klog.Errorf("failed to create index for clusterresourcebinding, err: %v", err)
		return err
	}
	return nil
}
