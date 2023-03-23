package search

import (
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"

	searchapis "github.com/karmada-io/karmada/pkg/apis/search"
	searchscheme "github.com/karmada-io/karmada/pkg/apis/search/scheme"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	searchstorage "github.com/karmada-io/karmada/pkg/registry/search/storage"
	"github.com/karmada-io/karmada/pkg/search/proxy"
)

// ExtraConfig holds custom apiserver config
type ExtraConfig struct {
	KarmadaSharedInformerFactory informerfactory.SharedInformerFactory
	Controller                   *Controller
	ProxyController              *proxy.Controller
	// Add custom config if necessary.
}

// Config defines the config for the APIServer.
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// APIServer contains state for karmada-search.
type APIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

// CompletedConfig embeds a private pointer that cannot be instantiated outside this package.
type CompletedConfig struct {
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (cfg *Config) Complete() CompletedConfig {
	c := completedConfig{
		cfg.GenericConfig.Complete(),
		&cfg.ExtraConfig,
	}

	c.GenericConfig.Version = &version.Info{
		Major: "1",
		Minor: "0",
	}

	return CompletedConfig{&c}
}

func (c completedConfig) New() (*APIServer, error) {
	genericServer, err := c.GenericConfig.New("karmada-search", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	server := &APIServer{
		GenericAPIServer: genericServer,
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(searchapis.GroupName, searchscheme.Scheme, searchscheme.ParameterCodec, searchscheme.Codecs)

	resourceRegistryStorage, err := searchstorage.NewResourceRegistryStorage(searchscheme.Scheme, c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		klog.Errorf("unable to create REST storage for a resource due to %v, will die", err)
		return nil, err
	}

	v1alpha1search := map[string]rest.Storage{}
	v1alpha1search["resourceregistries"] = resourceRegistryStorage.ResourceRegistry
	v1alpha1search["resourceregistries/status"] = resourceRegistryStorage.Status

	if c.ExtraConfig.Controller != nil {
		searchREST := searchstorage.NewSearchREST(
			c.ExtraConfig.Controller.InformerManager,
			c.ExtraConfig.Controller.clusterLister,
			c.ExtraConfig.Controller.restMapper)
		v1alpha1search["search"] = searchREST
	}

	if c.ExtraConfig.ProxyController != nil {
		proxyingREST := searchstorage.NewProxyingREST(c.ExtraConfig.ProxyController)
		v1alpha1search["proxying"] = proxyingREST
	}

	apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = v1alpha1search

	if err = server.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return server, nil
}
