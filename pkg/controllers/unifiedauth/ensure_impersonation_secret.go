package unifiedauth

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// ensureImpersonationSecret make sure create impersonation secret for all Cluster.
// This logic is used only in the upgrade scenario of the current version
// and can be deleted in the next version.
func (c *Controller) ensureImpersonationSecret() {
	clusterList := &clusterv1alpha1.ClusterList{}
	if err := c.Client.List(context.TODO(), clusterList); err != nil {
		klog.Errorf("Failed to list clusterList, error: %v", err)
		return
	}

	for index, cluster := range clusterList.Items {
		if cluster.Spec.SyncMode == clusterv1alpha1.Pull {
			continue
		}
		err := c.ensureImpersonationSecretForCluster(&clusterList.Items[index])
		if err != nil {
			klog.Errorf("Failed to ensure impersonation secret exist for cluster %s", cluster.Name)
		}
	}
}

func (c *Controller) ensureImpersonationSecretForCluster(cluster *clusterv1alpha1.Cluster) error {
	controlPlaneKubeClient := kubeclient.NewForConfigOrDie(c.ControllerPlaneConfig)
	controlPlaneKarmadaClient := karmadaclientset.NewForConfigOrDie(c.ControllerPlaneConfig)

	klog.V(4).Infof("Create impersonation secret for cluster %s", cluster.Name)
	// create a ClusterClient for the given member cluster
	clusterClient, err := c.ClusterClientSetFunc(cluster.Name, c.Client, nil)
	if err != nil {
		klog.Errorf("Failed to create a ClusterClient for the given member cluster: %v, err is : %v", cluster.Name, err)
		return err
	}

	// clusterNamespace store namespace where serviceaccount and secret exist.
	clusterNamespace := cluster.Spec.SecretRef.Namespace

	// create a ServiceAccount for impersonation in cluster.
	impersonationSA := &corev1.ServiceAccount{}
	impersonationSA.Namespace = clusterNamespace
	impersonationSA.Name = names.GenerateServiceAccountName("impersonator")
	if impersonationSA, err = c.ensureServiceAccountExist(clusterClient.KubeClient, impersonationSA); err != nil {
		return err
	}

	clusterImpersonatorSecret, err := util.WaitForServiceAccountSecretCreation(clusterClient.KubeClient, impersonationSA)
	if err != nil {
		return fmt.Errorf("failed to get serviceAccount secret for impersonation from cluster(%s), error: %v", cluster.Name, err)
	}

	// create secret to store impersonation info in control plane
	impersonatorSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      names.GenerateImpersonationSecretName(cluster.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterv1alpha1.SchemeGroupVersion.WithKind("Cluster")),
			},
		},
		Data: map[string][]byte{
			clusterv1alpha1.SecretTokenKey: clusterImpersonatorSecret.Data[clusterv1alpha1.SecretTokenKey],
		},
	}

	_, err = util.CreateSecret(controlPlaneKubeClient, impersonatorSecret)
	if err != nil {
		return fmt.Errorf("failed to create impersonator secret in control plane. error: %v", err)
	}

	if cluster.Spec.ImpersonatorSecretRef == nil {
		mutateFunc := func(cluster *clusterv1alpha1.Cluster) {
			cluster.Spec.ImpersonatorSecretRef = &clusterv1alpha1.LocalSecretReference{
				Namespace: impersonatorSecret.Namespace,
				Name:      impersonatorSecret.Name,
			}
		}

		_, err = util.CreateOrUpdateClusterObject(controlPlaneKarmadaClient, cluster, mutateFunc)
		if err != nil {
			return err
		}
	}

	return nil
}

// ensureServiceAccountExist makes sure that the specific service account exist in cluster.
// If service account not exit, just create it.
func (c *Controller) ensureServiceAccountExist(client kubeclient.Interface, saObj *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	exist, err := util.IsServiceAccountExist(client, saObj.Namespace, saObj.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check if impersonation service account exist. error: %v", err)
	}
	if exist {
		return saObj, nil
	}

	createdObj, err := util.CreateServiceAccount(client, saObj)
	if err != nil {
		return nil, fmt.Errorf("ensure impersonation service account failed due to create failed, error: %v", err)
	}

	return createdObj, nil
}
