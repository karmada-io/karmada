package proxy

import (
	"fmt"
	"net/http"
	"net/url"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	clusterlisters "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/search/proxy/store"
)

func newMultiClusterStore(clusterLister clusterlisters.ClusterLister,
	secretLister listcorev1.SecretLister, restMapper meta.RESTMapper) store.Store {
	clientFactory := &clientFactory{
		ClusterLister: clusterLister,
		SecretLister:  secretLister,
	}

	return store.NewMultiClusterCache(clientFactory.DynamicClientForCluster, restMapper)
}

type clientFactory struct {
	ClusterLister clusterlisters.ClusterLister
	SecretLister  listcorev1.SecretLister
}

// DynamicClientForCluster creates a dynamic client for required cluster.
// TODO: reuse with karmada/pkg/util/membercluster_client.go
func (factory *clientFactory) DynamicClientForCluster(clusterName string) (dynamic.Interface, error) {
	cluster, err := factory.ClusterLister.Get(clusterName)
	if err != nil {
		return nil, err
	}

	apiEndpoint := cluster.Spec.APIEndpoint
	if apiEndpoint == "" {
		return nil, fmt.Errorf("the api endpoint of cluster %s is empty", clusterName)
	}

	if cluster.Spec.SecretRef == nil {
		return nil, fmt.Errorf("cluster %s does not have a secret", clusterName)
	}

	secret, err := factory.SecretLister.Secrets(cluster.Spec.SecretRef.Namespace).Get(cluster.Spec.SecretRef.Name)
	if err != nil {
		return nil, err
	}

	token, tokenFound := secret.Data[clusterv1alpha1.SecretTokenKey]
	if !tokenFound || len(token) == 0 {
		return nil, fmt.Errorf("the secret for cluster %s is missing a non-empty value for %q", clusterName, clusterv1alpha1.SecretTokenKey)
	}

	clusterConfig, err := clientcmd.BuildConfigFromFlags(apiEndpoint, "")
	if err != nil {
		return nil, err
	}

	clusterConfig.BearerToken = string(token)

	if cluster.Spec.InsecureSkipTLSVerification {
		clusterConfig.TLSClientConfig.Insecure = true
	} else {
		clusterConfig.CAData = secret.Data[clusterv1alpha1.SecretCADataKey]
	}

	if cluster.Spec.ProxyURL != "" {
		proxy, err := url.Parse(cluster.Spec.ProxyURL)
		if err != nil {
			klog.Errorf("parse proxy error. %v", err)
			return nil, err
		}
		clusterConfig.Proxy = http.ProxyURL(proxy)
	}

	return dynamic.NewForConfig(clusterConfig)
}
