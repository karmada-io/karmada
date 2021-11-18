package defaultexplorer

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// retentionExplorer is the function that retains values from "observed" object.
type retentionExplorer func(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error)

func getAllDefaultRetentionExplorer() map[schema.GroupVersionKind]retentionExplorer {
	explorers := make(map[schema.GroupVersionKind]retentionExplorer)
	explorers[corev1.SchemeGroupVersion.WithKind(util.PodKind)] = retainPodFields
	explorers[corev1.SchemeGroupVersion.WithKind(util.ServiceKind)] = retainServiceFields
	explorers[corev1.SchemeGroupVersion.WithKind(util.ServiceAccountKind)] = retainServiceAccountFields
	explorers[corev1.SchemeGroupVersion.WithKind(util.PersistentVolumeClaimKind)] = retainPersistentVolumeClaimFields
	explorers[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = retainJobSelectorFields
	return explorers
}

/*
This code is directly lifted from the kubefed codebase. It's a list of functions to update the desired object with values retained
from the cluster object.
For reference: https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/sync/dispatch/retain.go#L27-L133
*/

const (
	// SecretsField indicates the 'secrets' field of a service account
	SecretsField = "secrets"
)

func retainPodFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	desiredPod, err := helper.ConvertToPod(desired)
	if err != nil {
		return nil, fmt.Errorf("failed to convert desiredPod from unstructured object: %v", err)
	}

	clusterPod, err := helper.ConvertToPod(observed)
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

	unCastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(desiredPod)
	if err != nil {
		return nil, fmt.Errorf("failed to transform Pod: %v", err)
	}

	desired.Object = unCastObj
	return desired, nil
}

// retainServiceFields updates the desired service object with values retained from the cluster object.
func retainServiceFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// healthCheckNodePort is allocated by APIServer and unchangeable, so it should be retained while updating
	if err := retainServiceHealthCheckNodePort(desired, observed); err != nil {
		return nil, err
	}

	// ClusterIP and NodePort are allocated to Service by cluster, so retain the same if any while updating
	if err := retainServiceClusterIP(desired, observed); err != nil {
		return nil, err
	}

	if err := retainServiceNodePort(desired, observed); err != nil {
		return nil, err
	}

	return desired, nil
}

func retainServiceHealthCheckNodePort(desired, observed *unstructured.Unstructured) error {
	healthCheckNodePort, ok, err := unstructured.NestedInt64(observed.Object, "spec", "healthCheckNodePort")
	if err != nil {
		return fmt.Errorf("error retrieving healthCheckNodePort from service: %w", err)
	}
	if ok && healthCheckNodePort > 0 {
		if err = unstructured.SetNestedField(desired.Object, healthCheckNodePort, "spec", "healthCheckNodePort"); err != nil {
			return fmt.Errorf("error setting healthCheckNodePort for service: %w", err)
		}
	}
	return nil
}

func retainServiceClusterIP(desired, observed *unstructured.Unstructured) error {
	clusterIP, ok, err := unstructured.NestedString(observed.Object, "spec", "clusterIP")
	if err != nil {
		return fmt.Errorf("error retrieving clusterIP from cluster service: %w", err)
	}
	// !ok could indicate that a cluster ip was not assigned
	if ok && clusterIP != "" {
		err = unstructured.SetNestedField(desired.Object, clusterIP, "spec", "clusterIP")
		if err != nil {
			return fmt.Errorf("error setting clusterIP for service: %w", err)
		}
	}
	return nil
}

func retainServiceNodePort(desired, observed *unstructured.Unstructured) error {
	clusterPorts, ok, err := unstructured.NestedSlice(observed.Object, "spec", "ports")
	if err != nil {
		return fmt.Errorf("error retrieving ports from cluster service: %w", err)
	}
	if !ok {
		return nil
	}

	var desiredPorts []interface{}
	desiredPorts, ok, err = unstructured.NestedSlice(desired.Object, "spec", "ports")
	if err != nil {
		return fmt.Errorf("error retrieving ports from service: %w", err)
	}
	if !ok {
		desiredPorts = []interface{}{}
	}
	for desiredIndex := range desiredPorts {
		for clusterIndex := range clusterPorts {
			fPort := desiredPorts[desiredIndex].(map[string]interface{})
			cPort := clusterPorts[clusterIndex].(map[string]interface{})
			if !(fPort["name"] == cPort["name"] && fPort["protocol"] == cPort["protocol"] && fPort["port"] == cPort["port"]) {
				continue
			}
			nodePort, ok := cPort["nodePort"]
			if ok {
				fPort["nodePort"] = nodePort
			}
		}
	}
	err = unstructured.SetNestedSlice(desired.Object, desiredPorts, "spec", "ports")
	if err != nil {
		return fmt.Errorf("error setting ports for service: %w", err)
	}

	return nil
}

// retainServiceAccountFields retains the 'secrets' field of a service account
// if the desired representation does not include a value for the field.  This
// ensures that the sync controller doesn't continually clear a generated
// secret from a service account, prompting continual regeneration by the
// service account controller in the member cluster.
func retainServiceAccountFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Check whether the secrets field is populated in the desired object.
	desiredSecrets, ok, err := unstructured.NestedSlice(desired.Object, SecretsField)
	if err != nil {
		return nil, fmt.Errorf("error retrieving secrets from desired service account: %w", err)
	}
	if ok && len(desiredSecrets) > 0 {
		// Field is populated, so an update to the target resource does not
		// risk triggering a race with the service account controller.
		return desired, nil
	}

	// Retrieve the secrets from the cluster object and retain them.
	secrets, ok, err := unstructured.NestedSlice(observed.Object, SecretsField)
	if err != nil {
		return nil, fmt.Errorf("error retrieving secrets from service account: %w", err)
	}
	if ok && len(secrets) > 0 {
		err := unstructured.SetNestedField(desired.Object, secrets, SecretsField)
		if err != nil {
			return nil, fmt.Errorf("error setting secrets for service account: %w", err)
		}
	}
	return desired, nil
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
