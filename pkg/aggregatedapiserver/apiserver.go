package aggregatedapiserver

import (
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	clusterscheme "github.com/karmada-io/karmada/pkg/apis/cluster/scheme"
	clusterstorage "github.com/karmada-io/karmada/pkg/registry/cluster/storage"
)

// ExtraConfig holds custom apiserver config
type ExtraConfig struct {
	// Add custom config if necessary.
}

// Config defines the config for the apiserver
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// APIServer contains state for karmada aggregated-apiserver.
type APIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

// CompletedConfig embeds a private pointer that cannot be instantiated outside of this package.
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

func (c completedConfig) New(kubeClient kubernetes.Interface) (*APIServer, error) {
	genericServer, err := c.GenericConfig.New("aggregated-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	server := &APIServer{
		GenericAPIServer: genericServer,
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(clusterapis.GroupName, clusterscheme.Scheme, clusterscheme.ParameterCodec, clusterscheme.Codecs)

	clusterStorage, err := clusterstorage.NewStorage(clusterscheme.Scheme, kubeClient, c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		klog.Errorf("Unable to create REST storage for a resource due to %v, will die", err)
		return nil, err
	}
	v1alpha1cluster := map[string]rest.Storage{}
	v1alpha1cluster["clusters"] = clusterStorage.Cluster
	v1alpha1cluster["clusters/status"] = clusterStorage.Status
	v1alpha1cluster["clusters/proxy"] = clusterStorage.Proxy
	apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = v1alpha1cluster

	if err = server.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return server, nil
}
