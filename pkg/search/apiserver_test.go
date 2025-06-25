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

package search

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/compatibility"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"

	searchstorage "github.com/karmada-io/karmada/pkg/registry/search/storage"
)

func TestNewKarmadaSearchAPIServer(t *testing.T) {
	tests := []struct {
		name                   string
		cfg                    *completedConfig
		genericAPIServerConfig *genericapiserver.Config
		client                 clientset.Interface
		restConfig             *restclient.Config
		prep                   func(*completedConfig, *genericapiserver.Config, clientset.Interface) error
		wantErr                bool
		errMsg                 string
	}{
		{
			name: "NewKarmadaSearchAPIServer_NetworkIssue_FailedToCreateRESTStorage",
			cfg: &completedConfig{
				ExtraConfig: &ExtraConfig{},
			},
			client: fakeclientset.NewSimpleClientset(),
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
			prep: func(cfg *completedConfig, genericAPIServerCfg *genericapiserver.Config, client clientset.Interface) error {
				sharedInformer := informers.NewSharedInformerFactory(client, 0)
				cfg.GenericConfig = genericAPIServerCfg.Complete(sharedInformer)
				resourceRegistryStorageBuilder = func(*runtime.Scheme, generic.RESTOptionsGetter) (*searchstorage.ResourceRegistryStorage, error) {
					return nil, errors.New("unexpected network issue while creating the resource registry storage")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected network issue while creating the resource registry storage",
		},
		{
			name: "NewKarmadaSearchAPIServer_InstalledAPIGroup_FailedToInstallAPIGroup",
			cfg: &completedConfig{
				ExtraConfig: &ExtraConfig{},
			},
			client: fakeclientset.NewSimpleClientset(),
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
			prep: func(cfg *completedConfig, genericAPIServerCfg *genericapiserver.Config, client clientset.Interface) error {
				sharedInformer := informers.NewSharedInformerFactory(client, 0)
				cfg.GenericConfig = genericAPIServerCfg.Complete(sharedInformer)
				resourceRegistryStorageBuilder = func(*runtime.Scheme, generic.RESTOptionsGetter) (*searchstorage.ResourceRegistryStorage, error) {
					return &searchstorage.ResourceRegistryStorage{}, nil
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
			name: "NewKarmadaSearchAPIServer_InstalledAPIGroup_APIGroupInstalled",
			cfg: &completedConfig{
				ExtraConfig: &ExtraConfig{},
			},
			client: fakeclientset.NewSimpleClientset(),
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
			prep: func(cfg *completedConfig, genericAPIServerCfg *genericapiserver.Config, client clientset.Interface) error {
				sharedInformer := informers.NewSharedInformerFactory(client, 0)
				cfg.GenericConfig = genericAPIServerCfg.Complete(sharedInformer)
				resourceRegistryStorageBuilder = func(*runtime.Scheme, generic.RESTOptionsGetter) (*searchstorage.ResourceRegistryStorage, error) {
					return &searchstorage.ResourceRegistryStorage{}, nil
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
				t.Fatalf("failed to prep test environment before creating new karmada search apiserver, got: %v", err)
			}
			_, err := test.cfg.New()
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
