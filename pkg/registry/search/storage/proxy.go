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

package storage

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"k8s.io/apimachinery/pkg/runtime"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	searchapis "github.com/karmada-io/karmada/pkg/apis/search"
	"github.com/karmada-io/karmada/pkg/search/proxy"
)

var proxyMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

// ProxyingREST implements a RESTStorage for proxying resource.
type ProxyingREST struct {
	ctl *proxy.Controller
}

var _ rest.Scoper = &ProxyingREST{}
var _ rest.Storage = &ProxyingREST{}
var _ rest.Connecter = &ProxyingREST{}
var _ rest.SingularNameProvider = &ProxyingREST{}

// NewProxyingREST returns a RESTStorage object that will work against search.
func NewProxyingREST(ctl *proxy.Controller) *ProxyingREST {
	return &ProxyingREST{
		ctl: ctl,
	}
}

// New return empty Proxy object.
func (r *ProxyingREST) New() runtime.Object {
	return &searchapis.Proxying{}
}

// NamespaceScoped returns false because Proxy is not namespaced.
func (r *ProxyingREST) NamespaceScoped() bool {
	return false
}

// ConnectMethods returns the list of HTTP methods handled by Connect.
func (r *ProxyingREST) ConnectMethods() []string {
	return proxyMethods
}

// NewConnectOptions returns an empty options object that will be used to pass options to the Connect method.
func (r *ProxyingREST) NewConnectOptions() (runtime.Object, bool, string) {
	return nil, true, ""
}

// Connect returns a handler for proxy.
func (r *ProxyingREST) Connect(ctx context.Context, _ string, _ runtime.Object, responder rest.Responder) (http.Handler, error) {
	info, ok := genericrequest.RequestInfoFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("no RequestInfo found in the context")
	}

	if len(info.Parts) < 2 {
		return nil, fmt.Errorf("invalid requestInfo parts: %v", info.Parts)
	}

	// For example, the whole request URL is /apis/search.karmada.io/v1alpha1/proxying/foo/proxy/api/v1/nodes
	// info.Parts is [proxying foo proxy api v1 nodes], so proxyPath is /api/v1/nodes
	proxyPath := "/" + path.Join(info.Parts[3:]...)
	klog.V(4).Infof("ProxyingREST connect %v", proxyPath)
	return r.ctl.Connect(ctx, proxyPath, responder)
}

// Destroy cleans up its resources on shutdown.
func (r *ProxyingREST) Destroy() {
	// Given no underlying store, so we don't
	// need to destroy anything.
}

// GetSingularName returns singular name of resources
func (r *ProxyingREST) GetSingularName() string {
	return "proxying"
}
