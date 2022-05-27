package search

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"

	searchapis "github.com/karmada-io/karmada/pkg/apis/search"
	searchinstall "github.com/karmada-io/karmada/pkg/apis/search/install"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	searchstorage "github.com/karmada-io/karmada/pkg/registry/search/storage"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
)

var (
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme = runtime.NewScheme()
	// Codecs provides methods for retrieving codecs and serializers for specific
	// versions and content types.
	Codecs = serializer.NewCodecFactory(Scheme)
	// ParameterCodec handles versioning of objects that are converted to query parameters.
	ParameterCodec = runtime.NewParameterCodec(Scheme)
)

func init() {
	searchinstall.Install(Scheme)

	// we need to add the options to empty v1
	// TODO fix the server code to avoid this
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})

	// TODO: keep the generic API server from wanting this
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
}

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

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(searchapis.GroupName, Scheme, ParameterCodec, Codecs)

	resourceRegistryStorage, err := searchstorage.NewResourceRegistryStorage(Scheme, c.GenericConfig.RESTOptionsGetter)
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
