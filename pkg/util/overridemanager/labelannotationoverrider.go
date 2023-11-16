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

package overridemanager

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func applyLabelsOverriders(rawObj *unstructured.Unstructured, labelOverriders []policyv1alpha1.LabelAnnotationOverrider) error {
	return applyLabelAnnotationOverriders(rawObj, labelOverriders, "metadata", "labels")
}

func applyAnnotationsOverriders(rawObj *unstructured.Unstructured, annotationOverriders []policyv1alpha1.LabelAnnotationOverrider) error {
	return applyLabelAnnotationOverriders(rawObj, annotationOverriders, "metadata", "annotations")
}

func applyLabelAnnotationOverriders(rawObj *unstructured.Unstructured, labelAnnotationOverriders []policyv1alpha1.LabelAnnotationOverrider, path ...string) error {
	for index := range labelAnnotationOverriders {
		patches := buildLabelAnnotationOverriderPatches(rawObj, labelAnnotationOverriders[index], path)
		if len(patches) == 0 {
			continue
		}
		if err := applyJSONPatch(rawObj, patches); err != nil {
			return err
		}
	}
	return nil
}

func buildLabelAnnotationOverriderPatches(rawObj *unstructured.Unstructured, overrider policyv1alpha1.LabelAnnotationOverrider, path []string) []overrideOption {
	patches := make([]overrideOption, 0)
	for key, value := range overrider.Value {
		switch overrider.Operator {
		case policyv1alpha1.OverriderOpRemove, policyv1alpha1.OverriderOpReplace:
			match, _, _ := unstructured.NestedStringMap(rawObj.Object, path...)
			if _, exist := match[key]; !exist {
				continue
			}
		case policyv1alpha1.OverriderOpAdd:
			_, exist, _ := unstructured.NestedStringMap(rawObj.Object, path...)
			if exist {
				break
			}
			if err := unstructured.SetNestedStringMap(rawObj.Object, map[string]string{}, path...); err != nil {
				continue
			}
		}
		// If the key contains '/', we must replace(escape) it to '~1' according to the
		// rule of jsonpath identifying a specific value.
		// See https://jsonpatch.com/#json-pointer for more details.
		// Note: here don't replace '~' because it is not a valid character for
		// both annotations and labels, the key with '~' should be prevented at
		// the validation phase.
		key = strings.ReplaceAll(key, "/", "~1")
		patches = append(patches, overrideOption{
			Op:    string(overrider.Operator),
			Path:  "/" + strings.Join(append(path, key), "/"),
			Value: value,
		})
	}
	return patches
}
