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
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/native/prune"
	"github.com/karmada-io/karmada/pkg/util"
)

// CreateOrUpdateWork creates a Work object if not exist, or updates if it already exists.
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

	runtimeObject := work.DeepCopy()
	var operationResult controllerutil.OperationResult
	err = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		operationResult, err = controllerutil.CreateOrUpdate(ctx, c, runtimeObject, func() error {
			if !runtimeObject.DeletionTimestamp.IsZero() {
				return fmt.Errorf("work %s/%s is being deleted", runtimeObject.GetNamespace(), runtimeObject.GetName())
			}

			runtimeObject.Labels = util.DedupeAndMergeLabels(runtimeObject.Labels, work.Labels)
			runtimeObject.Annotations = util.DedupeAndMergeAnnotations(runtimeObject.Annotations, work.Annotations)
			runtimeObject.Finalizers = util.MergeFinalizers(runtimeObject.Finalizers, work.Finalizers)
			runtimeObject.Spec = work.Spec

			// Do the same thing as the mutating webhook does, add the permanent ID to workload if not exist,
			// This is to avoid unnecessary updates to the Work object, especially when controller starts.
			util.SetLabelsAndAnnotationsForWorkload(resource, runtimeObject)
			workloadJSON, err := json.Marshal(resource)
			if err != nil {
				klog.Errorf("Failed to marshal workload(%s/%s), error: %v", resource.GetNamespace(), resource.GetName(), err)
				return err
			}
			runtimeObject.Spec.Workload = workv1alpha1.WorkloadTemplate{
				Manifests: []workv1alpha1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Raw: workloadJSON,
						},
					},
				},
			}
			return nil
		})
		return err
	})
	if err != nil {
		klog.Errorf("Failed to create/update work %s/%s. Error: %v", work.GetNamespace(), work.GetName(), err)
		return err
	}

	switch operationResult {
	case controllerutil.OperationResultCreated:
		klog.V(2).Infof("Create work %s/%s successfully.", work.GetNamespace(), work.GetName())
	case controllerutil.OperationResultUpdated:
		klog.V(2).Infof("Update work %s/%s successfully.", work.GetNamespace(), work.GetName())
	default:
		klog.V(2).Infof("Work %s/%s is up to date.", work.GetNamespace(), work.GetName())
	}

	return nil
}
