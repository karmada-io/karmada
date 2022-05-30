package search

import (
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"

	searchapis "github.com/karmada-io/karmada/pkg/apis/search"
	searchscheme "github.com/karmada-io/karmada/pkg/apis/search/scheme"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	searchstorage "github.com/karmada-io/karmada/pkg/registry/search/storage"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
)

// ExtraConfig holds custom apiserver config
type ExtraConfig struct {
	MultiClusterInformerManager informermanager.MultiClusterInformerManager
	ClusterLister               clusterlister.ClusterLister

	// Add custom config if necessary.
}

// Config defines the config for the APIServer.
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	Controller    *Controller
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
		&ExtraConfig{
			MultiClusterInformerManager: cfg.Controller.InformerManager,
			ClusterLister:               cfg.Controller.clusterLister,
		},
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
	searchREST := searchstorage.NewSearchREST(c.ExtraConfig.MultiClusterInformerManager, c.ExtraConfig.ClusterLister)

	v1alpha1search := map[string]rest.Storage{}
	v1alpha1search["resourceregistries"] = resourceRegistryStorage.ResourceRegistry
	v1alpha1search["resourceregistries/status"] = resourceRegistryStorage.Status
	v1alpha1search["search"] = searchREST
	apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = v1alpha1search

	if err = server.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return server, nil
}
