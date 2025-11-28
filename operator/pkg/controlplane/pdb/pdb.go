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
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
)

// EnsurePodDisruptionBudget ensures the PodDisruptionBudget for a given component.
// If pdbConfig is nil, it deletes any existing PDB for the component.
// The owner parameter should be the Deployment or StatefulSet that owns this PDB.
func EnsurePodDisruptionBudget(client clientset.Interface, pdbName, namespace string, pdbConfig *operatorv1alpha1.PodDisruptionBudgetConfig, componentLabels map[string]string, ownerRefs []metav1.OwnerReference) error {
	if pdbConfig == nil {
		if err := deletePodDisruptionBudget(client, namespace, pdbName); err != nil {
			return fmt.Errorf("failed to delete existing PDB for %s, err: %w", pdbName, err)
		}
		return nil
	}

	pdb, err := createPodDisruptionBudget(pdbName, namespace, pdbConfig, componentLabels, ownerRefs)
	if err != nil {
		return fmt.Errorf("failed to create PDB manifest for %s, err: %w", pdbName, err)
	}

	if err := apiclient.CreateOrUpdatePodDisruptionBudget(client, pdb); err != nil {
		return fmt.Errorf("failed to create PDB resource for %s, err: %w", pdbName, err)
	}

	klog.V(2).InfoS("Successfully ensured PDB for component", "name", pdb.Name, "namespace", namespace)
	return nil
}

// createPodDisruptionBudget creates a PodDisruptionBudget manifest for the component
func createPodDisruptionBudget(pdbName, namespace string, pdbConfig *operatorv1alpha1.PodDisruptionBudgetConfig, componentLabels map[string]string, ownerRefs []metav1.OwnerReference) (*policyv1.PodDisruptionBudget, error) {
	blockOwnerDeletion := true
	isController := true

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pdbName,
			Namespace:       namespace,
			Labels:          componentLabels,
			OwnerReferences: ownerRefs,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: componentLabels,
			},
		},
	}

	if pdbConfig.MinAvailable != nil {
		pdb.Spec.MinAvailable = pdbConfig.MinAvailable
	} else if pdbConfig.MaxUnavailable != nil {
		pdb.Spec.MaxUnavailable = pdbConfig.MaxUnavailable
	}

	// Ensure controller & blockOwnerDeletion flags on provided refs
	for i := range pdb.ObjectMeta.OwnerReferences {
		pdb.ObjectMeta.OwnerReferences[i].Controller = &isController
		pdb.ObjectMeta.OwnerReferences[i].BlockOwnerDeletion = &blockOwnerDeletion
	}

	return pdb, nil
}

// deletePodDisruptionBudget deletes a PodDisruptionBudget if it exists
func deletePodDisruptionBudget(client clientset.Interface, namespace, name string) error {
	err := client.PolicyV1().PodDisruptionBudgets(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}
