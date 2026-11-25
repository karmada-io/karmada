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

package util

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var (
	clusterResourceKind = schema.GroupVersion{Group: clusterv1alpha1.GroupVersion.Group, Version: clusterv1alpha1.GroupVersion.Version}.WithKind("Cluster")

	// Policy rules allowing full access to resources in the cluster or namespace.
	namespacedPolicyRules = []rbacv1.PolicyRule{
		{
			Verbs:     []string{rbacv1.VerbAll},
			APIGroups: []string{rbacv1.APIGroupAll},
			Resources: []string{rbacv1.ResourceAll},
		},
	}
	// ClusterPolicyRules represents cluster policy rules
	ClusterPolicyRules = []rbacv1.PolicyRule{
		namespacedPolicyRules[0],
		{
			NonResourceURLs: []string{rbacv1.NonResourceAll},
			Verbs:           []string{"get"},
		},
	}
)

type generateClusterInControllerPlaneFunc func(opts ClusterRegisterOption) (*clusterv1alpha1.Cluster, error)

// ObtainCredentialsFromMemberCluster obtain credentials for member cluster
func ObtainCredentialsFromMemberCluster(clusterKubeClient kubeclient.Interface, opts ClusterRegisterOption) (*corev1.Secret, *corev1.Secret, error) {
	var impersonatorSecret *corev1.Secret
	var clusterSecret *corev1.Secret
	var err error
	// It's necessary to set the label of namespace to make sure that the namespace is created by Karmada.
	labels := map[string]string{
		KarmadaSystemLabel: KarmadaSystemLabelValue,
	}
	// ensure namespace where the karmada control plane credential be stored exists in cluster.
	if _, err = EnsureNamespaceExistWithLabels(clusterKubeClient, opts.ClusterNamespace, opts.DryRun, labels); err != nil {
		return nil, nil, err
	}

	if opts.IsKubeImpersonatorEnabled() {
		// create a ServiceAccount for impersonation in cluster.
		impersonationSA := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: opts.ClusterNamespace,
				Name:      names.GenerateServiceAccountName("impersonator"),
				Labels:    labels,
			},
		}
		if impersonationSA, err = EnsureServiceAccountExist(clusterKubeClient, impersonationSA, opts.DryRun); err != nil {
			return nil, nil, err
		}

		impersonatorSecret, err = WaitForServiceAccountSecretCreation(clusterKubeClient, impersonationSA)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get serviceAccount secret for impersonation from cluster(%s), error: %v", opts.ClusterName, err)
		}
	}
	if opts.IsKubeCredentialsEnabled() {
		// create a ServiceAccount in cluster.
		serviceAccountObj := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: opts.ClusterNamespace,
				Name:      names.GenerateServiceAccountName(opts.ClusterName),
				Labels:    labels,
			},
		}
		if serviceAccountObj, err = EnsureServiceAccountExist(clusterKubeClient, serviceAccountObj, opts.DryRun); err != nil {
			return nil, nil, err
		}

		// create a ClusterRole in cluster.
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   names.GenerateRoleName(serviceAccountObj.Name),
				Labels: labels,
			},
			Rules: ClusterPolicyRules,
		}
		if _, err = EnsureClusterRoleExist(clusterKubeClient, clusterRole, opts.DryRun); err != nil {
			return nil, nil, err
		}

		// create a ClusterRoleBinding in cluster.
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   clusterRole.Name,
				Labels: labels,
			},
			Subjects: BuildRoleBindingSubjects(serviceAccountObj.Name, serviceAccountObj.Namespace),
			RoleRef:  BuildClusterRoleReference(clusterRole.Name),
		}
		if _, err = EnsureClusterRoleBindingExist(clusterKubeClient, clusterRoleBinding, opts.DryRun); err != nil {
			return nil, nil, err
		}
		clusterSecret, err = WaitForServiceAccountSecretCreation(clusterKubeClient, serviceAccountObj)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get serviceAccount secret from cluster(%s), error: %v", opts.ClusterName, err)
		}
	}
	if opts.DryRun {
		return nil, nil, nil
	}
	return clusterSecret, impersonatorSecret, nil
}

