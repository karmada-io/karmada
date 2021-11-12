/*
Copyright The Karmada Authors.

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
