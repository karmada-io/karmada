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
	"encoding/json"
	"sync"

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
)

const (
	// IndexNameForResourceBindingDependency index name for resourcebinding dependency used for fieldSelector
	IndexNameForResourceBindingDependency = "resourcebinding.dependency"
)

var (
	registerOnce sync.Once
)

// RegisterFieldIndexes register all field indexes
func RegisterFieldIndexes(ctx context.Context, mgr ctrl.Manager) error {
	var err error
	registerOnce.Do(func() {
		if err = IndexWork(ctx, mgr); err != nil {
			return
		}
		if err = IndexResourceBinding(ctx, mgr); err != nil {
			return
		}
	})
	return err
}

// IndexWork creates index for Work.
func IndexWork(ctx context.Context, mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha1.Work{}, indexregistry.WorkIndexByResourceBindingID,
		IndexerFuncBasedOnLabel(workv1alpha2.ResourceBindingPermanentIDLabel))
	if err != nil {
		klog.Errorf("failed to create index for work, err: %v", err)
		return err
	}
	err = mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha1.Work{}, indexregistry.WorkIndexByClusterResourceBindingID,
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
	err := mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha2.ResourceBinding{}, IndexNameForResourceBindingDependency, IndexFuncResourceBindingDependencies())
	if err != nil {
		klog.Errorf("failed to create index for resource.dependencies, err: %v", err)
		return err
	}
	return nil
}

// IndexFuncResourceBindingDependencies returns an IndexerFunc used to index resource with the given dependency resource.
func IndexFuncResourceBindingDependencies() client.IndexerFunc {
	return func(obj client.Object) []string {
		val, ok := obj.GetAnnotations()[workv1alpha2.DependenciesAnnotationKey]
		if !ok {
			return nil
		}
		var dependenciesSlice []configv1alpha1.DependentObjectReference
		err := json.Unmarshal([]byte(val), &dependenciesSlice)
		if err != nil {
			return nil
		}
		var dependencies []string
		for _, dependencyObj := range dependenciesSlice {
			dependencyObj.LabelSelector = nil
			dependencyObj.Name = ""
			dependenciesStr, err := json.Marshal(dependencyObj)
			if err != nil {
				continue
			}
			dependencies = append(dependencies, string(dependenciesStr))
		}
		return dependencies
	}
}
