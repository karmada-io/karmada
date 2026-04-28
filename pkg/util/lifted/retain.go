/*
Copyright 2020 The Kubernetes Authors.

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

// This code is directly lifted from the kubefed codebase.
// For reference:
// https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/sync/dispatch/retain.go#L48-L155

package lifted

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// SecretsField indicates the 'secrets' field of a service account
	SecretsField = "secrets"
	// ImagePullSecretsField indicates the 'imagePullSecrets' field of a service account
	ImagePullSecretsField = "imagePullSecrets"
)

// +lifted:source=https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/sync/dispatch/retain.go
// +lifted:changed

// RetainServiceFields updates the desired service object with values retained from the cluster object.
func RetainServiceFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// healthCheckNodePort is allocated by APIServer and unchangeable, so it should be retained while updating
	if err := retainServiceHealthCheckNodePort(desired, observed); err != nil {
		return nil, err
	}

	// ClusterIP is allocated to Service by cluster, so retain the same if any while updating
	if err := retainServiceClusterIP(desired, observed); err != nil {
		return nil, err
	}

	return desired, nil
}

// +lifted:source=https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/sync/dispatch/retain.go
// +lifted:changed
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

// +lifted:source=https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/sync/dispatch/retain.go
// +lifted:changed
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

// +lifted:source=https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/sync/dispatch/retain.go
// +lifted:changed

// RetainServiceAccountFields merges the 'secrets' field and the 'imagePullSecrets' field in the service account
// of the control plane and the member clusters and retains the merged service account. This
// ensures that the karmada-controller-manager doesn't continually clear a generated
// secret from a service account, prompting continual regeneration by the
// service account controller in the member cluster.
// Related issue:
// https://github.com/karmada-io/karmada/issues/2573
// https://github.com/karmada-io/karmada/issues/3370
func RetainServiceAccountFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	secretsFields := []string{
		SecretsField,
		ImagePullSecretsField,
	}

	for _, field := range secretsFields {
		if err := retainSecretsField(desired, observed, field); err != nil {
			return nil, err
		}
	}
	return desired, nil
}

// retainSecretsField merges a given secrets field (e.g. 'secrets' or 'imagePullSecrets') in the service account.
func retainSecretsField(desired, observed *unstructured.Unstructured, fieldName string) error {
	var mergedSecrets []interface{}
	isSecretExistMap := make(map[string]struct{})

	desiredSecrets, ok, err := unstructured.NestedSlice(desired.Object, fieldName)
	if err != nil {
		return fmt.Errorf("error retrieving %s from desired service account: %w", fieldName, err)
	}

	if ok && len(desiredSecrets) > 0 {
		for _, desiredSecret := range desiredSecrets {
			secretName := desiredSecret.(map[string]interface{})["name"].(string)
			mergedSecrets = append(mergedSecrets, desiredSecret)
			isSecretExistMap[secretName] = struct{}{}
		}
	}

	secrets, ok, err := unstructured.NestedSlice(observed.Object, fieldName)
	if err != nil {
		return fmt.Errorf("error retrieving %s from service account: %w", fieldName, err)
	}

	if ok && len(secrets) > 0 {
		for _, secret := range secrets {
			secretName := secret.(map[string]interface{})["name"].(string)
			if _, exist := isSecretExistMap[secretName]; exist {
				continue
			}
			mergedSecrets = append(mergedSecrets, secret)
			isSecretExistMap[secretName] = struct{}{}
		}
	}

	err = unstructured.SetNestedField(desired.Object, mergedSecrets, fieldName)
	if err != nil {
		return fmt.Errorf("error setting %s for service account: %w", fieldName, err)
	}
	return nil
}
