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
		patches = append(patches, overrideOption{
			Op:    string(overrider.Operator),
			Path:  "/" + strings.Join(append(path, key), "/"),
			Value: value,
		})
	}
	return patches
}
