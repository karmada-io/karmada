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

package util

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

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
		uid := GetAnnotationValue(resource.GetAnnotations(), workv1alpha2.ResourceTemplateUIDAnnotation)
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

// SetLabelsAndAnnotationsForWorkload sets the associated work object labels and annotations for workload.
func SetLabelsAndAnnotationsForWorkload(workload *unstructured.Unstructured, work *workv1alpha1.Work) {
	RecordManagedAnnotations(workload)
	if work.Labels[workv1alpha2.WorkPermanentIDLabel] != "" {
		workload.SetLabels(DedupeAndMergeLabels(workload.GetLabels(), map[string]string{
			workv1alpha2.WorkPermanentIDLabel: work.Labels[workv1alpha2.WorkPermanentIDLabel],
		}))
	}
	RecordManagedLabels(workload)
}

// GetWorkSuspendDispatching will get suspendDispatching field from work spec
func GetWorkSuspendDispatching(spec *workv1alpha1.WorkSpec) []string {
	return []string{strconv.FormatBool(ptr.Deref(spec.SuspendDispatching, false))}
}
