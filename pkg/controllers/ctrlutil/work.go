/*
Copyright 2021 The Karmada Authors.

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

package ctrlutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/native/prune"
	"github.com/karmada-io/karmada/pkg/util"
)

// CreateOrUpdateWork creates a Work object if not exist, or updates if it already exists.
// Performance optimization: Uses Create-First pattern - tries Create first, then Get+Update if already exists.
// This saves 1 API call for new Works (the common case during initial sync).
// Additional optimization: Uses direct Get+Update instead of controllerutil.CreateOrUpdate to avoid redundant Get.
func CreateOrUpdateWork(ctx context.Context, c client.Client, workMeta metav1.ObjectMeta, resource *unstructured.Unstructured, options ...WorkOption) error {
	resource = resource.DeepCopy()
	// set labels
	util.MergeLabel(resource, util.ManagedByKarmadaLabel, util.ManagedByKarmadaLabelValue)
	// set annotations
	util.MergeAnnotation(resource, workv1alpha2.ResourceTemplateUIDAnnotation, string(resource.GetUID()))
	util.MergeAnnotation(resource, workv1alpha2.WorkNameAnnotation, workMeta.Name)
	util.MergeAnnotation(resource, workv1alpha2.WorkNamespaceAnnotation, workMeta.Namespace)
	if conflictResolution, ok := workMeta.GetAnnotations()[workv1alpha2.ResourceConflictResolutionAnnotation]; ok {
		util.MergeAnnotation(resource, workv1alpha2.ResourceConflictResolutionAnnotation, conflictResolution)
	}
	// Do the same thing as the mutating webhook does, remove the irrelevant fields for the resource.
	// This is to avoid unnecessary updates to the Work object, especially when controller starts.
	err := prune.RemoveIrrelevantFields(resource, prune.RemoveJobTTLSeconds)
	if err != nil {
		klog.Errorf("Failed to prune irrelevant fields for resource %s/%s. Error: %v", resource.GetNamespace(), resource.GetName(), err)
		return err
	}

	work := &workv1alpha1.Work{
		ObjectMeta: workMeta,
	}

	applyWorkOptions(work, options)

	// Build the complete Work object for creation
	newWork := work.DeepCopy()
	// Do the same thing as the mutating webhook does, add the permanent ID to workload if not exist
	// This must be called BEFORE marshaling resource to include managed-annotations and managed-labels
	util.SetLabelsAndAnnotationsForWorkload(resource, newWork)

	// Prepare the workload JSON after SetLabelsAndAnnotationsForWorkload has modified resource
	workloadJSON, err := json.Marshal(resource)
	if err != nil {
		klog.Errorf("Failed to marshal workload(%s/%s), error: %v", resource.GetNamespace(), resource.GetName(), err)
		return err
	}

	newWork.Spec.Workload = workv1alpha1.WorkloadTemplate{
		Manifests: []workv1alpha1.Manifest{
			{
				RawExtension: runtime.RawExtension{
					Raw: workloadJSON,
				},
			},
		},
	}

	// Create-First pattern: Try Create first (optimistic path for new Works)
	err = c.Create(ctx, newWork)
	if err == nil {
		klog.V(2).Infof("Create work %s/%s successfully.", work.GetNamespace(), work.GetName())
		return nil
	}

	// If not AlreadyExists error, return immediately
	if !apierrors.IsAlreadyExists(err) {
		klog.Errorf("Failed to create work %s/%s. Error: %v", work.GetNamespace(), work.GetName(), err)
		return err
	}

	// Work already exists, need to update it
	// Use direct Get+Update instead of controllerutil.CreateOrUpdate to avoid redundant Get
	var skipped bool
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existing := &workv1alpha1.Work{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: work.Namespace, Name: work.Name}, existing); err != nil {
			return err
		}

		if !existing.DeletionTimestamp.IsZero() {
			return fmt.Errorf("work %s/%s is being deleted", existing.GetNamespace(), existing.GetName())
		}

		// Performance optimization: Skip update if nothing changed
		// Compare workload content and spec fields
		if workUnchanged(existing, work, workloadJSON) {
			skipped = true
			return nil
		}

		// Merge metadata
		existing.Labels = util.DedupeAndMergeLabels(existing.Labels, work.Labels)
		existing.Annotations = util.DedupeAndMergeAnnotations(existing.Annotations, work.Annotations)
		existing.Finalizers = util.MergeFinalizers(existing.Finalizers, work.Finalizers)
		existing.Spec = work.Spec

		// Update workload - use the same JSON we prepared earlier
		// Note: workloadJSON already includes managed-annotations and managed-labels
		// since SetLabelsAndAnnotationsForWorkload was called before marshaling
		existing.Spec.Workload = workv1alpha1.WorkloadTemplate{
			Manifests: []workv1alpha1.Manifest{
				{
					RawExtension: runtime.RawExtension{
						Raw: workloadJSON,
					},
				},
			},
		}

		return c.Update(ctx, existing)
	})

	if err != nil {
		klog.Errorf("Failed to update work %s/%s. Error: %v", work.GetNamespace(), work.GetName(), err)
		return err
	}

	if skipped {
		klog.V(4).Infof("Skip update work %s/%s, no changes detected.", work.GetNamespace(), work.GetName())
	} else {
		klog.V(2).Infof("Update work %s/%s successfully.", work.GetNamespace(), work.GetName())
	}
	return nil
}

// workUnchanged checks if the Work object has any changes that require an update.
// Returns true if the existing Work is identical to the desired Work (no update needed).
func workUnchanged(existing, desired *workv1alpha1.Work, newWorkloadJSON []byte) bool {
	// Check if workload content changed
	if len(existing.Spec.Workload.Manifests) == 0 {
		return false // No existing workload, needs update
	}
	existingWorkloadJSON := existing.Spec.Workload.Manifests[0].Raw
	if !bytes.Equal(existingWorkloadJSON, newWorkloadJSON) {
		return false // Workload content changed
	}

	// Check spec fields
	if existing.Spec.SuspendDispatching != desired.Spec.SuspendDispatching {
		return false
	}
	if existing.Spec.PreserveResourcesOnDeletion != desired.Spec.PreserveResourcesOnDeletion {
		return false
	}

	// Check if labels need update
	for key, value := range desired.Labels {
		if existing.Labels[key] != value {
			return false // Label needs to be added or changed
		}
	}

	// Check if annotations need update
	for key, value := range desired.Annotations {
		if existing.Annotations[key] != value {
			return false // Annotation needs to be added or changed
		}
	}

	// Check if finalizers need update
	existingFinalizers := make(map[string]bool)
	for _, f := range existing.Finalizers {
		existingFinalizers[f] = true
	}
	for _, f := range desired.Finalizers {
		if !existingFinalizers[f] {
			return false // New finalizer needs to be added
		}
	}

	return true // No changes detected
}
