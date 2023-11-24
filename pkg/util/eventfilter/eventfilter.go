/*
Copyright 2022 The Karmada Authors.

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

package eventfilter

import (
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/karmada-io/karmada/pkg/features"
)

const labelOrAnnoKeyPrefixByKarmada = ".karmada.io"

// SpecificationChanged check if the specification of the given object change or not
func SpecificationChanged(oldObj, newObj *unstructured.Unstructured) bool {
	oldBackup := oldObj.DeepCopy()
	newBackup := newObj.DeepCopy()

	// Remove the status and some system defined mutable fields in metadata, including managedFields and resourceVersion.
	// Refer to https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/object-meta/#ObjectMeta for more details.
	removedFields := [][]string{{"status"}, {"metadata", "managedFields"}, {"metadata", "resourceVersion"}}
	for _, r := range removedFields {
		unstructured.RemoveNestedField(oldBackup.Object, r...)
		unstructured.RemoveNestedField(newBackup.Object, r...)
	}

	// Notes: assuming StaticPolicy disabled, now we can not add this keyPrefixFilter.
	// Because if you delete a Policy, the detector just remove the labels from RT, thus trigger the reconciliation of RT,
	// and then the RT may be put into waiting list.
	// If this keyPrefixFilter added when StaticPolicy disabled, it will eventually prevent the RT from putting into waiting list.
	if features.FeatureGate.Enabled(features.StaticPolicy) {
		for k := range oldBackup.GetLabels() {
			if strings.Contains(k, labelOrAnnoKeyPrefixByKarmada) {
				unstructured.RemoveNestedField(oldBackup.Object, []string{"metadata", "labels", k}...)
			}
		}
		for k := range oldBackup.GetAnnotations() {
			if strings.Contains(k, labelOrAnnoKeyPrefixByKarmada) {
				unstructured.RemoveNestedField(oldBackup.Object, []string{"metadata", "annotations", k}...)
			}
		}
		for k := range newBackup.GetLabels() {
			if strings.Contains(k, labelOrAnnoKeyPrefixByKarmada) {
				unstructured.RemoveNestedField(newBackup.Object, []string{"metadata", "labels", k}...)
			}
		}
		for k := range newBackup.GetAnnotations() {
			if strings.Contains(k, labelOrAnnoKeyPrefixByKarmada) {
				unstructured.RemoveNestedField(newBackup.Object, []string{"metadata", "annotations", k}...)
			}
		}
	}

	return !reflect.DeepEqual(oldBackup, newBackup)
}
