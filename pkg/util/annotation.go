package util

import (
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// MergeAnnotation adds annotation for the given object, keep the value unchanged if key exist.
func MergeAnnotation(obj *unstructured.Unstructured, annotationKey string, annotationValue string) {
	objectAnnotation := obj.GetAnnotations()
	if objectAnnotation == nil {
		objectAnnotation = make(map[string]string, 1)
	}

	if _, exist := objectAnnotation[annotationKey]; !exist {
		objectAnnotation[annotationKey] = annotationValue
		obj.SetAnnotations(objectAnnotation)
	}
}

// ReplaceAnnotation adds annotation for the given object, replace the value if key exist.
func ReplaceAnnotation(obj *unstructured.Unstructured, annotationKey string, annotationValue string) {
	objectAnnotation := obj.GetAnnotations()
	if objectAnnotation == nil {
		objectAnnotation = make(map[string]string, 1)
	}

	objectAnnotation[annotationKey] = annotationValue
	obj.SetAnnotations(objectAnnotation)
}

// RetainAnnotations merges the annotations that added by controllers running
// in member cluster to avoid overwriting.
// Following keys will be ignored if :
//   - the keys were previous propagated to member clusters(that are tracked
//     by "resourcetemplate.karmada.io/managed-annotations" annotation in observed)
//     but have been removed from Karmada control plane(don't exist in desired anymore).
//   - the keys that exist in both desired and observed even those been accidentally modified
//     in member clusters.
func RetainAnnotations(desired *unstructured.Unstructured, observed *unstructured.Unstructured) {
	objectAnnotation := desired.GetAnnotations()
	if objectAnnotation == nil {
		objectAnnotation = make(map[string]string, 0)
	}
	deletedAnnotationKeys := getDeletedAnnotationKeys(desired, observed)
	for key, value := range observed.GetAnnotations() {
		if deletedAnnotationKeys.Has(key) {
			continue
		}
		if _, exist := objectAnnotation[key]; exist {
			continue
		}
		objectAnnotation[key] = value
	}
	if len(objectAnnotation) > 0 {
		desired.SetAnnotations(objectAnnotation)
	}
}

// GetAnnotationValue retrieves the value via 'annotationKey' (if it exists), otherwise an empty string is returned.
func GetAnnotationValue(annotations map[string]string, annotationKey string) string {
	if annotations == nil {
		return ""
	}
	return annotations[annotationKey]
}

func getDeletedAnnotationKeys(desired, observed *unstructured.Unstructured) sets.Set[string] {
	recordKeys := sets.New[string](strings.Split(observed.GetAnnotations()[workv1alpha2.ManagedAnnotation], ",")...)
	for key := range desired.GetAnnotations() {
		recordKeys.Delete(key)
	}
	return recordKeys
}

// RecordManagedAnnotations sets or updates the annotation(resourcetemplate.karmada.io/managed-annotations)
// to record the annotation keys.
func RecordManagedAnnotations(object *unstructured.Unstructured) {
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}
	// record annotations.
	managedKeys := []string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels}
	for key := range annotations {
		if key == workv1alpha2.ManagedAnnotation || key == workv1alpha2.ManagedLabels {
			continue
		}
		managedKeys = append(managedKeys, key)
	}
	sort.Strings(managedKeys)
	annotations[workv1alpha2.ManagedAnnotation] = strings.Join(managedKeys, ",")
	object.SetAnnotations(annotations)
}

// DedupeAndMergeAnnotations merges the new annotations into exist annotations.
func DedupeAndMergeAnnotations(existAnnotation, newAnnotation map[string]string) map[string]string {
	if existAnnotation == nil {
		return newAnnotation
	}

	for k, v := range newAnnotation {
		existAnnotation[k] = v
	}
	return existAnnotation
}
