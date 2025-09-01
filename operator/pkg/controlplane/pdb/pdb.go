/*
Copyright 2025 The Karmada Authors.

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
package pdb

import (
	"context"
	"fmt"

	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
)

// EnsurePodDisruptionBudget ensures that a PodDisruptionBudget exists for the component
func EnsurePodDisruptionBudget(component, name, namespace string, commonSettings *operatorv1alpha1.CommonSettings, client clientset.Interface) error {
	if commonSettings == nil || commonSettings.PodDisruptionBudgetConfig == nil {
		// If no PDB config is specified, ensure any existing PDB is deleted
		pdbName := getPDBName(name, component)
		if err := deletePodDisruptionBudget(client, namespace, pdbName); err != nil {
			return fmt.Errorf("failed to delete existing PDB for component %s, err: %w", component, err)
		}
		return nil
	}

	pdb, err := createPodDisruptionBudget(name, namespace, component, commonSettings.PodDisruptionBudgetConfig)
	if err != nil {
		return fmt.Errorf("failed to create PDB manifest for component %s, err: %w", component, err)
	}

	if err := apiclient.CreateOrUpdatePodDisruptionBudget(client, pdb); err != nil {
		return fmt.Errorf("failed to create PDB resource for component %s, err: %w", component, err)
	}

	klog.V(2).InfoS("Successfully ensured PDB for component", "component", component, "name", pdb.Name, "namespace", namespace)
	return nil
}

// createPodDisruptionBudget creates a PodDisruptionBudget manifest for the component
func createPodDisruptionBudget(karmadaName, namespace, component string, pdbConfig *operatorv1alpha1.PodDisruptionBudgetConfig) (*policyv1.PodDisruptionBudget, error) {
	pdbName := getPDBName(karmadaName, component)

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pdbName,
			Namespace: namespace,
			Labels:    getComponentLabels(karmadaName, component),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: getComponentLabels(karmadaName, component),
			},
		},
	}

	// Set either minAvailable or maxUnavailable based on the configuration
	if pdbConfig.MinAvailable != nil {
		pdb.Spec.MinAvailable = pdbConfig.MinAvailable
	} else if pdbConfig.MaxUnavailable != nil {
		pdb.Spec.MaxUnavailable = pdbConfig.MaxUnavailable
	}

	return pdb, nil
}

// getPDBName returns the name for the PodDisruptionBudget resource
func getPDBName(karmadaName, component string) string {
	return fmt.Sprintf("%s-%s", karmadaName, component)
}

// getComponentLabels returns the labels for the component
// These labels must match the labels used in deployment templates
func getComponentLabels(karmadaName, component string) map[string]string {
	return map[string]string{
		constants.AppNameLabel:     getComponentAppName(component),
		constants.AppInstanceLabel: karmadaName,
	}
}

// getComponentAppName returns the app.kubernetes.io/name value for the component
// This must match the labels used in deployment templates
func getComponentAppName(component string) string {
	switch component {
	// Handle component type identifiers (used in controlplane.go)
	case constants.KarmadaControllerManagerComponent:
		return constants.KarmadaControllerManager
	case constants.KarmadaSchedulerComponent:
		return constants.KarmadaScheduler
	case constants.KarmadaDeschedulerComponent:
		return constants.KarmadaDescheduler
	case constants.KubeControllerManagerComponent:
		return constants.KubeControllerManager
	// Handle direct component names (used in other component files)
	case constants.KarmadaAPIServer:
		return constants.KarmadaAPIServer
	case constants.KarmadaAggregatedAPIServer:
		return constants.KarmadaAggregatedAPIServer
	case constants.KarmadaWebhook:
		return constants.KarmadaWebhook
	case constants.KarmadaSearch:
		return constants.KarmadaSearch
	case constants.KarmadaMetricsAdapter:
		return constants.KarmadaMetricsAdapter
	case constants.Etcd:
		return constants.Etcd
	default:
		return component
	}
}

// deletePodDisruptionBudget deletes a PodDisruptionBudget if it exists
func deletePodDisruptionBudget(client clientset.Interface, namespace, name string) error {
	err := client.PolicyV1().PodDisruptionBudgets(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}
