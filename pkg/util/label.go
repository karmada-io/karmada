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

package util

import (
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// GetLabelValue retrieves the value via 'labelKey' if exist, otherwise returns an empty string.
func GetLabelValue(labels map[string]string, labelKey string) string {
	if labels == nil {
		return ""
	}

	return labels[labelKey]
}

// RetainLabels merges the labels that added by controllers running
// in member cluster to avoid overwriting.
// Following keys will be ignored if :
//   - the keys were previous propagated to member clusters(that are tracked
//     by "resourcetemplate.karmada.io/managed-lables" annotation in observed)
//     but have been removed from Karmada control plane(don't exist in desired anymore).
//   - the keys that exist in both desired and observed even those been accidentally modified
//     in member clusters.
func RetainLabels(desired *unstructured.Unstructured, observed *unstructured.Unstructured) {
	labels := desired.GetLabels()
	if labels == nil {
		labels = make(map[string]string, 0)
	}
	deletedLabelKeys := getDeletedLabelKeys(desired, observed)
	for key, value := range observed.GetLabels() {
		if deletedLabelKeys.Has(key) {
			continue
		}
		if _, exist := labels[key]; exist {
			continue
		}
		labels[key] = value
	}
	if len(labels) > 0 {
		desired.SetLabels(labels)
	}
}

// MergeLabel adds label for the given object, replace the value if key exist.
func MergeLabel(obj metav1.Object, labelKey string, labelValue string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string, 1)
	}
	labels[labelKey] = labelValue
	obj.SetLabels(labels)
}

// RemoveLabels removes the labels from the given object.
func RemoveLabels(obj metav1.Object, labelKeys ...string) {
	if len(labelKeys) == 0 {
		return
	}

	objLabels := obj.GetLabels()
	for _, labelKey := range labelKeys {
		delete(objLabels, labelKey)
	}
	obj.SetLabels(objLabels)
}

// DedupeAndMergeLabels merges the new labels into exist labels.
func DedupeAndMergeLabels(existLabel, newLabel map[string]string) map[string]string {
	if existLabel == nil {
		return newLabel
	}

	for k, v := range newLabel {
		existLabel[k] = v
	}
	return existLabel
}

func getDeletedLabelKeys(desired, observed *unstructured.Unstructured) sets.Set[string] {
	recordKeys := sets.New[string](strings.Split(observed.GetAnnotations()[workv1alpha2.ManagedLabels], ",")...)
	for key := range desired.GetLabels() {
		recordKeys.Delete(key)
	}
	return recordKeys
}

// RecordManagedLabels sets or updates the annotation(resourcetemplate.karmada.io/managed-labels)
// to record the label keys.
func RecordManagedLabels(object *unstructured.Unstructured) {
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}
	var managedKeys []string
	// record labels.
	labels := object.GetLabels()
	for key := range labels {
		managedKeys = append(managedKeys, key)
	}
	sort.Strings(managedKeys)
	annotations[workv1alpha2.ManagedLabels] = strings.Join(managedKeys, ",")
	object.SetAnnotations(annotations)
}

// DedupeAndMergeFinalizers merges the new finalizers into exist finalizers.
func DedupeAndMergeFinalizers(existFinalizers, newFinalizers []string) []string {
	if len(existFinalizers) == 0 {
		return newFinalizers
	}
	existFinalizerSets := sets.Set[string]{}
	existFinalizerSets.Insert(existFinalizers...)

	var mergedFinalizers []string
	mergedFinalizers = append(mergedFinalizers, existFinalizers...)
	for _, item := range newFinalizers {
		if !existFinalizerSets.Has(item) {
			mergedFinalizers = append(mergedFinalizers, item)
		}
	}
	return mergedFinalizers
}
