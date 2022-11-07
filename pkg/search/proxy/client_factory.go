package proxy

import (
	"fmt"

	clusterlisters "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/search/proxy/store"
	"github.com/karmada-io/karmada/pkg/util"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	listcorev1 "k8s.io/client-go/listers/core/v1"
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
	clusterConfig, err := util.BuildConfigWithSecret(secret, cluster, apiEndpoint)
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(clusterConfig)
}
