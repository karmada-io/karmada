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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
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

const (
	pullClusterBootstrapRetryInterval = time.Second
	pullClusterBootstrapTimeout       = 2 * time.Minute
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
func RegisterClusterInControllerPlane(opts ClusterRegisterOption, controlPlaneKubeClient kubeclient.Interface, controlPlaneClusterClient karmadaclientset.Interface, generateClusterInControllerPlane generateClusterInControllerPlaneFunc) error {
	clusterObj, err := generateClusterInControllerPlane(opts)
	if err != nil {
		return err
	}

	if clusterObj.Spec.SyncMode == clusterv1alpha1.Pull {
		return registerPullClusterInControllerPlane(opts, controlPlaneKubeClient, controlPlaneClusterClient, clusterObj)
	}

	return registerPushClusterInControllerPlane(opts, controlPlaneKubeClient, controlPlaneClusterClient, clusterObj)
}

func registerPushClusterInControllerPlane(opts ClusterRegisterOption, controlPlaneKubeClient kubeclient.Interface, controlPlaneClusterClient karmadaclientset.Interface, clusterObj *clusterv1alpha1.Cluster) error {
	// It's necessary to set the label of namespace to make sure that the namespace is created by Karmada.
	labels := map[string]string{
		KarmadaSystemLabel: KarmadaSystemLabelValue,
	}
	// ensure namespace where the cluster object be stored exists in control plane.
	if _, err := EnsureNamespaceExistWithLabels(controlPlaneKubeClient, opts.ClusterNamespace, opts.DryRun, labels); err != nil {
		return err
	}

	secretObj, impersonatorSecretObj := desiredControlPlaneSecrets(opts)
	secret, impersonatorSecret, err := createControlPlaneSecrets(controlPlaneKubeClient, secretObj, impersonatorSecretObj)
	if err != nil {
		return err
	}

	if secret != nil {
		clusterObj.Spec.SecretRef = &clusterv1alpha1.LocalSecretReference{
			Namespace: secret.Namespace,
			Name:      secret.Name,
		}
	}
	if impersonatorSecret != nil {
		clusterObj.Spec.ImpersonatorSecretRef = &clusterv1alpha1.LocalSecretReference{
			Namespace: impersonatorSecret.Namespace,
			Name:      impersonatorSecret.Name,
		}
	}

	cluster, err := CreateClusterObject(controlPlaneClusterClient, clusterObj)
	if err != nil {
		return fmt.Errorf("failed to create cluster(%s) object. error: %w", opts.ClusterName, err)
	}

	return bindControlPlaneSecretsToCluster(controlPlaneKubeClient, cluster, secret, impersonatorSecret)
}

func registerPullClusterInControllerPlane(opts ClusterRegisterOption, controlPlaneKubeClient kubeclient.Interface, controlPlaneClusterClient karmadaclientset.Interface, clusterObj *clusterv1alpha1.Cluster) error {
	cluster, err := createOrGetPullCluster(controlPlaneClusterClient, clusterObj)
	if err != nil {
		return fmt.Errorf("failed to create cluster(%s) object. error: %w", opts.ClusterName, err)
	}

	secretObj, impersonatorSecretObj := desiredControlPlaneSecrets(opts)

	var secret *corev1.Secret
	var impersonatorSecret *corev1.Secret
	err = wait.PollUntilContextTimeout(context.TODO(), pullClusterBootstrapRetryInterval, pullClusterBootstrapTimeout, false, func(context.Context) (bool, error) {
		if secretObj != nil || impersonatorSecretObj != nil {
			secret, impersonatorSecret, err = createControlPlaneSecrets(controlPlaneKubeClient, secretObj, impersonatorSecretObj)
			if err != nil {
				if isRetryablePullClusterBootstrapError(err) {
					return false, nil
				}
				return false, err
			}
		}

		if secret != nil || impersonatorSecret != nil {
			if err := bindControlPlaneSecretsToCluster(controlPlaneKubeClient, cluster, secret, impersonatorSecret); err != nil {
				if isRetryablePullClusterBootstrapError(err) {
					return false, nil
				}
				return false, err
			}
		}

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to finish pull mode cluster bootstrap for cluster(%s): %w", opts.ClusterName, err)
	}

	return nil
}

func createOrGetPullCluster(controlPlaneClusterClient karmadaclientset.Interface, clusterObj *clusterv1alpha1.Cluster) (*clusterv1alpha1.Cluster, error) {
	cluster, err := controlPlaneClusterClient.ClusterV1alpha1().Clusters().Create(context.TODO(), clusterObj, metav1.CreateOptions{})
	if err == nil {
		return cluster, nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return nil, err
	}

	existing, err := controlPlaneClusterClient.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterObj.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if existing.Spec.ID != clusterObj.Spec.ID {
		return nil, fmt.Errorf("cluster(%s) already exists with different ID", clusterObj.Name)
	}
	if existing.Spec.SyncMode != clusterv1alpha1.Pull {
		return nil, fmt.Errorf("cluster(%s) already exists in %s mode", clusterObj.Name, existing.Spec.SyncMode)
	}

	return existing, nil
}

func desiredControlPlaneSecrets(opts ClusterRegisterOption) (*corev1.Secret, *corev1.Secret) {
	labels := map[string]string{
		KarmadaSystemLabel: KarmadaSystemLabelValue,
	}

	var secret *corev1.Secret
	var impersonatorSecret *corev1.Secret

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
	}

	if opts.IsKubeImpersonatorEnabled() {
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
	}

	return secret, impersonatorSecret
}

func createControlPlaneSecrets(controlPlaneKubeClient kubeclient.Interface, secretObj, impersonatorSecretObj *corev1.Secret) (*corev1.Secret, *corev1.Secret, error) {
	var secret *corev1.Secret
	var impersonatorSecret *corev1.Secret
	var err error

	if secretObj != nil {
		secret, err = CreateSecret(controlPlaneKubeClient, secretObj)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create secret in control plane: %w", err)
		}
	}

	if impersonatorSecretObj != nil {
		impersonatorSecret, err = CreateSecret(controlPlaneKubeClient, impersonatorSecretObj)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create impersonator secret in control plane: %w", err)
		}
	}

	return secret, impersonatorSecret, nil
}

func bindControlPlaneSecretsToCluster(controlPlaneKubeClient kubeclient.Interface, cluster *clusterv1alpha1.Cluster, secret, impersonatorSecret *corev1.Secret) error {
	if cluster == nil {
		return fmt.Errorf("cluster is nil")
	}

	patchSecretBody := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterResourceKind),
			},
		},
	}

	if impersonatorSecret != nil {
		if err := PatchSecret(controlPlaneKubeClient, impersonatorSecret.Namespace, impersonatorSecret.Name, types.MergePatchType, patchSecretBody); err != nil {
			return fmt.Errorf("failed to patch impersonator secret %s/%s, error: %w", impersonatorSecret.Namespace, impersonatorSecret.Name, err)
		}
	}

	if secret != nil {
		if err := PatchSecret(controlPlaneKubeClient, secret.Namespace, secret.Name, types.MergePatchType, patchSecretBody); err != nil {
			return fmt.Errorf("failed to patch secret %s/%s, error: %w", secret.Namespace, secret.Name, err)
		}
	}

	return nil
}

func isRetryablePullClusterBootstrapError(err error) bool {
	return apierrors.IsConflict(err) || apierrors.IsForbidden(err) || apierrors.IsNotFound(err)
}
