/*
Copyright 2022 The Karmada Authors.

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

package karmada

import (
	"context"
	"net/http"
	"net/url"
	"path"

	restclient "k8s.io/client-go/rest"

	"github.com/karmada-io/karmada/pkg/search/proxy/framework"
	pluginruntime "github.com/karmada-io/karmada/pkg/search/proxy/framework/runtime"
	"github.com/karmada-io/karmada/pkg/util/proxy"
)

const (
	// We keep a big gap between in-tree plugins, to allow users to insert custom plugins between them.
	order = 3000
)

// Karmada proxies requests to karmada control panel.
// For non-resource requests, or resources are not defined in ResourceRegistry,
// we redirect the requests to karmada apiserver.
// Usually the request are
// - api index, e.g.: `/api`, `/apis`
// - to workload created in karmada controller panel, such as deployments and services.
type Karmada struct {
	proxyLocation  *url.URL
	proxyTransport http.RoundTripper
}

var _ framework.Plugin = (*Karmada)(nil)

// New creates an instance of Karmada
func New(dep pluginruntime.PluginDependency) (framework.Plugin, error) {
	location, err := url.Parse(dep.RestConfig.Host)
	if err != nil {
		return nil, err
	}

	transport, err := restclient.TransportFor(dep.RestConfig)
	if err != nil {
		return nil, err
	}

	return &Karmada{
		proxyLocation:  location,
		proxyTransport: transport,
	}, nil
}

// Order implements Plugin
func (p *Karmada) Order() int {
	return order
}

// SupportRequest implements Plugin
func (p *Karmada) SupportRequest(_ framework.ProxyRequest) bool {
	// This plugin's order is the last one. It's actually a fallback plugin.
	// So we return true here to indicate we always support the request.
	return true
}

// Connect implements Plugin
func (p *Karmada) Connect(_ context.Context, request framework.ProxyRequest) (http.Handler, error) {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		location, transport := p.resourceLocation()
		location.Path = path.Join(location.Path, request.ProxyPath)
		location.RawQuery = req.URL.RawQuery

		handler := proxy.NewThrottledUpgradeAwareProxyHandler(
			location, transport, true, false, request.Responder)
		handler.ServeHTTP(rw, req)
	}), nil
}

func (p *Karmada) resourceLocation() (*url.URL, http.RoundTripper) {
	location := *p.proxyLocation
	return &location, p.proxyTransport
}