// RegisterClusterInControllerPlane represents register cluster in controller plane
func RegisterClusterInControllerPlane(opts ClusterRegisterOption, controlPlaneKubeClient kubeclient.Interface, generateClusterInControllerPlane generateClusterInControllerPlaneFunc) error {
	// It's necessary to set the label of namespace to make sure that the namespace is created by Karmada.
	labels := map[string]string{
		KarmadaSystemLabel: KarmadaSystemLabelValue,
	}
	// ensure namespace where the cluster object be stored exists in control plane.
	if _, err := EnsureNamespaceExistWithLabels(controlPlaneKubeClient, opts.ClusterNamespace, opts.DryRun, labels); err != nil {
		return err
	}

	impersonatorSecret := &corev1.Secret{}
	secret := &corev1.Secret{}
	var err error
	var created bool
	var createdSecrets []*corev1.Secret
	attemptID := string(uuid.NewUUID())
	cleanupSecrets := func() { rollbackRegistrationSecrets(controlPlaneKubeClient, createdSecrets) }

	if opts.IsKubeImpersonatorEnabled() {
		// create secret to store impersonation info in control plane
		impersonatorSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: opts.ClusterNamespace,
				Name:      names.GenerateImpersonationSecretName(opts.ClusterName),
				Labels:    labels,
			},
			Data: map[string][]byte{
				clusterv1alpha1.SecretTokenKey: opts.ImpersonatorSecret.Data[clusterv1alpha1.SecretTokenKey],
			},
		}
		impersonatorSecret, created, err = claimRegistrationSecret(controlPlaneKubeClient, impersonatorSecret, opts.ClusterName, attemptID)
		if err != nil {
			return fmt.Errorf("failed to create impersonator secret in control plane. error: %w", err)
		}
		opts.ImpersonatorSecret = *impersonatorSecret
		if created {
			createdSecrets = append(createdSecrets, impersonatorSecret)
		}
	}

	if opts.IsKubeCredentialsEnabled() {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: opts.ClusterNamespace,
				Name:      opts.ClusterName,
				Labels:    labels,
			},
			Data: map[string][]byte{
				clusterv1alpha1.SecretCADataKey: opts.Secret.Data["ca.crt"],
				clusterv1alpha1.SecretTokenKey:  opts.Secret.Data[clusterv1alpha1.SecretTokenKey],
			},
		}

		secret, created, err = claimRegistrationSecret(controlPlaneKubeClient, secret, opts.ClusterName, attemptID)
		if err != nil {
			cleanupSecrets()
			return fmt.Errorf("failed to create secret in control plane. error: %w", err)
		}
		opts.Secret = *secret
		if created {
			createdSecrets = append(createdSecrets, secret)
		}
	}
	cluster, err := generateClusterInControllerPlane(opts)
	if err != nil {
		cleanupSecrets()
		return err
	}
	patchSecretBody := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterResourceKind),
			},
		},
	}
	if opts.IsKubeImpersonatorEnabled() {
		err = PatchSecret(controlPlaneKubeClient, impersonatorSecret.Namespace, impersonatorSecret.Name, types.MergePatchType, patchSecretBody)
		if err != nil {
			return fmt.Errorf("failed to patch impersonator secret %s/%s, error: %v", impersonatorSecret.Namespace, impersonatorSecret.Name, err)
		}
	}

	if opts.IsKubeCredentialsEnabled() {
		err = PatchSecret(controlPlaneKubeClient, secret.Namespace, secret.Name, types.MergePatchType, patchSecretBody)
		if err != nil {
			return fmt.Errorf("failed to patch secret %s/%s, error: %v", secret.Namespace, secret.Name, err)
		}
	}
	return nil
}

// claimRegistrationSecret advances the version even when credentials are unchanged.
// Never retry a stale claim: another registration may already be using this version.
func claimRegistrationSecret(client kubeclient.Interface, desired *corev1.Secret, clusterName, attemptID string) (*corev1.Secret, bool, error) {
	const attemptAnnotation = "karmada.io/registration-attempt"
	desired.Annotations = map[string]string{attemptAnnotation: attemptID}
	secrets := client.CoreV1().Secrets(desired.Namespace)
	result, err := secrets.Create(context.TODO(), desired, metav1.CreateOptions{})
	if err == nil {
		return result, true, nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return nil, false, err
	}
	existing, err := secrets.Get(context.TODO(), desired.Name, metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	if owner := metav1.GetControllerOf(existing); owner != nil && (owner.APIVersion != clusterResourceKind.GroupVersion().String() || owner.Kind != clusterResourceKind.Kind || owner.Name != clusterName) {
		return nil, false, apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, desired.Name, fmt.Errorf("secret is controlled by %s %s, expected Cluster %s", owner.Kind, owner.Name, clusterName))
	}
	data := make(map[string]any, len(existing.Data)+len(desired.Data))
	for key := range existing.Data {
		data[key] = nil
	}
	for key, value := range desired.Data {
		data[key] = value
	}
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"resourceVersion": existing.ResourceVersion, "annotations": desired.Annotations},
		"data":     data,
	})
	if err != nil {
		return nil, false, err
	}
	result, err = secrets.Patch(context.TODO(), desired.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	return result, false, err
}

func rollbackRegistrationSecrets(client kubeclient.Interface, secrets []*corev1.Secret) {
	for _, secret := range secrets {
		options := metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &secret.UID, ResourceVersion: &secret.ResourceVersion}}
		if err := client.CoreV1().Secrets(secret.Namespace).Delete(context.TODO(), secret.Name, options); err != nil && !apierrors.IsNotFound(err) {
			klog.Errorf("Failed to roll back secret %s/%s: %v", secret.Namespace, secret.Name, err)
		}
	}
}
