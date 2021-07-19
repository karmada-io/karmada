package util

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// MergeAnnotation adds annotation for the given object.
func MergeAnnotation(obj *unstructured.Unstructured, annotationKey string, annotationValue string) {
	objectAnnotation := obj.GetAnnotations()
	if objectAnnotation == nil {
		objectAnnotation = make(map[string]string, 1)
	}
	objectAnnotation[annotationKey] = annotationValue
	obj.SetAnnotations(objectAnnotation)
}

// MergeAnnotations merges the annotations from 'src' to 'dst'.
func MergeAnnotations(dst *unstructured.Unstructured, src *unstructured.Unstructured) {
	for key, value := range src.GetAnnotations() {
		MergeAnnotation(dst, key, value)
	}
}
