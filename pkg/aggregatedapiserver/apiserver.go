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

package aggregatedapiserver

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/compatibility"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"
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
		GenericConfig: cfg.GenericConfig.Complete(),
		ExtraConfig:   &cfg.ExtraConfig,
	}
	c.GenericConfig.EffectiveVersion = compatibility.DefaultBuildEffectiveVersion()

	return CompletedConfig{&c}
}

var newClusterStorageBuilder = func(scheme *runtime.Scheme, restConfig *restclient.Config, secretLister listcorev1.SecretLister, optsGetter generic.RESTOptionsGetter) (*clusterstorage.ClusterStorage, error) {
	return clusterstorage.NewStorage(scheme, restConfig, secretLister, optsGetter)
}
var apiGroupInstaller = func(server *APIServer, apiGroupInfo *genericapiserver.APIGroupInfo) error {
	return server.GenericAPIServer.InstallAPIGroup(apiGroupInfo)
}

func (c completedConfig) New(restConfig *restclient.Config, secretLister listcorev1.SecretLister) (*APIServer, error) {
	genericServer, err := c.GenericConfig.New("aggregated-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	server := &APIServer{
		GenericAPIServer: genericServer,
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(clusterapis.GroupName, clusterscheme.Scheme, clusterscheme.ParameterCodec, clusterscheme.Codecs)

	clusterStorage, err := newClusterStorageBuilder(clusterscheme.Scheme, restConfig, secretLister, c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		klog.Errorf("Unable to create REST storage for a resource due to %v, will die", err)
		return nil, err
	}
	v1alpha1cluster := map[string]rest.Storage{}
	v1alpha1cluster["clusters"] = clusterStorage.Cluster
	v1alpha1cluster["clusters/status"] = clusterStorage.Status
	v1alpha1cluster["clusters/proxy"] = clusterStorage.Proxy
	apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = v1alpha1cluster

	if err = apiGroupInstaller(server, &apiGroupInfo); err != nil {
		return nil, err
	}

	return server, nil
}
