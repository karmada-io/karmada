package membercluster

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/huawei-cloudnative/karmada/pkg/apis/membercluster/v1alpha1"
)

const (
	// kubeAPIQPS is the maximum QPS to the master from this client
	kubeAPIQPS = 20.0
	// kubeAPIBurst is the Maximum burst for throttle
	kubeAPIBurst = 30
	tokenKey     = "token"
	cADataKey    = "caBundle"
)

// ClusterClient stands for a ClusterClient for the given member cluster
type ClusterClient struct {
	KubeClient  *kubeclientset.Clientset
	clusterName string
}

// NewClusterClientSet returns a ClusterClient for the given member cluster.
func NewClusterClientSet(c *v1alpha1.MemberCluster, client kubeclientset.Interface, namespace string) (*ClusterClient, error) {
	clusterConfig, err := buildMemberClusterConfig(c, client, namespace)
	if err != nil {
		return nil, err
	}
	var clusterClientSet = ClusterClient{clusterName: c.Name}

	if clusterConfig != nil {
		clusterClientSet.KubeClient = kubeclientset.NewForConfigOrDie(clusterConfig)
	}
	return &clusterClientSet, nil
}

func buildMemberClusterConfig(memberCluster *v1alpha1.MemberCluster, client kubeclientset.Interface, namespace string) (*rest.Config, error) {
	clusterName := memberCluster.Name
	apiEndpoint := memberCluster.Spec.APIEndpoint
	if apiEndpoint == "" {
		return nil, fmt.Errorf("the api endpoint of cluster %s is empty", clusterName)
	}

	secretName := memberCluster.Spec.SecretRef.Name
	if secretName == "" {
		return nil, fmt.Errorf("cluster %s does not have a secret name", clusterName)
	}

	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	token, tokenFound := secret.Data[tokenKey]
	if !tokenFound || len(token) == 0 {
		return nil, fmt.Errorf("the secret for cluster %s is missing a non-empty value for %q", clusterName, tokenKey)
	}

	clusterConfig, err := clientcmd.BuildConfigFromFlags(apiEndpoint, "")
	if err != nil {
		return nil, err
	}
	clusterConfig.CAData = secret.Data[cADataKey]
	clusterConfig.BearerToken = string(token)
	clusterConfig.QPS = kubeAPIQPS
	clusterConfig.Burst = kubeAPIBurst

	return clusterConfig, nil
}
