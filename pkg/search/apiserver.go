/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package search

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/compatibility"
	"k8s.io/klog/v2"

	searchapis "github.com/karmada-io/karmada/pkg/apis/search"
	searchscheme "github.com/karmada-io/karmada/pkg/apis/search/scheme"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	searchstorage "github.com/karmada-io/karmada/pkg/registry/search/storage"
	"github.com/karmada-io/karmada/pkg/search/proxy"
	"github.com/karmada-io/karmada/pkg/util/names"
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
		GenericConfig: cfg.GenericConfig.Complete(),
		ExtraConfig:   &cfg.ExtraConfig,
	}
	c.GenericConfig.EffectiveVersion = compatibility.DefaultBuildEffectiveVersion()

	return CompletedConfig{&c}
}

var resourceRegistryStorageBuilder = func(scheme *runtime.Scheme, optsGetter generic.RESTOptionsGetter) (*searchstorage.ResourceRegistryStorage, error) {
	return searchstorage.NewResourceRegistryStorage(scheme, optsGetter)
}
var apiGroupInstaller = func(server *APIServer, apiGroupInfo *genericapiserver.APIGroupInfo) error {
	return server.GenericAPIServer.InstallAPIGroup(apiGroupInfo)
}

func (c completedConfig) New() (*APIServer, error) {
	genericServer, err := c.GenericConfig.New(names.KarmadaSearchComponentName, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	server := &APIServer{
		GenericAPIServer: genericServer,
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(searchapis.GroupName, searchscheme.Scheme, searchscheme.ParameterCodec, searchscheme.Codecs)

	resourceRegistryStorage, err := resourceRegistryStorageBuilder(searchscheme.Scheme, c.GenericConfig.RESTOptionsGetter)
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

	if err = apiGroupInstaller(server, &apiGroupInfo); err != nil {
		return nil, err
	}

	return server, nil
}
