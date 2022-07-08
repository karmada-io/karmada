package util

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclient "k8s.io/client-go/kubernetes"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var (
	clusterResourceKind = clusterv1alpha1.SchemeGroupVersion.WithKind("Cluster")

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
	// ensure namespace where the karmada control plane credential be stored exists in cluster.
	if _, err = EnsureNamespaceExist(clusterKubeClient, opts.ClusterNamespace, opts.DryRun); err != nil {
		return nil, nil, err
	}

	if opts.IsKubeImpersonatorEnabled() {
		// create a ServiceAccount for impersonation in cluster.
		impersonationSA := &corev1.ServiceAccount{}
		impersonationSA.Namespace = opts.ClusterNamespace
		impersonationSA.Name = names.GenerateServiceAccountName("impersonator")
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
		serviceAccountObj := &corev1.ServiceAccount{}
		serviceAccountObj.Namespace = opts.ClusterNamespace
		serviceAccountObj.Name = names.GenerateServiceAccountName(opts.ClusterName)
		if serviceAccountObj, err = EnsureServiceAccountExist(clusterKubeClient, serviceAccountObj, opts.DryRun); err != nil {
			return nil, nil, err
		}

		// create a ClusterRole in cluster.
		clusterRole := &rbacv1.ClusterRole{}
		clusterRole.Name = names.GenerateRoleName(serviceAccountObj.Name)
		clusterRole.Rules = ClusterPolicyRules
		if _, err = EnsureClusterRoleExist(clusterKubeClient, clusterRole, opts.DryRun); err != nil {
			return nil, nil, err
		}

		// create a ClusterRoleBinding in cluster.
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
		clusterRoleBinding.Name = clusterRole.Name
		clusterRoleBinding.Subjects = BuildRoleBindingSubjects(serviceAccountObj.Name, serviceAccountObj.Namespace)
		clusterRoleBinding.RoleRef = BuildClusterRoleReference(clusterRole.Name)
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
	impersonatorSecret := &corev1.Secret{}
	secret := &corev1.Secret{}
	var err error

	if opts.IsKubeImpersonatorEnabled() {
		// create secret to store impersonation info in control plane
		impersonatorSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: opts.ClusterNamespace,
				Name:      names.GenerateImpersonationSecretName(opts.ClusterName),
			},
			Data: map[string][]byte{
				clusterv1alpha1.SecretTokenKey: opts.ImpersonatorSecret.Data[clusterv1alpha1.SecretTokenKey],
			},
		}
		impersonatorSecret, err = CreateSecret(controlPlaneKubeClient, impersonatorSecret)
		if err != nil {
			return fmt.Errorf("failed to create impersonator secret in control plane. error: %v", err)
		}
		opts.ImpersonatorSecret = *impersonatorSecret
	}

	if opts.IsKubeCredentialsEnabled() {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: opts.ClusterNamespace,
				Name:      opts.ClusterName,
			},
			Data: map[string][]byte{
				clusterv1alpha1.SecretCADataKey: opts.Secret.Data["ca.crt"],
				clusterv1alpha1.SecretTokenKey:  opts.Secret.Data[clusterv1alpha1.SecretTokenKey],
			},
		}

		secret, err = CreateSecret(controlPlaneKubeClient, secret)
		if err != nil {
			return fmt.Errorf("failed to create secret in control plane. error: %v", err)
		}
		opts.Secret = *secret
	}
	cluster, err := generateClusterInControllerPlane(opts)
	if err != nil {
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
