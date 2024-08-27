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

package storage

import (
	"context"
	_ "net/http"
	_ "net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	_ "k8s.io/apiserver/pkg/registry/rest"

	searchapis "github.com/karmada-io/karmada/pkg/apis/search"
	"github.com/karmada-io/karmada/pkg/search/proxy"
)

func TestNewProxyingREST(t *testing.T) {
	ctl := &proxy.Controller{}
	r := NewProxyingREST(ctl)
	assert.NotNil(t, r)
	assert.Equal(t, ctl, r.ctl)
}

func TestProxyingREST_New(t *testing.T) {
	ctl := &proxy.Controller{}
	r := NewProxyingREST(ctl)
	obj := r.New()
	assert.NotNil(t, obj)
	_, ok := obj.(*searchapis.Proxying)
	assert.True(t, ok)
}

func TestProxyingREST_NamespaceScoped(t *testing.T) {
	ctl := &proxy.Controller{}
	r := NewProxyingREST(ctl)
	assert.False(t, r.NamespaceScoped())
}

func TestProxyingREST_ConnectMethods(t *testing.T) {
	ctl := &proxy.Controller{}
	r := NewProxyingREST(ctl)
	methods := r.ConnectMethods()
	assert.Equal(t, proxyMethods, methods)
}

func TestProxyingREST_NewConnectOptions(t *testing.T) {
	ctl := &proxy.Controller{}
	r := NewProxyingREST(ctl)
	obj, ok, s := r.NewConnectOptions()
	assert.Nil(t, obj)
	assert.True(t, ok)
	assert.Equal(t, "", s)
}

func TestProxyingREST_Connect(t *testing.T) {
	ctl := &proxy.Controller{}
	r := NewProxyingREST(ctl)

	t.Run("Test missing RequestInfo in context", func(t *testing.T) {
		ctx := context.Background()
		_, err := r.Connect(ctx, "", nil, nil)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "no RequestInfo found in the context")
	})

	t.Run("Test invalid RequestInfo parts", func(t *testing.T) {
		ctx := genericrequest.WithRequestInfo(context.Background(), &genericrequest.RequestInfo{
			Parts: []string{"proxying"},
		})
		_, err := r.Connect(ctx, "", nil, nil)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "invalid requestInfo parts: [proxying]")
	})
}

func TestProxyingREST_Destroy(_ *testing.T) {
	ctl := &proxy.Controller{}
	r := NewProxyingREST(ctl)
	r.Destroy()
}

func TestProxyingREST_GetSingularName(t *testing.T) {
	ctl := &proxy.Controller{}
	r := NewProxyingREST(ctl)
	name := r.GetSingularName()
	assert.Equal(t, "proxying", name)
}
