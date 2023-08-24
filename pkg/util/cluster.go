package util

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
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

// ClusterRegisterOption represents the option for RegistryCluster.
type ClusterRegisterOption struct {
	ClusterNamespace   string
	ClusterName        string
	ReportSecrets      []string
	ClusterAPIEndpoint string
	ProxyServerAddress string
	ClusterProvider    string
	ClusterRegion      string
	// Deprecated: Use ClusterZones instead.
	ClusterZone  string
	ClusterZones []string
	DryRun       bool

	ControlPlaneConfig *rest.Config
	ClusterConfig      *rest.Config
	Secret             corev1.Secret
	ImpersonatorSecret corev1.Secret
	ClusterID          string
}

// IsKubeCredentialsEnabled represents whether report secret
func (r ClusterRegisterOption) IsKubeCredentialsEnabled() bool {
	for _, sct := range r.ReportSecrets {
		if sct == KubeCredentials {
			return true
		}
	}
	return false
}

// IsKubeImpersonatorEnabled represents whether report impersonator secret
func (r ClusterRegisterOption) IsKubeImpersonatorEnabled() bool {
	for _, sct := range r.ReportSecrets {
		if sct == KubeImpersonator {
			return true
		}
	}
	return false
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
		if reflect.DeepEqual(cluster.Spec, clusterObj.Spec) {
			klog.Warningf("Cluster(%s) already exist and newest", clusterObj.Name)
			return cluster, nil
		}
		mutate(cluster)
		cluster, err = updateCluster(controlPlaneClient, cluster)
		if err != nil {
			klog.Warningf("Failed to create cluster(%s). error: %v", clusterObj.Name, err)
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

// ObtainClusterID returns the cluster ID property with clusterKubeClient
func ObtainClusterID(clusterKubeClient kubernetes.Interface) (string, error) {
	ns, err := clusterKubeClient.CoreV1().Namespaces().Get(context.TODO(), metav1.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(ns.UID), nil
}

// IsClusterIdentifyUnique checks whether the ClusterID exists in the karmada control plane.
func IsClusterIdentifyUnique(controlPlaneClient karmadaclientset.Interface, id string) (bool, string, error) {
	clusterList, err := controlPlaneClient.ClusterV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return false, "", err
	}

	for _, cluster := range clusterList.Items {
		if cluster.Spec.ID == id {
			return false, cluster.Name, nil
		}
	}
	return true, "", nil
}

// ClusterAccessCredentialChanged checks whether the cluster a ccess credential changed
func ClusterAccessCredentialChanged(newSpec, oldSpec clusterv1alpha1.ClusterSpec) bool {
	if oldSpec.APIEndpoint == newSpec.APIEndpoint &&
		oldSpec.InsecureSkipTLSVerification == newSpec.InsecureSkipTLSVerification &&
		oldSpec.ProxyURL == newSpec.ProxyURL &&
		equality.Semantic.DeepEqual(oldSpec.ProxyHeader, newSpec.ProxyHeader) {
		return false
	}
	return true
}
