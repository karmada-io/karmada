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

	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

const (
	// WorkIndexByResourceBindingID is the index name for Works that are associated with a ResourceBinding ID.
	// This index allows efficient lookup of Works by their `metadata.labels.<ResourceBindingPermanentIDLabel>`,
	// which references the ID of the bound ResourceBinding object.
	WorkIndexByResourceBindingID = "WorkIndexByResourceBindingID"

	// WorkIndexByClusterResourceBindingID is the index name for Works associated with a ClusterResourceBinding ID.
	// The index is built using `metadata.labels.<ClusterResourceBindingPermanentIDLabel>` to enable fast queries
	// of Works linked to specific ClusterResourceBinding objects across all namespaces.
	WorkIndexByClusterResourceBindingID = "WorkIndexByClusterResourceBindingID"
)

// RegisterWorkParentRBIndex registers index for Work object based on the referencing ResourceBinding ID.
func RegisterWorkParentRBIndex(ctx context.Context, mgr controllerruntime.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha1.Work{}, WorkIndexByResourceBindingID, GenLabelIndexerFunc(workv1alpha2.ResourceBindingPermanentIDLabel))
	if err != nil {
		klog.Errorf("failed to create index for work, err: %v", err)
		return err
	}
	return nil
}

// RegisterWorkParentCRBIndex registers index for Work object based on the referencing ClusterResourceBinding ID.
func RegisterWorkParentCRBIndex(ctx context.Context, mgr controllerruntime.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha1.Work{}, WorkIndexByClusterResourceBindingID, GenLabelIndexerFunc(workv1alpha2.ClusterResourceBindingPermanentIDLabel))
	if err != nil {
		klog.Errorf("failed to create index for work, err: %v", err)
		return err
	}
	return nil
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
