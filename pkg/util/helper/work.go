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

package helper

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

// CreateOrUpdateWork creates a Work object if not exist, or updates if it already exists.
func CreateOrUpdateWork(ctx context.Context, client client.Client, workMeta metav1.ObjectMeta, resource *unstructured.Unstructured, options ...WorkOption) error {
	if workMeta.Labels[util.PropagationInstruction] != util.PropagationInstructionSuppressed {
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
	}

	workloadJSON, err := resource.MarshalJSON()
	if err != nil {
		klog.Errorf("Failed to marshal workload(%s/%s), error: %v", resource.GetNamespace(), resource.GetName(), err)
		return err
	}

	work := &workv1alpha1.Work{
		ObjectMeta: workMeta,
		Spec: workv1alpha1.WorkSpec{
			Workload: workv1alpha1.WorkloadTemplate{
				Manifests: []workv1alpha1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Raw: workloadJSON,
						},
					},
				},
			},
		},
	}

	applyWorkOptions(work, options)

	runtimeObject := work.DeepCopy()
	var operationResult controllerutil.OperationResult
	err = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		operationResult, err = controllerutil.CreateOrUpdate(ctx, client, runtimeObject, func() error {
			if !runtimeObject.DeletionTimestamp.IsZero() {
				return fmt.Errorf("work %s/%s is being deleted", runtimeObject.GetNamespace(), runtimeObject.GetName())
			}

			runtimeObject.Spec = work.Spec
			runtimeObject.Labels = util.DedupeAndMergeLabels(runtimeObject.Labels, work.Labels)
			runtimeObject.Annotations = util.DedupeAndMergeAnnotations(runtimeObject.Annotations, work.Annotations)
			runtimeObject.Finalizers = work.Finalizers
			return nil
		})
		return err
	})
	if err != nil {
		klog.Errorf("Failed to create/update work %s/%s. Error: %v", work.GetNamespace(), work.GetName(), err)
		return err
	}

	if operationResult == controllerutil.OperationResultCreated {
		klog.V(2).Infof("Create work %s/%s successfully.", work.GetNamespace(), work.GetName())
	} else if operationResult == controllerutil.OperationResultUpdated {
		klog.V(2).Infof("Update work %s/%s successfully.", work.GetNamespace(), work.GetName())
	} else {
		klog.V(2).Infof("Work %s/%s is up to date.", work.GetNamespace(), work.GetName())
	}

	return nil
}

// GetWorksByLabelsSet gets WorkList by matching labels.Set.
func GetWorksByLabelsSet(ctx context.Context, c client.Client, ls labels.Set) (*workv1alpha1.WorkList, error) {
	workList := &workv1alpha1.WorkList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(ls)}

	return workList, c.List(ctx, workList, listOpt)
}

// GetWorksByBindingID gets WorkList by matching same binding's permanent id.
func GetWorksByBindingID(ctx context.Context, c client.Client, bindingID string, namespaced bool) (*workv1alpha1.WorkList, error) {
	var ls labels.Set
	if namespaced {
		ls = labels.Set{workv1alpha2.ResourceBindingPermanentIDLabel: bindingID}
	} else {
		ls = labels.Set{workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID}
	}

	return GetWorksByLabelsSet(ctx, c, ls)
}

// GenEventRef returns the event reference. sets the UID(.spec.uid) that might be missing for fire events.
// Do nothing if the UID already exist, otherwise set the UID from annotation.
func GenEventRef(resource *unstructured.Unstructured) (*corev1.ObjectReference, error) {
	ref := &corev1.ObjectReference{
		Kind:       resource.GetKind(),
		Namespace:  resource.GetNamespace(),
		Name:       resource.GetName(),
		UID:        resource.GetUID(),
		APIVersion: resource.GetAPIVersion(),
	}

	if len(resource.GetUID()) == 0 {
		uid := util.GetAnnotationValue(resource.GetAnnotations(), workv1alpha2.ResourceTemplateUIDAnnotation)
		ref.UID = types.UID(uid)
	}

	if len(ref.UID) == 0 {
		return nil, fmt.Errorf("missing mandatory uid")
	}

	if len(ref.Name) == 0 {
		return nil, fmt.Errorf("missing mandatory name")
	}

	if len(ref.Kind) == 0 {
		return nil, fmt.Errorf("missing mandatory kind")
	}

	return ref, nil
}

// IsWorkContains checks if the target resource exists in a work.spec.workload.manifests.
func IsWorkContains(manifests []workv1alpha1.Manifest, targetResource schema.GroupVersionKind) bool {
	for index := range manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifests[index].Raw)
		if err != nil {
			klog.Errorf("Failed to unmarshal work manifests index %d, error is: %v", index, err)
			continue
		}

		if targetResource == workload.GroupVersionKind() {
			return true
		}
	}
	return false
}

// IsWorkSuspendDispatching checks if the work is suspended from dispatching.
func IsWorkSuspendDispatching(work *workv1alpha1.Work) bool {
	return ptr.Deref(work.Spec.SuspendDispatching, false)
}
