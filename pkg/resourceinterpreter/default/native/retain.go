package native

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// retentionInterpreter is the function that retains values from "observed" object.
type retentionInterpreter func(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error)

func getAllDefaultRetentionInterpreter() map[schema.GroupVersionKind]retentionInterpreter {
	s := make(map[schema.GroupVersionKind]retentionInterpreter)
	s[corev1.SchemeGroupVersion.WithKind(util.PodKind)] = retainPodFields
	s[corev1.SchemeGroupVersion.WithKind(util.ServiceKind)] = lifted.RetainServiceFields
	s[corev1.SchemeGroupVersion.WithKind(util.ServiceAccountKind)] = lifted.RetainServiceAccountFields
	s[corev1.SchemeGroupVersion.WithKind(util.PersistentVolumeClaimKind)] = retainPersistentVolumeClaimFields
	s[corev1.SchemeGroupVersion.WithKind(util.PersistentVolumeKind)] = retainPersistentVolumeFields
	s[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = retainJobSelectorFields
	return s
}

func retainPodFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	desiredPod := &corev1.Pod{}
	err := helper.ConvertToTypedObject(desired, desiredPod)
	if err != nil {
		return nil, fmt.Errorf("failed to convert desiredPod from unstructured object: %v", err)
	}

	clusterPod := &corev1.Pod{}
	err = helper.ConvertToTypedObject(observed, clusterPod)
	if err != nil {
		return nil, fmt.Errorf("failed to convert clusterPod from unstructured object: %v", err)
	}

	// Pod updates may not change fields other than `spec.containers[*].image`, `spec.initContainers[*].image`,
	// `spec.activeDeadlineSeconds`, `spec.tolerations` (only additions to existing tolerations), in place.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/#pod-update-and-replacement
	//
	// There are many fields in Pod, but only a few fields that can be modified. We choose to return clusterPod
	// because it is unnecessary waste to copy from clusterPod to desiredPod.
	clusterPod.Labels = desiredPod.Labels
	clusterPod.Annotations = desiredPod.Annotations
	updatePodImages(desiredPod, clusterPod)
	updatePodActiveDeadlineSeconds(desiredPod, clusterPod)
	updatePodTolerations(desiredPod, clusterPod)

	unstructuredObj, err := helper.ToUnstructured(clusterPod)
	if err != nil {
		return nil, fmt.Errorf("failed to transform Pod: %v", err)
	}

	return unstructuredObj, nil
}

func updatePodImages(desiredPod, clusterPod *corev1.Pod) {
	for clusterIndex, clusterContainer := range clusterPod.Spec.InitContainers {
		for _, desiredContainer := range desiredPod.Spec.InitContainers {
			if desiredContainer.Name == clusterContainer.Name {
				clusterPod.Spec.InitContainers[clusterIndex].Image = desiredContainer.Image
				break
			}
		}
	}
	for clusterIndex, clusterContainer := range clusterPod.Spec.Containers {
		for _, desiredContainer := range desiredPod.Spec.Containers {
			if desiredContainer.Name == clusterContainer.Name {
				clusterPod.Spec.Containers[clusterIndex].Image = desiredContainer.Image
				break
			}
		}
	}
}

func updatePodActiveDeadlineSeconds(desiredPod, clusterPod *corev1.Pod) {
	desiredActiveDeadlineSeconds := pointer.Int64Deref(desiredPod.Spec.ActiveDeadlineSeconds, 0)
	clusterActiveDeadlineSeconds := pointer.Int64Deref(clusterPod.Spec.ActiveDeadlineSeconds, 0)
	if (clusterPod.Spec.ActiveDeadlineSeconds == nil && desiredActiveDeadlineSeconds > 0) ||
		(desiredActiveDeadlineSeconds > 0 && desiredActiveDeadlineSeconds < clusterActiveDeadlineSeconds) {
		clusterPod.Spec.ActiveDeadlineSeconds = desiredPod.Spec.ActiveDeadlineSeconds
	}
}

func updatePodTolerations(desiredPod, clusterPod *corev1.Pod) {
outer:
	for i := 0; i < len(desiredPod.Spec.Tolerations); i++ {
		desiredToleration := &desiredPod.Spec.Tolerations[i]
		for j := 0; j < len(clusterPod.Spec.Tolerations); j++ {
			if clusterPod.Spec.Tolerations[j].MatchToleration(desiredToleration) {
				continue outer
			}
		}
		clusterPod.Spec.Tolerations = append(clusterPod.Spec.Tolerations, *desiredToleration)
	}
}

func retainPersistentVolumeClaimFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// volumeName is allocated by member cluster and unchangeable, so it should be retained while updating
	volumeName, ok, err := unstructured.NestedString(observed.Object, "spec", "volumeName")
	if err != nil {
		return nil, fmt.Errorf("error retrieving volumeName from pvc: %w", err)
	}
	if ok && len(volumeName) > 0 {
		if err = unstructured.SetNestedField(desired.Object, volumeName, "spec", "volumeName"); err != nil {
			return nil, fmt.Errorf("error setting volumeName for pvc: %w", err)
		}
	}
	return desired, nil
}

func retainPersistentVolumeFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// claimRef is updated by kube-controller-manager.
	claimRef, ok, err := unstructured.NestedFieldNoCopy(observed.Object, "spec", "claimRef")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve claimRef from pv: %v", err)
	}
	if ok {
		if err = unstructured.SetNestedField(desired.Object, claimRef, "spec", "claimRef"); err != nil {
			return nil, fmt.Errorf("failed to set claimRef for pv: %v", err)
		}
	}
	return desired, nil
}

func retainJobSelectorFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	matchLabels, exist, err := unstructured.NestedStringMap(observed.Object, "spec", "selector", "matchLabels")
	if err != nil {
		return nil, err
	}
	if exist {
		err = unstructured.SetNestedStringMap(desired.Object, matchLabels, "spec", "selector", "matchLabels")
		if err != nil {
			return nil, err
		}
	}

	templateLabels, exist, err := unstructured.NestedStringMap(observed.Object, "spec", "template", "metadata", "labels")
	if err != nil {
		return nil, err
	}
	if exist {
		err = unstructured.SetNestedStringMap(desired.Object, templateLabels, "spec", "template", "metadata", "labels")
		if err != nil {
			return nil, err
		}
	}
	return desired, nil
}
