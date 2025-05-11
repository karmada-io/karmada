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

package indexregistry

import (
	"context"
	"sync"

	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	// WorkIndexByLabelResourceBindingID is the index name for Works that are associated with a ResourceBinding ID.
	// This index allows efficient lookup of Works by their `metadata.labels.<ResourceBindingPermanentIDLabel>`,
	// which references the ID of the bound ResourceBinding object.
	WorkIndexByLabelResourceBindingID = "WorkIndexByLabelResourceBindingID"

	// WorkIndexByLabelClusterResourceBindingID is the index name for Works associated with a ClusterResourceBinding ID.
	// The index is built using `metadata.labels.<ClusterResourceBindingPermanentIDLabel>` to enable fast queries
	// of Works linked to specific ClusterResourceBinding objects across all namespaces.
	WorkIndexByLabelClusterResourceBindingID = "WorkIndexByLabelClusterResourceBindingID"

	// WorkIndexByFieldSuspendDispatching is the index name used for Works that are suspended from dispatching.
	// This index is built based on the `spec.suspendDispatching` field of each Work object.
	WorkIndexByFieldSuspendDispatching = "WorkIndexByFieldSuspendDispatching"

	// ResourceBindingIndexByFieldCluster is the index name used for ResourceBindings associated with specific clusters.
	// The index is constructed from the cluster names listed in the `spec.clusters` field of each ResourceBinding,
	// enabling efficient lookups of ResourceBindings targeting particular clusters.
	ResourceBindingIndexByFieldCluster = "ResourceBindingIndexByFieldCluster"

	// ClusterResourceBindingIndexByFieldCluster is the index name used for ClusterResourceBindings associated with specific clusters.
	// The index is constructed from the cluster names listed in the `spec.clusters` field of each ClusterResourceBindings,
	// enabling efficient lookups of ClusterResourceBindings targeting particular clusters.
	ClusterResourceBindingIndexByFieldCluster = "ClusterResourceBindingIndexByFieldCluster"
)

var (
	registerWorkIndexByLabelResourceBindingIDOnce        sync.Once
	registerWorkIndexByLabelClusterResourceBindingIDOnce sync.Once
	registerWorkIndexByFieldSuspendDispatchingOnce       sync.Once
	registerRBIndexByFieldClusterOnce                    sync.Once
	registerCRBIndexByFieldClusterOnce                   sync.Once
)

// RegisterWorkIndexByLabelResourceBindingID registers index for Work object based on the referencing ResourceBinding ID.
func RegisterWorkIndexByLabelResourceBindingID(ctx context.Context, mgr controllerruntime.Manager) error {
	var err error
	registerWorkIndexByLabelResourceBindingIDOnce.Do(func() {
		err = mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha1.Work{}, WorkIndexByLabelResourceBindingID, GenLabelIndexerFunc(workv1alpha2.ResourceBindingPermanentIDLabel))
		if err != nil {
			klog.Errorf("failed to create Work index by label ResourceBindingID, err: %v", err)
		}
	})
	return err
}

// RegisterWorkIndexByLabelClusterResourceBindingID registers index for Work object based on the referencing ClusterResourceBinding ID.
func RegisterWorkIndexByLabelClusterResourceBindingID(ctx context.Context, mgr controllerruntime.Manager) error {
	var err error
	registerWorkIndexByLabelClusterResourceBindingIDOnce.Do(func() {
		err = mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha1.Work{}, WorkIndexByLabelClusterResourceBindingID, GenLabelIndexerFunc(workv1alpha2.ClusterResourceBindingPermanentIDLabel))
		if err != nil {
			klog.Errorf("failed to create Work index by label ClusterResourceBindingID, err: %v", err)
		}
	})
	return err
}

// RegisterWorkIndexByFieldSuspendDispatching registers index for Work object based on the spec.suspendDispatching field.
func RegisterWorkIndexByFieldSuspendDispatching(ctx context.Context, mgr controllerruntime.Manager) error {
	var err error
	registerWorkIndexByFieldSuspendDispatchingOnce.Do(func() {
		workIndexerFunc := func(obj client.Object) []string {
			work, ok := obj.(*workv1alpha1.Work)
			if !ok {
				return nil
			}
			return util.GetWorkSuspendDispatching(&work.Spec)
		}

		err = mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha1.Work{}, WorkIndexByFieldSuspendDispatching, workIndexerFunc)
		if err != nil {
			klog.Errorf("failed to create Work index by field spec.suspendDispatching, err: %v", err)
		}
	})
	return err
}

// RegisterResourceBindingIndexByFieldCluster registers index for ResourceBinding object based on the spec.clusters field.
func RegisterResourceBindingIndexByFieldCluster(ctx context.Context, mgr controllerruntime.Manager) error {
	var err error
	registerRBIndexByFieldClusterOnce.Do(func() {
		rbIndexerFunc := func(obj client.Object) []string {
			rb, ok := obj.(*workv1alpha2.ResourceBinding)
			if !ok {
				return nil
			}
			return util.GetBindingClusterNames(&rb.Spec)
		}
		err = mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha2.ResourceBinding{}, ResourceBindingIndexByFieldCluster, rbIndexerFunc)
		if err != nil {
			klog.Errorf("failed to create ResourceBinding index by field spec.clusters, err: %v", err)
		}
	})
	return err
}

// RegisterClusterResourceBindingIndexByFieldCluster registers index for ClusterResourceBinding object based on the spec.clusters field.
func RegisterClusterResourceBindingIndexByFieldCluster(ctx context.Context, mgr controllerruntime.Manager) error {
	var err error
	registerCRBIndexByFieldClusterOnce.Do(func() {
		crbIndexerFunc := func(obj client.Object) []string {
			crb, ok := obj.(*workv1alpha2.ClusterResourceBinding)
			if !ok {
				return nil
			}
			return util.GetBindingClusterNames(&crb.Spec)
		}
		err = mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha2.ClusterResourceBinding{}, ClusterResourceBindingIndexByFieldCluster, crbIndexerFunc)
		if err != nil {
			klog.Errorf("failed to create ClusterResourceBinding index by field spec.clusters, err: %v", err)
		}
	})
	return err
}

// GenLabelIndexerFunc returns an IndexerFunc used to index resource with the given label key.
func GenLabelIndexerFunc(labelKey string) client.IndexerFunc {
	return func(obj client.Object) []string {
		labelValue, ok := obj.GetLabels()[labelKey]
		if !ok {
			return nil
		}
		return []string{labelValue}
	}
}
