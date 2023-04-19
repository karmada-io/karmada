package native

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

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

	desiredPod.Spec.NodeName = clusterPod.Spec.NodeName
	desiredPod.Spec.ServiceAccountName = clusterPod.Spec.ServiceAccountName
	desiredPod.Spec.Volumes = clusterPod.Spec.Volumes
	// retain volumeMounts in each container
	for _, clusterContainer := range clusterPod.Spec.Containers {
		for desiredIndex, desiredContainer := range desiredPod.Spec.Containers {
			if desiredContainer.Name == clusterContainer.Name {
				desiredPod.Spec.Containers[desiredIndex].VolumeMounts = clusterContainer.VolumeMounts
				break
			}
		}
	}

	// retain volumeMounts in each init container
	for _, clusterInitContainer := range clusterPod.Spec.InitContainers {
		for desiredIndex, desiredInitContainer := range desiredPod.Spec.InitContainers {
			if desiredInitContainer.Name == clusterInitContainer.Name {
				desiredPod.Spec.InitContainers[desiredIndex].VolumeMounts = clusterInitContainer.VolumeMounts
				break
			}
		}
	}
	unstructuredObj, err := helper.ToUnstructured(desiredPod)
	if err != nil {
		return nil, fmt.Errorf("failed to transform Pod: %v", err)
	}

	return unstructuredObj, nil
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
