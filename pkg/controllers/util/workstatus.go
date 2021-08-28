package util

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

// GetManifestIndex get the index of clusterObj in manifest list, if not exist return -1.
func GetManifestIndex(manifests []workv1alpha1.Manifest, clusterObj *unstructured.Unstructured) (int, error) {
	for index, rawManifest := range manifests {
		manifest := &unstructured.Unstructured{}
		if err := manifest.UnmarshalJSON(rawManifest.Raw); err != nil {
			return -1, err
		}

		if manifest.GetAPIVersion() == clusterObj.GetAPIVersion() &&
			manifest.GetKind() == clusterObj.GetKind() &&
			manifest.GetNamespace() == clusterObj.GetNamespace() &&
			manifest.GetName() == clusterObj.GetName() {
			return index, nil
		}
	}

	return -1, fmt.Errorf("no such manifest exist")
}

// IsResourceApplied checks whether resource has been dispatched to member cluster or not
func IsResourceApplied(workStatus *workv1alpha1.WorkStatus) bool {
	return meta.IsStatusConditionTrue(workStatus.Conditions, workv1alpha1.WorkApplied)
}

// IsWorkContains checks if the target resource exists in a work.
// Note: This function checks the Work object's status to detect the target resource, so the Work should be 'Applied',
// otherwise always returns false.
func IsWorkContains(workStatus *workv1alpha1.WorkStatus, targetResource schema.GroupVersionKind) bool {
	for _, manifestStatuses := range workStatus.ManifestStatuses {
		if targetResource.Group == manifestStatuses.Identifier.Group &&
			targetResource.Version == manifestStatuses.Identifier.Version &&
			targetResource.Kind == manifestStatuses.Identifier.Kind {
			return true
		}
	}
	return false
}
