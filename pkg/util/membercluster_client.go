package util

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

const (
	// kubeAPIQPS is the maximum QPS to the master from this client
	kubeAPIQPS = 20.0
	// kubeAPIBurst is the maximum burst for throttle
	kubeAPIBurst = 30
	tokenKey     = "token"
	cADataKey    = "caBundle"
)

// ClusterClient stands for a cluster Clientset for the given member cluster
type ClusterClient struct {
	KubeClient  *kubeclientset.Clientset
	ClusterName string
}

// DynamicClusterClient stands for a dynamic client for the given member cluster
type DynamicClusterClient struct {
	DynamicClientSet dynamic.Interface
	ClusterName      string
}

// NewClusterClientSet returns a ClusterClient for the given member cluster.
func NewClusterClientSet(c *v1alpha1.Cluster, client client.Client) (*ClusterClient, error) {
	clusterConfig, err := buildClusterConfig(c, client)
	if err != nil {
		return nil, err
	}
	var clusterClientSet = ClusterClient{ClusterName: c.Name}

	if clusterConfig != nil {
		clusterClientSet.KubeClient = kubeclientset.NewForConfigOrDie(clusterConfig)
	}
	return &clusterClientSet, nil
}

// NewClusterClientSetForAgent returns a ClusterClient for the given member cluster which will be used in karmada agent.
func NewClusterClientSetForAgent(c *v1alpha1.Cluster, client client.Client) (*ClusterClient, error) {
	clusterConfig, err := controllerruntime.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig of member cluster: %s", err.Error())
	}
	var clusterClientSet = ClusterClient{ClusterName: c.Name}

	if clusterConfig != nil {
		clusterClientSet.KubeClient = kubeclientset.NewForConfigOrDie(clusterConfig)
	}
	return &clusterClientSet, nil
}

// NewClusterDynamicClientSet returns a dynamic client for the given member cluster.
func NewClusterDynamicClientSet(c *v1alpha1.Cluster, client client.Client) (*DynamicClusterClient, error) {
	clusterConfig, err := buildClusterConfig(c, client)
	if err != nil {
		return nil, err
	}
	var clusterClientSet = DynamicClusterClient{ClusterName: c.Name}

	if clusterConfig != nil {
		clusterClientSet.DynamicClientSet = dynamic.NewForConfigOrDie(clusterConfig)
	}
	return &clusterClientSet, nil
}

// NewClusterDynamicClientSetForAgent returns a dynamic client for the given member cluster which will be used in karmada agent.
func NewClusterDynamicClientSetForAgent(c *v1alpha1.Cluster, client client.Client) (*DynamicClusterClient, error) {
	clusterConfig, err := controllerruntime.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig of member cluster: %s", err.Error())
	}
	var clusterClientSet = DynamicClusterClient{ClusterName: c.Name}

	if clusterConfig != nil {
		clusterClientSet.DynamicClientSet = dynamic.NewForConfigOrDie(clusterConfig)
	}
	return &clusterClientSet, nil
}

func buildClusterConfig(cluster *v1alpha1.Cluster, client client.Client) (*rest.Config, error) {
	clusterName := cluster.Name
	apiEndpoint := cluster.Spec.APIEndpoint
	if apiEndpoint == "" {
		return nil, fmt.Errorf("the api endpoint of cluster %s is empty", clusterName)
	}

	secretNamespace := cluster.Spec.SecretRef.Namespace
	secretName := cluster.Spec.SecretRef.Name
	if secretName == "" {
		return nil, fmt.Errorf("cluster %s does not have a secret name", clusterName)
	}

	secret := &v1.Secret{}
	if err := client.Get(context.TODO(), types.NamespacedName{Namespace: secretNamespace, Name: secretName}, secret); err != nil {
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

	clusterConfig.BearerToken = string(token)
	clusterConfig.QPS = kubeAPIQPS
	clusterConfig.Burst = kubeAPIBurst

	if cluster.Spec.InsecureSkipTLSVerification {
		clusterConfig.TLSClientConfig.Insecure = true
	} else {
		clusterConfig.CAData = secret.Data[cADataKey]
	}

	if cluster.Spec.ProxyURL != "" {
		proxy, err := url.Parse(cluster.Spec.ProxyURL)
		if err != nil {
			klog.Errorf("parse proxy error. %v", err)
			return nil, err
		}
		clusterConfig.Proxy = http.ProxyURL(proxy)
	}

	return clusterConfig, nil
}
