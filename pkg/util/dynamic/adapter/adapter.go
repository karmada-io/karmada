/*
Copyright 2026 The Karmada Authors.

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

package adapter

import (
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/features"
	utildynamic "github.com/karmada-io/karmada/pkg/util/dynamic"
	utildynamicinformer "github.com/karmada-io/karmada/pkg/util/dynamic/dynamicinformer"
)

// InformerMode controls which dynamic informer implementation should be used.
type InformerMode string

const (
	// InformerModeUnstructured uses client-go's unstructured dynamic informer implementation.
	InformerModeUnstructured InformerMode = "unstructured"
	// InformerModeRawDynamic uses Karmada's raw dynamic client and informer implementation.
	InformerModeRawDynamic InformerMode = "rawdynamic"
)

// DynamicInformerEnabled returns whether the Karmada dynamic informer path is enabled.
//
//nolint:revive
func DynamicInformerEnabled() bool {
	return features.FeatureGate.Enabled(features.RawDynamicInformer)
}

func informerMode() InformerMode {
	if DynamicInformerEnabled() {
		return InformerModeRawDynamic
	}
	return InformerModeUnstructured
}

type informerInterface interface {
	Interface
	InformerMode() InformerMode
	DynamicClient() utildynamic.Interface
}

// DynamicClient delegates normal dynamic client calls to client-go's dynamic client.
// The embedded Karmada dynamic client is used only when the selected informer
// mode is InformerModeRawDynamic.
//
//nolint:revive
type DynamicClient struct {
	Interface
	dynamicClient utildynamic.Interface
	informerMode  InformerMode
}

var _ Interface = &DynamicClient{}
var _ informerInterface = &DynamicClient{}

// ConfigFor returns a copy of the provided config with client-go dynamic
// client defaults set.
func ConfigFor(inConfig *rest.Config) *rest.Config {
	return dynamic.ConfigFor(inConfig)
}

// New creates a new DynamicClient for the given RESTClient.
func New(c rest.Interface) *DynamicClient {
	mode := informerMode()
	var dynamicClient utildynamic.Interface
	if mode == InformerModeRawDynamic {
		dynamicClient = utildynamic.New(c)
	}

	return &DynamicClient{
		Interface:     dynamic.New(c),
		dynamicClient: dynamicClient,
		informerMode:  mode,
	}
}

// NewForClients creates a delegating dynamic client from explicit client-go
// and Karmada dynamic clients.
func NewForClients(client Interface, dynamicClient utildynamic.Interface) (*DynamicClient, error) {
	return newForClients(client, dynamicClient, informerMode())
}

// MustNewForClients returns a delegating dynamic client and panics on invalid input.
func MustNewForClients(client Interface, dynamicClient utildynamic.Interface) *DynamicClient {
	delegatingClient, err := NewForClients(client, dynamicClient)
	if err != nil {
		panic(err)
	}
	return delegatingClient
}

func newForClients(client Interface, dynamicClient utildynamic.Interface, mode InformerMode) (*DynamicClient, error) {
	if client == nil {
		return nil, fmt.Errorf("dynamic client must not be nil")
	}
	if mode == InformerModeRawDynamic && dynamicClient == nil {
		return nil, fmt.Errorf("karmada dynamic client must not be nil when informer mode is %q", InformerModeRawDynamic)
	}

	return &DynamicClient{
		Interface:     client,
		dynamicClient: dynamicClient,
		informerMode:  mode,
	}, nil
}

// NewForConfigOrDie creates a delegating dynamic client and panics on errors.
func NewForConfigOrDie(config *rest.Config) *DynamicClient {
	client, err := NewForConfig(config)
	if err != nil {
		panic(err)
	}
	return client
}

// NewForConfig creates a delegating dynamic client from a REST config.
// NewForConfig is equivalent to NewForConfigAndClient(c, httpClient),
// where httpClient was generated with rest.HTTPClientFor(c).
func NewForConfig(inConfig *rest.Config) (*DynamicClient, error) {
	config := ConfigFor(inConfig)

	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, err
	}

	return NewForConfigAndClient(config, httpClient)
}

// NewForConfigAndClient creates a delegating dynamic client for the given
// config and http client.
func NewForConfigAndClient(inConfig *rest.Config, h *http.Client) (*DynamicClient, error) {
	mode := informerMode()
	client, err := dynamic.NewForConfigAndClient(inConfig, h)
	if err != nil {
		return nil, err
	}

	var dynamicClient utildynamic.Interface
	if mode == InformerModeRawDynamic {
		dynamicClient, err = utildynamic.NewForConfigAndClient(inConfig, h)
		if err != nil {
			return nil, err
		}
	}

	return newForClients(client, dynamicClient, mode)
}

// InformerMode returns the selected informer mode.
func (c *DynamicClient) InformerMode() InformerMode {
	return c.informerMode
}

// DynamicClient returns the Karmada dynamic client used by the new informer path.
func (c *DynamicClient) DynamicClient() utildynamic.Interface {
	return c.dynamicClient
}

// NewDynamicSharedInformerFactory creates a dynamic informer factory that follows
// the informer mode selected by the provided client.
func NewDynamicSharedInformerFactory(client Interface, defaultResync time.Duration) dynamicinformer.DynamicSharedInformerFactory {
	if delegatingClient, ok := client.(informerInterface); ok && delegatingClient.InformerMode() == InformerModeRawDynamic {
		if dynamicClient := delegatingClient.DynamicClient(); dynamicClient != nil {
			klog.Infof("using raw dynamic client")
			return utildynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, defaultResync)
		}
	}
	klog.Info("using native dynamic client")
	return dynamicinformer.NewDynamicSharedInformerFactory(client, defaultResync)
}
