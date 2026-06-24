/*
Copyright 2023 The Karmada Authors.

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

package util

import (
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/transport"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

type proxyHeaderRoundTripper struct {
	proxyHeaders http.Header
	roundTripper http.RoundTripper
}

var _ http.RoundTripper = &proxyHeaderRoundTripper{}

// NewProxyHeaderRoundTripperWrapperConstructor returns a RoundTripper wrapper that's usable within restConfig.WrapTransport.
func NewProxyHeaderRoundTripperWrapperConstructor(wt transport.WrapperFunc, headers map[string]string) transport.WrapperFunc {
	return func(rt http.RoundTripper) http.RoundTripper {
		if wt != nil {
			rt = wt(rt)
		}
		return &proxyHeaderRoundTripper{
			proxyHeaders: parseProxyHeaders(headers),
			roundTripper: rt,
		}
	}
}

// RoundTrip implements the http.RoundTripper interface.
func (r *proxyHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if tr, ok := r.roundTripper.(*http.Transport); ok {
		tr = tr.Clone()
		tr.ProxyConnectHeader = r.proxyHeaders
		return tr.RoundTrip(req)
	}
	return r.roundTripper.RoundTrip(req)
}

func parseProxyHeaders(headers map[string]string) http.Header {
	if len(headers) == 0 {
		return nil
	}

	proxyHeaders := make(http.Header)
	for key, values := range headers {
		proxyHeaders[textproto.CanonicalMIMEHeaderKey(key)] = strings.Split(values, ",")
	}
	return proxyHeaders
}

// tokenCacheTTL controls how often the token is re-read from the Secret.
var tokenCacheTTL = 30 * time.Second

// tokenErrorRetryInterval is a shorter TTL used after a failed Secret read,
// so recovery happens faster than a full tokenCacheTTL cycle.
var tokenErrorRetryInterval = 5 * time.Second

// tokenRefreshingRoundTripper re-reads the member cluster token from the Karmada
// Secret on a TTL so long-lived informer clients survive token rotation.
type tokenRefreshingRoundTripper struct {
	inner        http.RoundTripper
	secretGetter func(namespace, name string) (*corev1.Secret, error)
	secretNS     string
	secretName   string

	mu          sync.RWMutex
	cachedToken string
	cacheExpiry time.Time
}

var _ http.RoundTripper = &tokenRefreshingRoundTripper{}

// WrappedRoundTripper implements utilnet.RoundTripperWrapper.
func (t *tokenRefreshingRoundTripper) WrappedRoundTripper() http.RoundTripper { return t.inner }

// NewTokenRefreshingRoundTripperWrapperConstructor returns a WrapperFunc that injects a TTL-refreshed bearer token.
// secretGetter is expected to be informer-backed (in-memory read, not a live API call).
func NewTokenRefreshingRoundTripperWrapperConstructor(
	secretGetter func(string, string) (*corev1.Secret, error),
	secretNS, secretName, initialToken string,
) transport.WrapperFunc {
	return func(rt http.RoundTripper) http.RoundTripper {
		return &tokenRefreshingRoundTripper{
			inner:        rt,
			secretGetter: secretGetter,
			secretNS:     secretNS,
			secretName:   secretName,
			cachedToken:  initialToken,
			cacheExpiry:  time.Now().Add(tokenCacheTTL),
		}
	}
}

// RoundTrip implements the http.RoundTripper interface.
func (t *tokenRefreshingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.getToken())
	return t.inner.RoundTrip(req)
}

func (t *tokenRefreshingRoundTripper) getToken() string {
	t.mu.RLock()
	if time.Now().Before(t.cacheExpiry) {
		token := t.cachedToken
		t.mu.RUnlock()
		return token
	}
	t.mu.RUnlock()
	return t.forceRefresh()
}

func (t *tokenRefreshingRoundTripper) forceRefresh() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if time.Now().Before(t.cacheExpiry) {
		return t.cachedToken
	}

	secret, err := t.secretGetter(t.secretNS, t.secretName)
	if err != nil {
		klog.Warningf("tokenRefreshingRoundTripper: failed to refresh token from secret %s/%s, keeping last known token: %v",
			t.secretNS, t.secretName, err)
		t.cacheExpiry = time.Now().Add(tokenErrorRetryInterval)
		return t.cachedToken
	}

	if token := string(secret.Data[clusterv1alpha1.SecretTokenKey]); token != "" {
		t.cachedToken = token
	} else {
		klog.Warningf("tokenRefreshingRoundTripper: secret %s/%s has empty token key, keeping last known token",
			t.secretNS, t.secretName)
	}
	t.cacheExpiry = time.Now().Add(tokenCacheTTL)
	return t.cachedToken
}
