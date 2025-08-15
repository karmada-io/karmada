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

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
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

	// ResourceTemplateIndexByLabelPolicyID is the index name for ResourceTemplates associated with a PropagationPolicy ID.
	// This index allows efficient lookup of ResourceTemplates by their `metadata.labels.<PropagationPolicyPermanentIDLabel>`,
	// which references the ID of the bound PropagationPolicy object.
	ResourceTemplateIndexByLabelPolicyID = "ResourceTemplateIndexByLabelPolicyID"

	// ResourceTemplateIndexByLabelClusterPolicyID is the index name for ResourceTemplates associated with a ClusterPropagationPolicy ID.
	// This index allows efficient lookup of ResourceTemplates by their `metadata.labels.<ClusterPropagationPolicyPermanentIDLabel>`,
	// which references the ID of the bound ClusterPropagationPolicy object.
	ResourceTemplateIndexByLabelClusterPolicyID = "ResourceTemplateIndexByLabelClusterPolicyID"

	// ResourceBindingIndexByLabelPolicyID is the index name for ResourceBindings associated with a PropagationPolicy ID.
	// This index allows efficient lookup of ResourceBindings by their `metadata.labels.<PropagationPolicyPermanentIDLabel>`,
	// which references the ID of the bound PropagationPolicy object.
	ResourceBindingIndexByLabelPolicyID = "ResourceBindingIndexByLabelPolicyID"

	// ResourceBindingIndexByLabelClusterPolicyID is the index name for ResourceBindings associated with a ClusterPropagationPolicy ID.
	// This index allows efficient lookup of ResourceBindings by their `metadata.labels.<ClusterPropagationPolicyPermanentIDLabel>`,
	// which references the ID of the bound ClusterPropagationPolicy object.
	ResourceBindingIndexByLabelClusterPolicyID = "ResourceBindingIndexByLabelClusterPolicyID"

	// ClusterResourceBindingIndexByLabelClusterPolicyID is the index name for ClusterResourceBindings associated with a ClusterPropagationPolicy ID.
	// This index allows efficient lookup of ClusterResourceBindings by their `metadata.labels.<ClusterPropagationPolicyPermanentIDLabel>`,
	// which references the ID of the bound ClusterPropagationPolicy object.
	ClusterResourceBindingIndexByLabelClusterPolicyID = "ClusterResourceBindingIndexByLabelClusterPolicyID"
)

var (
	registerWorkIndexByLabelResourceBindingIDOnce        sync.Once
	registerWorkIndexByLabelClusterResourceBindingIDOnce sync.Once
	registerWorkIndexByFieldSuspendDispatchingOnce       sync.Once
	registerRBIndexByFieldClusterOnce                    sync.Once
	registerCRBIndexByFieldClusterOnce                   sync.Once
	registerRBIndexByLabelPolicyIDOnce                   sync.Once
	registerRBIndexByLabelClusterPolicyIDOnce            sync.Once
	registerCRBIndexByLabelClusterPolicyIDOnce           sync.Once
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

// RegisterResourceBindingIndexByLabel registers indexes for ResourceBinding objects based on policy labels.
// This function registers both PropagationPolicy and ClusterPropagationPolicy permanent ID label indexes.
func RegisterResourceBindingIndexByLabel(ctx context.Context, mgr controllerruntime.Manager) error {
	var errs []error
	registerRBIndexByLabelPolicyIDOnce.Do(func() {
		err := mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha2.ResourceBinding{}, ResourceBindingIndexByLabelPolicyID, GenLabelIndexerFunc(policyv1alpha1.PropagationPolicyPermanentIDLabel))
		if err != nil {
			klog.Errorf("failed to create ResourceBinding index by label PolicyPermanentID, err: %v", err)
			errs = append(errs, err)
		}
	})
	registerRBIndexByLabelClusterPolicyIDOnce.Do(func() {
		err := mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha2.ResourceBinding{}, ResourceBindingIndexByLabelClusterPolicyID, GenLabelIndexerFunc(policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel))
		if err != nil {
			klog.Errorf("failed to create ResourceBinding index by label ClusterPolicyPermanentID, err: %v", err)
			errs = append(errs, err)
		}
	})

	return errors.NewAggregate(errs)
}

// RegisterClusterResourceBindingIndexByLabel registers index for ClusterResourceBinding objects based on ClusterPropagationPolicy label.
// This function registers the ClusterPropagationPolicy permanent ID label index for ClusterResourceBindings.
func RegisterClusterResourceBindingIndexByLabel(ctx context.Context, mgr controllerruntime.Manager) error {
	var err error
	registerCRBIndexByLabelClusterPolicyIDOnce.Do(func() {
		err = mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha2.ClusterResourceBinding{}, ClusterResourceBindingIndexByLabelClusterPolicyID, GenLabelIndexerFunc(policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel))
		if err != nil {
			klog.Errorf("failed to create ClusterResourceBinding index by label ClusterPolicyPermanentID, err: %v", err)
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

// GenLabelInformerIndexerFunc returns a cache.IndexFunc used to index resource with the given label key for informer-based indexing.
// This function is similar to GenLabelIndexerFunc but returns a cache.IndexFunc for use with client-go informers.
func GenLabelInformerIndexerFunc(labelKey string) cache.IndexFunc {
	return func(obj interface{}) ([]string, error) {
		object, err := meta.Accessor(obj)
		if err != nil {
			return nil, nil
		}
		if val, ok := object.GetLabels()[labelKey]; ok {
			return []string{val}, nil
		}
		return nil, nil
	}
}
