package objectwatcher

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

/*
This code is directly lifted from the kubefed codebase. It's a list of functions to update the desired object with values retained
from the cluster object.
For reference: https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/sync/dispatch/retain.go#L27-L133
*/

const (
	// ServiceKind indicates the target resource is a service
	ServiceKind = "Service"
	// PodKind indicates the target resource is a pod
	PodKind = "Pod"
	// ServiceAccountKind indicates the target resource is a serviceaccount
	ServiceAccountKind = "ServiceAccount"
	// SecretsField indicates the 'secrets' field of a service account
	SecretsField = "secrets"
)

// RetainClusterFields updates the desired object with values retained
// from the cluster object.
func RetainClusterFields(desiredObj, clusterObj *unstructured.Unstructured) error {
	targetKind := desiredObj.GetKind()
	// Pass the same ResourceVersion as in the cluster object for update operation, otherwise operation will fail.
	desiredObj.SetResourceVersion(clusterObj.GetResourceVersion())

	// Retain finalizers and annotations since they will typically be set by
	// controllers in a member cluster.  It is still possible to set the fields
	// via overrides.
	desiredObj.SetFinalizers(clusterObj.GetFinalizers())
	desiredObj.SetAnnotations(clusterObj.GetAnnotations())

	if targetKind == PodKind {
		return retainPodFields(desiredObj, clusterObj)
	}
	if targetKind == ServiceKind {
		return retainServiceFields(desiredObj, clusterObj)
	}
	if targetKind == ServiceAccountKind {
		return retainServiceAccountFields(desiredObj, clusterObj)
	}
	return nil
}

func retainPodFields(desiredObj, clusterObj *unstructured.Unstructured) error {
	nodeName, ok, err := unstructured.NestedString(clusterObj.Object, "spec", "nodeName")
	if err != nil {
		return fmt.Errorf("error retrieving nodeName from cluster pod: %w", err)
	}
	if ok && nodeName != "" {
		err := unstructured.SetNestedField(desiredObj.Object, nodeName, "spec", "nodeName")
		if err != nil {
			return fmt.Errorf("error setting nodeName for pod: %w", err)
		}
	}
	return nil
}

func retainServiceFields(desiredObj, clusterObj *unstructured.Unstructured) error {
	// healthCheckNodePort is allocated by APIServer and unchangeable, so it should be retained while updating
	healthCheckNodePort, ok, err := unstructured.NestedInt64(clusterObj.Object, "spec", "healthCheckNodePort")
	if err != nil {
		return fmt.Errorf("error retrieving healthCheckNodePort from service: %w", err)
	}
	if ok && healthCheckNodePort > 0 {
		if err = unstructured.SetNestedField(desiredObj.Object, healthCheckNodePort, "spec", "healthCheckNodePort"); err != nil {
			return fmt.Errorf("error setting healthCheckNodePort for service: %w", err)
		}
	}

	// ClusterIP and NodePort are allocated to Service by cluster, so retain the same if any while updating

	// Retain clusterip
	clusterIP, ok, err := unstructured.NestedString(clusterObj.Object, "spec", "clusterIP")
	if err != nil {
		return fmt.Errorf("error retrieving clusterIP from cluster service: %w", err)
	}
	// !ok could indicate that a cluster ip was not assigned
	if ok && clusterIP != "" {
		err := unstructured.SetNestedField(desiredObj.Object, clusterIP, "spec", "clusterIP")
		if err != nil {
			return fmt.Errorf("error setting clusterIP for service: %w", err)
		}
	}

	// Retain nodeports
	clusterPorts, ok, err := unstructured.NestedSlice(clusterObj.Object, "spec", "ports")
	if err != nil {
		return fmt.Errorf("error retrieving ports from cluster service: %w", err)
	}
	if !ok {
		return nil
	}
	var desiredPorts []interface{}
	desiredPorts, ok, err = unstructured.NestedSlice(desiredObj.Object, "spec", "ports")
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
	err = unstructured.SetNestedSlice(desiredObj.Object, desiredPorts, "spec", "ports")
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
func retainServiceAccountFields(desiredObj, clusterObj *unstructured.Unstructured) error {
	// Check whether the secrets field is populated in the desired object.
	desiredSecrets, ok, err := unstructured.NestedSlice(desiredObj.Object, SecretsField)
	if err != nil {
		return fmt.Errorf("error retrieving secrets from desired service account: %w", err)
	}
	if ok && len(desiredSecrets) > 0 {
		// Field is populated, so an update to the target resource does not
		// risk triggering a race with the service account controller.
		return nil
	}

	// Retrieve the secrets from the cluster object and retain them.
	secrets, ok, err := unstructured.NestedSlice(clusterObj.Object, SecretsField)
	if err != nil {
		return fmt.Errorf("error retrieving secrets from service account: %w", err)
	}
	if ok && len(secrets) > 0 {
		err := unstructured.SetNestedField(desiredObj.Object, secrets, SecretsField)
		if err != nil {
			return fmt.Errorf("error setting secrets for service account: %w", err)
		}
	}
	return nil
}
