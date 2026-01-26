/*
Copyright 2020 The Karmada Authors.

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
	"reflect"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// NamespaceClusterLease is the namespace which cluster lease are stored.
	NamespaceClusterLease = "karmada-cluster"
	// KubeCredentials is the secret that contains mandatory credentials whether reported when registering cluster
	KubeCredentials = "KubeCredentials"
	// KubeImpersonator is the secret that contains the token of impersonator whether reported when registering cluster
	KubeImpersonator = "KubeImpersonator"
	// None is means don't report any secrets.
	None = "None"
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

// ClusterRegisterOption represents the option for RegistryCluster.
type ClusterRegisterOption struct {
	ClusterNamespace   string
	ClusterName        string
	ClusterID          string
	ReportSecrets      []string
	ClusterAPIEndpoint string
	ProxyServerAddress string
	ClusterProvider    string
	ClusterRegion      string
	ClusterZones       []string
	DryRun             bool

	ControlPlaneConfig *rest.Config
	ClusterConfig      *rest.Config
	Secret             *corev1.Secret
	ImpersonatorSecret *corev1.Secret

	ControlPlaneKubeClient kubernetes.Interface
	ClusterKubeClient      kubernetes.Interface
	KarmadaClient          karmadaclientset.Interface
}

// Complete takes the command arguments and infers any remaining options.
func (r *ClusterRegisterOption) Complete() error {
	var err error

	if r.ControlPlaneKubeClient == nil {
		r.ControlPlaneKubeClient = kubernetes.NewForConfigOrDie(r.ControlPlaneConfig)
	}
	if r.ClusterKubeClient == nil {
		r.ClusterKubeClient = kubernetes.NewForConfigOrDie(r.ClusterConfig)
	}
	if r.KarmadaClient == nil {
		r.KarmadaClient = karmadaclientset.NewForConfigOrDie(r.ControlPlaneConfig)
	}

	r.ClusterID, err = ObtainClusterID(r.ClusterKubeClient)
	if err != nil {
		return err
	}

	return nil
}

// Validate validates the cluster register option, including clusterID, cluster name and so on.
func (r *ClusterRegisterOption) Validate(isAgent bool) error {
	clusterList, err := r.KarmadaClient.ClusterV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	clusterIDUsed, clusterNameUsed, sameCluster := r.validateCluster(clusterList)
	if isAgent && sameCluster {
		return nil
	}

	if clusterIDUsed || clusterNameUsed {
		return fmt.Errorf("the cluster ID %s or the cluster name %s has been registered", r.ClusterID, r.ClusterName)
	}

	return nil
}

// RunRegister runs the cluster registration process.
func (r *ClusterRegisterOption) RunRegister(generateClusterInControllerPlane generateClusterInControllerPlaneFunc) error {
	if r.DryRun {
		return nil
	}

	if r.IsKubeImpersonatorEnabled() || r.IsKubeCredentialsEnabled() {
		if err := r.EnsureCredentialNamespaceExist(); err != nil {
			return err
		}
	}

	if r.IsKubeImpersonatorEnabled() {
		if err := r.EnsureKubeImpersonatorExist(); err != nil {
			return err
		}
	}

	if r.IsKubeCredentialsEnabled() {
		if err := r.EnsureKubeCredentialsExist(); err != nil {
			return err
		}
	}

	cluster, err := generateClusterInControllerPlane(*r)
	if err != nil {
		return err
	}

	// Since OwnerReferences requires the UID of the cluster, patching should be done after creating the cluster object.
	patchSecretBody := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterResourceKind),
			},
		},
	}
	if r.IsKubeImpersonatorEnabled() {
		err = PatchSecret(r.ControlPlaneKubeClient, r.ImpersonatorSecret.Namespace, r.ImpersonatorSecret.Name, types.MergePatchType, patchSecretBody)
		if err != nil {
			return fmt.Errorf("failed to patch impersonator secret %s/%s, error: %v", r.ImpersonatorSecret.Namespace, r.ImpersonatorSecret.Name, err)
		}
	}

	if r.IsKubeCredentialsEnabled() {
		err = PatchSecret(r.ControlPlaneKubeClient, r.Secret.Namespace, r.Secret.Name, types.MergePatchType, patchSecretBody)
		if err != nil {
			return fmt.Errorf("failed to patch secret %s/%s, error: %v", r.Secret.Namespace, r.Secret.Name, err)
		}
	}

	return nil
}

// EnsureCredentialNamespaceExist ensures that the namespace for storing cluster credentials exists in both the cluster and control plane.
func (r *ClusterRegisterOption) EnsureCredentialNamespaceExist() error {
	// It's necessary to set the label of namespace to make sure that the namespace is created by Karmada.
	labels := map[string]string{
		KarmadaSystemLabel: KarmadaSystemLabelValue,
	}
	// ensure namespace where the karmada control plane credential be stored exists in cluster.
	_, err := EnsureNamespaceExistWithLabels(r.ClusterKubeClient, r.ClusterNamespace, r.DryRun, labels)
	if err != nil {
		return err
	}

	// ensure namespace where the cluster object be stored exists in control plane.
	_, err = EnsureNamespaceExistWithLabels(r.ControlPlaneKubeClient, r.ClusterNamespace, r.DryRun, labels)
	return err
}

// EnsureKubeImpersonatorExist ensures that the kube impersonator secret exists in the cluster and control plane.
func (r *ClusterRegisterOption) EnsureKubeImpersonatorExist() error {
	var impersonationSA *corev1.ServiceAccount
	var impersonatorSecret *corev1.Secret
	var err error

	// It's necessary to set the label of namespace to make sure that the namespace is created by Karmada.
	labels := map[string]string{
		KarmadaSystemLabel: KarmadaSystemLabelValue,
	}

	impersonationSA = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.ClusterNamespace,
			Name:      names.GenerateServiceAccountName("impersonator"),
			Labels:    labels,
		},
	}
	if impersonationSA, err = EnsureServiceAccountExist(r.ClusterKubeClient, impersonationSA, r.DryRun); err != nil {
		return err
	}

	impersonatorSecret, err = WaitForServiceAccountSecretCreation(r.ClusterKubeClient, impersonationSA)
	if err != nil {
		return fmt.Errorf("failed to get serviceAccount secret for impersonation from cluster(%s), error: %v", r.ClusterName, err)
	}
	// create secret to store impersonation info in control plane
	impersonatorSecretInControlPlane := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.ClusterNamespace,
			Name:      names.GenerateImpersonationSecretName(r.ClusterName),
			Labels:    labels,
		},
		Data: map[string][]byte{
			clusterv1alpha1.SecretTokenKey: impersonatorSecret.Data[clusterv1alpha1.SecretTokenKey],
		},
	}
	impersonatorSecretInControlPlane, err = CreateSecret(r.ControlPlaneKubeClient, impersonatorSecretInControlPlane)
	if err != nil {
		return fmt.Errorf("failed to create impersonator secret in control plane. error: %v", err)
	}

	r.ImpersonatorSecret = impersonatorSecretInControlPlane
	return nil
}

// EnsureKubeCredentialsExist ensures the kube credentials exist in the cluster and control plane.
func (r *ClusterRegisterOption) EnsureKubeCredentialsExist() error {
	var clusterSecret *corev1.Secret
	var err error

	// It's necessary to set the label of namespace to make sure that the namespace is created by Karmada.
	labels := map[string]string{
		KarmadaSystemLabel: KarmadaSystemLabelValue,
	}

	// create a ServiceAccount in cluster.
	serviceAccountObj := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.ClusterNamespace,
			Name:      names.GenerateServiceAccountName(r.ClusterName),
			Labels:    labels,
		},
	}
	if serviceAccountObj, err = EnsureServiceAccountExist(r.ClusterKubeClient, serviceAccountObj, r.DryRun); err != nil {
		return err
	}

	// create a ClusterRole in cluster.
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   names.GenerateRoleName(serviceAccountObj.Name),
			Labels: labels,
		},
		Rules: ClusterPolicyRules,
	}
	if _, err = EnsureClusterRoleExist(r.ClusterKubeClient, clusterRole, r.DryRun); err != nil {
		return err
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
	if _, err = EnsureClusterRoleBindingExist(r.ClusterKubeClient, clusterRoleBinding, r.DryRun); err != nil {
		return err
	}
	clusterSecret, err = WaitForServiceAccountSecretCreation(r.ClusterKubeClient, serviceAccountObj)
	if err != nil {
		return fmt.Errorf("failed to get serviceAccount secret from cluster(%s), error: %v", r.ClusterName, err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.ClusterNamespace,
			Name:      r.ClusterName,
			Labels:    labels,
		},
		Data: map[string][]byte{
			clusterv1alpha1.SecretCADataKey: clusterSecret.Data["ca.crt"],
			clusterv1alpha1.SecretTokenKey:  clusterSecret.Data[clusterv1alpha1.SecretTokenKey],
		},
	}

	secret, err = CreateSecret(r.ControlPlaneKubeClient, secret)
	if err != nil {
		return fmt.Errorf("failed to create secret in control plane. error: %v", err)
	}

	r.Secret = secret
	return nil
}

// IsKubeCredentialsEnabled represents whether report secret
func (r *ClusterRegisterOption) IsKubeCredentialsEnabled() bool {
	for _, sct := range r.ReportSecrets {
		if sct == KubeCredentials {
			return true
		}
	}
	return false
}

// IsKubeImpersonatorEnabled represents whether report impersonator secret
func (r *ClusterRegisterOption) IsKubeImpersonatorEnabled() bool {
	for _, sct := range r.ReportSecrets {
		if sct == KubeImpersonator {
			return true
		}
	}
	return false
}

// validateCluster validates the cluster register option whether the cluster name and cluster ID are unique.
// 1. When registering a cluster for the first time, the metrics `clusterIDUsed` and `clusterNameUsed` can be used
// to check if the cluster ID and the cluster name have already been used, which can avoid duplicate registrations.
// 2. In cases where the agent is restarted, the metric `sameCluster` can be used to determine if the cluster
// specified in the `RegisterOption` has already been registered, aiming to achieve the purpose of re-entering and updating the cluster.
func (r *ClusterRegisterOption) validateCluster(clusterList *clusterv1alpha1.ClusterList) (clusterIDUsed, clusterNameUsed, sameCluster bool) {
	for _, cluster := range clusterList.Items {
		if cluster.Spec.ID == r.ClusterID && cluster.GetName() == r.ClusterName {
			return true, true, true
		}
		if cluster.Spec.ID == r.ClusterID {
			clusterIDUsed = true
		}
		if cluster.GetName() == r.ClusterName {
			clusterNameUsed = true
		}
	}

	return clusterIDUsed, clusterNameUsed, false
}

// IsClusterReady tells whether the cluster status in 'Ready' condition.
func IsClusterReady(clusterStatus *clusterv1alpha1.ClusterStatus) bool {
	return meta.IsStatusConditionTrue(clusterStatus.Conditions, clusterv1alpha1.ClusterConditionReady)
}

// GetCluster returns the given Cluster resource
func GetCluster(hostClient client.Client, clusterName string) (*clusterv1alpha1.Cluster, error) {
	cluster := &clusterv1alpha1.Cluster{}
	if err := hostClient.Get(context.TODO(), types.NamespacedName{Name: clusterName}, cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}

// GetClusterSet returns the given Clusters name set
func GetClusterSet(hostClient client.Client) (sets.Set[string], error) {
	clusterList := &clusterv1alpha1.ClusterList{}
	if err := hostClient.List(context.Background(), clusterList); err != nil {
		return nil, err
	}
	clusterSet := sets.New[string]()
	for _, cluster := range clusterList.Items {
		clusterSet.Insert(cluster.Name)
	}
	return clusterSet, nil
}

// CreateClusterObject create cluster object in karmada control plane
func CreateClusterObject(controlPlaneClient karmadaclientset.Interface, clusterObj *clusterv1alpha1.Cluster) (*clusterv1alpha1.Cluster, error) {
	cluster, exist, err := GetClusterWithKarmadaClient(controlPlaneClient, clusterObj.Name)
	if err != nil {
		return nil, err
	}

	if exist {
		return cluster, fmt.Errorf("cluster(%s) already exist", clusterObj.Name)
	}

	if cluster, err = createCluster(controlPlaneClient, clusterObj); err != nil {
		klog.Warningf("Failed to create cluster(%s). error: %v", clusterObj.Name, err)
		return nil, err
	}

	return cluster, nil
}

// CreateOrUpdateClusterObject create cluster object in karmada control plane,
// if cluster object has been existed and different from input clusterObj, update it.
func CreateOrUpdateClusterObject(controlPlaneClient karmadaclientset.Interface, clusterObj *clusterv1alpha1.Cluster, mutate func(*clusterv1alpha1.Cluster)) (*clusterv1alpha1.Cluster, error) {
	cluster, exist, err := GetClusterWithKarmadaClient(controlPlaneClient, clusterObj.Name)
	if err != nil {
		return nil, err
	}
	if exist {
		clusterCopy := cluster.DeepCopy()
		mutate(cluster)
		if reflect.DeepEqual(clusterCopy.Spec, cluster.Spec) {
			klog.Warningf("Cluster(%s) already exist and newest", clusterObj.Name)
			return cluster, nil
		}

		cluster, err = updateCluster(controlPlaneClient, cluster)
		if err != nil {
			klog.Warningf("Failed to update cluster(%s). error: %v", clusterObj.Name, err)
			return nil, err
		}
		return cluster, nil
	}

	mutate(clusterObj)
	if cluster, err = createCluster(controlPlaneClient, clusterObj); err != nil {
		klog.Warningf("Failed to create cluster(%s). error: %v", clusterObj.Name, err)
		return nil, err
	}
	return cluster, nil
}

// GetClusterWithKarmadaClient tells if a cluster already joined to control plane.
func GetClusterWithKarmadaClient(client karmadaclientset.Interface, name string) (*clusterv1alpha1.Cluster, bool, error) {
	cluster, err := client.ClusterV1alpha1().Clusters().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}

		klog.Warningf("Failed to retrieve cluster(%s). error: %v", name, err)
		return nil, false, err
	}

	return cluster, true, nil
}

func createCluster(controlPlaneClient karmadaclientset.Interface, cluster *clusterv1alpha1.Cluster) (*clusterv1alpha1.Cluster, error) {
	newCluster, err := controlPlaneClient.ClusterV1alpha1().Clusters().Create(context.TODO(), cluster, metav1.CreateOptions{})
	if err != nil {
		klog.Warningf("Failed to create cluster(%s). error: %v", cluster.Name, err)
		return nil, err
	}

	return newCluster, nil
}

func updateCluster(controlPlaneClient karmadaclientset.Interface, cluster *clusterv1alpha1.Cluster) (*clusterv1alpha1.Cluster, error) {
	newCluster, err := controlPlaneClient.ClusterV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{})
	if err != nil {
		klog.Warningf("Failed to update cluster(%s). error: %v", cluster.Name, err)
		return nil, err
	}

	return newCluster, nil
}

// ObtainClusterID returns the cluster ID property with ClusterKubeClient
func ObtainClusterID(clusterKubeClient kubernetes.Interface) (string, error) {
	ns, err := clusterKubeClient.CoreV1().Namespaces().Get(context.TODO(), metav1.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(ns.UID), nil
}

// ClusterAccessCredentialChanged checks whether the cluster access credential changed
func ClusterAccessCredentialChanged(newSpec, oldSpec clusterv1alpha1.ClusterSpec) bool {
	if oldSpec.APIEndpoint == newSpec.APIEndpoint &&
		oldSpec.InsecureSkipTLSVerification == newSpec.InsecureSkipTLSVerification &&
		oldSpec.ProxyURL == newSpec.ProxyURL &&
		equality.Semantic.DeepEqual(oldSpec.ProxyHeader, newSpec.ProxyHeader) {
		return false
	}
	return true
}
