package util

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

var _ genericclioptions.RESTClientGetter = &ClusterRESTClientGetter{}

// ClusterRESTClientGetter implements a cluster RESTClientGetter for the given member cluster.
type ClusterRESTClientGetter struct {
	clusterConfig rest.Config
	namespace     string
}

// NewClusterRESTClientGetter returns a ClusterRESTClientGetter for the given member cluster.
func NewClusterRESTClientGetter(config rest.Config, namespace string) ClusterRESTClientGetter {
	return ClusterRESTClientGetter{
		clusterConfig: config,
		namespace:     namespace,
	}
}

// ToDiscoveryClient implements the interface of RESTClientGetter.
func (c *ClusterRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	discoveryClient, _ := discovery.NewDiscoveryClientForConfig(&c.clusterConfig)
	return memory.NewMemCacheClient(discoveryClient), nil
}

// ToRESTMapper implements the interface of RESTClientGetter.
func (c *ClusterRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := c.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

// ToRawKubeConfigLoader implements the interface of RESTClientGetter.
func (c *ClusterRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}
	overrides.Context.Namespace = c.namespace

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}

// ToRESTConfig implements the interface of RESTClientGetter.
func (c *ClusterRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return &c.clusterConfig, nil
}
