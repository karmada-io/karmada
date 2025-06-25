/*
Copyright 2024 The Karmada Authors.

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
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/compatibility"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"

	clusterscheme "github.com/karmada-io/karmada/pkg/apis/cluster/scheme"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
	clusterstorage "github.com/karmada-io/karmada/pkg/registry/cluster/storage"
)

func TestNewAggregatedAPIServer(t *testing.T) {
	tests := []struct {
		name                   string
		cfg                    *completedConfig
		genericAPIServerConfig *genericapiserver.Config
		restConfig             *restclient.Config
		secretLister           listcorev1.SecretLister
		client                 clientset.Interface
		prep                   func(*completedConfig, *genericapiserver.Config, clientset.Interface) error
		wantErr                bool
		errMsg                 string
	}{
		{
			name: "NewAggregatedAPIServer_NetworkIssue_FailedToCreateRESTStorage",
			cfg: &completedConfig{
				ExtraConfig: &ExtraConfig{},
			},
			genericAPIServerConfig: &genericapiserver.Config{
				RESTOptionsGetter: generic.RESTOptions{},
				Serializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{
					MediaType: runtime.ContentTypeJSON,
				}),
				LoopbackClientConfig:       &restclient.Config{},
				EquivalentResourceRegistry: runtime.NewEquivalentResourceRegistry(),
				BuildHandlerChainFunc: func(http.Handler, *genericapiserver.Config) (secure http.Handler) {
					return nil
				},
				ExternalAddress:  "10.0.0.0:10000",
				EffectiveVersion: compatibility.DefaultBuildEffectiveVersion(),
			},
			client: fakeclientset.NewSimpleClientset(),
			prep: func(cfg *completedConfig, genericAPIServerCfg *genericapiserver.Config, client clientset.Interface) error {
				sharedInformer := informers.NewSharedInformerFactory(client, 0)
				cfg.GenericConfig = genericAPIServerCfg.Complete(sharedInformer)
				newClusterStorageBuilder = func(*runtime.Scheme, *restclient.Config, listcorev1.SecretLister, generic.RESTOptionsGetter) (*clusterstorage.ClusterStorage, error) {
					return nil, errors.New("unexpected network issue while creating the cluster storage")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected network issue while creating the cluster storage",
		},
		{
			name: "NewAggregatedAPIServer_InstalledAPIGroup_FailedToInstallAPIGroup",
			cfg: &completedConfig{
				ExtraConfig: &ExtraConfig{},
			},
			genericAPIServerConfig: &genericapiserver.Config{
				RESTOptionsGetter: generic.RESTOptions{},
				Serializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{
					MediaType: runtime.ContentTypeJSON,
				}),
				LoopbackClientConfig:       &restclient.Config{},
				EquivalentResourceRegistry: runtime.NewEquivalentResourceRegistry(),
				BuildHandlerChainFunc: func(http.Handler, *genericapiserver.Config) (secure http.Handler) {
					return nil
				},
				OpenAPIV3Config:  genericapiserver.DefaultOpenAPIV3Config(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(clusterscheme.Scheme)),
				ExternalAddress:  "10.0.0.0:10000",
				EffectiveVersion: compatibility.DefaultBuildEffectiveVersion(),
			},
			prep: func(cfg *completedConfig, genericAPIServerCfg *genericapiserver.Config, client clientset.Interface) error {
				sharedInformer := informers.NewSharedInformerFactory(client, 0)
				cfg.GenericConfig = genericAPIServerCfg.Complete(sharedInformer)
				newClusterStorageBuilder = func(*runtime.Scheme, *restclient.Config, listcorev1.SecretLister, generic.RESTOptionsGetter) (*clusterstorage.ClusterStorage, error) {
					return &clusterstorage.ClusterStorage{}, nil
				}
				apiGroupInstaller = func(*APIServer, *genericapiserver.APIGroupInfo) error {
					return errors.New("failed to install api group")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "failed to install api group",
		},
		{
			name: "NewAggregatedAPIServer_InstalledAPIGroup_APIGroupInstalled",
			cfg: &completedConfig{
				ExtraConfig: &ExtraConfig{},
			},
			genericAPIServerConfig: &genericapiserver.Config{
				RESTOptionsGetter: generic.RESTOptions{},
				SecureServing: &genericapiserver.SecureServingInfo{
					Listener: &net.TCPListener{},
				},
				Serializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{
					MediaType: runtime.ContentTypeJSON,
				}),
				LoopbackClientConfig:       &restclient.Config{},
				EquivalentResourceRegistry: runtime.NewEquivalentResourceRegistry(),
				BuildHandlerChainFunc: func(http.Handler, *genericapiserver.Config) (secure http.Handler) {
					return nil
				},
				OpenAPIV3Config:  genericapiserver.DefaultOpenAPIV3Config(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(clusterscheme.Scheme)),
				ExternalAddress:  "10.0.0.0:10000",
				EffectiveVersion: compatibility.DefaultBuildEffectiveVersion(),
			},
			prep: func(cfg *completedConfig, genericAPIServerCfg *genericapiserver.Config, client clientset.Interface) error {
				sharedInformer := informers.NewSharedInformerFactory(client, 0)
				cfg.GenericConfig = genericAPIServerCfg.Complete(sharedInformer)
				newClusterStorageBuilder = func(*runtime.Scheme, *restclient.Config, listcorev1.SecretLister, generic.RESTOptionsGetter) (*clusterstorage.ClusterStorage, error) {
					return &clusterstorage.ClusterStorage{}, nil
				}
				apiGroupInstaller = func(*APIServer, *genericapiserver.APIGroupInfo) error {
					return nil
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.cfg, test.genericAPIServerConfig, test.client); err != nil {
				t.Fatalf("failed to prep test environment before creating new aggregated apiserver, got: %v", err)
			}
			_, err := test.cfg.New(test.restConfig, test.secretLister)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}
