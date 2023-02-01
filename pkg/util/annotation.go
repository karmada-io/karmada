package util

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// MergeAnnotation adds annotation for the given object.
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

// MergeAnnotations merges the annotations from 'src' to 'dst', identical keys will not be merged.
func MergeAnnotations(dst *unstructured.Unstructured, src *unstructured.Unstructured) {
	for key, value := range src.GetAnnotations() {
		MergeAnnotation(dst, key, value)
	}
}

// GetAnnotationValue retrieves the value via 'annotationKey' (if it exists), otherwise an empty string is returned.
func GetAnnotationValue(annotations map[string]string, annotationKey string) string {
	if annotations == nil {
		return ""
	}
	return annotations[annotationKey]
}
