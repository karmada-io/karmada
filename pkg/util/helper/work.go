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

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// CreateOrUpdateWork creates a Work object if not exist, or updates if it already exist.
func CreateOrUpdateWork(client client.Client, workMeta metav1.ObjectMeta, resource *unstructured.Unstructured) error {
	workload := resource.DeepCopy()
	if conflictResolution, ok := workMeta.GetAnnotations()[workv1alpha2.ResourceConflictResolutionAnnotation]; ok {
		util.MergeAnnotation(workload, workv1alpha2.ResourceConflictResolutionAnnotation, conflictResolution)
	}
	util.MergeAnnotation(workload, workv1alpha2.ResourceTemplateUIDAnnotation, string(workload.GetUID()))
	util.RecordManagedAnnotations(workload)
	workloadJSON, err := workload.MarshalJSON()
	if err != nil {
		klog.Errorf("Failed to marshal workload(%s/%s), Error: %v", workload.GetNamespace(), workload.GetName(), err)
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

	runtimeObject := work.DeepCopy()
	var operationResult controllerutil.OperationResult
	err = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		operationResult, err = controllerutil.CreateOrUpdate(context.TODO(), client, runtimeObject, func() error {
			if !runtimeObject.DeletionTimestamp.IsZero() {
				return fmt.Errorf("work %s/%s is being deleted", runtimeObject.GetNamespace(), runtimeObject.GetName())
			}

			runtimeObject.Spec = work.Spec
			runtimeObject.Labels = work.Labels
			runtimeObject.Annotations = work.Annotations
			if util.GetLabelValue(runtimeObject.Labels, workv1alpha2.WorkPermanentIDLabel) == "" {
				runtimeObject.Labels = util.DedupeAndMergeLabels(runtimeObject.Labels, map[string]string{workv1alpha2.WorkPermanentIDLabel: uuid.New().String()})
			}
			util.RecordManagedLabels(runtimeObject)
			return nil
		})
		if err != nil {
			return err
		}
		return nil
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

// GetWorksByLabelsSet get WorkList by matching labels.Set.
func GetWorksByLabelsSet(c client.Client, ls labels.Set) (*workv1alpha1.WorkList, error) {
	workList := &workv1alpha1.WorkList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(ls)}

	return workList, c.List(context.TODO(), workList, listOpt)
}

// GetWorksByBindingNamespaceName get WorkList by matching same Namespace and same Name.
func GetWorksByBindingNamespaceName(c client.Client, bindingNamespace, bindingName string) (*workv1alpha1.WorkList, error) {
	referenceKey := names.GenerateBindingReferenceKey(bindingNamespace, bindingName)
	var ls labels.Set
	if bindingNamespace != "" {
		ls = labels.Set{workv1alpha2.ResourceBindingReferenceKey: referenceKey}
	} else {
		ls = labels.Set{workv1alpha2.ClusterResourceBindingReferenceKey: referenceKey}
	}

	workList, err := GetWorksByLabelsSet(c, ls)
	if err != nil {
		return nil, err
	}
	retWorkList := &workv1alpha1.WorkList{}
	// Due to the hash collision problem, we have to filter the Works by annotation.
	// More details please refer to https://github.com/karmada-io/karmada/issues/2071.
	for i := range workList.Items {
		if len(bindingNamespace) > 0 { // filter Works that derived by 'ResourceBinding'
			if util.GetAnnotationValue(workList.Items[i].GetAnnotations(), workv1alpha2.ResourceBindingNameAnnotationKey) == bindingName &&
				util.GetAnnotationValue(workList.Items[i].GetAnnotations(), workv1alpha2.ResourceBindingNamespaceAnnotationKey) == bindingNamespace {
				retWorkList.Items = append(retWorkList.Items, workList.Items[i])
			}
		} else { // filter Works that derived by 'ClusterResourceBinding'
			if util.GetAnnotationValue(workList.Items[i].GetAnnotations(), workv1alpha2.ClusterResourceBindingAnnotationKey) == bindingName {
				retWorkList.Items = append(retWorkList.Items, workList.Items[i])
			}
		}
	}

	return retWorkList, nil
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
