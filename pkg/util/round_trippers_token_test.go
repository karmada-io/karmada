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

package util

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

type recordingRoundTripper struct {
	mu      sync.Mutex
	lastReq *http.Request
}

func (r *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.lastReq = req
	r.mu.Unlock()
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (r *recordingRoundTripper) authHeader() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastReq.Header.Get("Authorization")
}

func tokenSecret(token string) *corev1.Secret {
	return &corev1.Secret{Data: map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte(token)}}
}

func newTokenRT(rec http.RoundTripper, getter func(string, string) (*corev1.Secret, error), initialToken string) *tokenRefreshingRoundTripper {
	return NewTokenRefreshingRoundTripperWrapperConstructor(getter, "ns", "name", initialToken)(rec).(*tokenRefreshingRoundTripper)
}

func doGet(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://example.test/api", nil)
	assert.NoError(t, err)
	_, err = rt.RoundTrip(req)
	assert.NoError(t, err)
}

func TestTokenRefreshingRoundTripper_InjectsToken(t *testing.T) {
	rec := &recordingRoundTripper{}
	getter := func(string, string) (*corev1.Secret, error) { return tokenSecret("token-A"), nil }
	rt := newTokenRT(rec, getter, "token-A")

	req, err := http.NewRequest(http.MethodGet, "https://example.test/api", nil)
	assert.NoError(t, err)
	_, err = rt.RoundTrip(req)
	assert.NoError(t, err)

	assert.Equal(t, "Bearer token-A", rec.authHeader(), "token must be injected into the request sent downstream")
	assert.Empty(t, req.Header.Get("Authorization"), "the caller's original request must not be mutated")
}

func TestTokenRefreshingRoundTripper_RefreshesAfterTTL(t *testing.T) {
	rec := &recordingRoundTripper{}
	var mu sync.Mutex
	current := "token-A"
	getter := func(string, string) (*corev1.Secret, error) {
		mu.Lock()
		defer mu.Unlock()
		return tokenSecret(current), nil
	}
	rt := newTokenRT(rec, getter, "token-A")

	mu.Lock()
	current = "token-B"
	mu.Unlock()
	rt.cacheExpiry = time.Now().Add(-time.Minute)

	doGet(t, rt)
	assert.Equal(t, "Bearer token-B", rec.authHeader(), "rotated token must be picked up after the TTL")
}

func TestTokenRefreshingRoundTripper_KeepsTokenOnSecretError(t *testing.T) {
	rec := &recordingRoundTripper{}
	getter := func(string, string) (*corev1.Secret, error) { return nil, errors.New("temporarily unavailable") }
	rt := newTokenRT(rec, getter, "token-A")
	rt.cacheExpiry = time.Now().Add(-time.Minute)

	doGet(t, rt)
	assert.Equal(t, "Bearer token-A", rec.authHeader(), "on Secret read error the last good token must be retained")
}

func TestTokenRefreshingRoundTripper_IgnoresEmptyToken(t *testing.T) {
	rec := &recordingRoundTripper{}
	getter := func(string, string) (*corev1.Secret, error) { return tokenSecret(""), nil }
	rt := newTokenRT(rec, getter, "token-A")
	rt.cacheExpiry = time.Now().Add(-time.Minute)

	doGet(t, rt)
	assert.Equal(t, "Bearer token-A", rec.authHeader(), "an empty token from the Secret must not overwrite a good cached token")
}

func TestTokenRefreshingRoundTripper_ChainsWithProxyHeaders(t *testing.T) {
	var capturedReq *http.Request
	inner := &http.Transport{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	getter := func(string, string) (*corev1.Secret, error) { return tokenSecret("my-token"), nil }
	proxyWrapper := NewProxyHeaderRoundTripperWrapperConstructor(nil, map[string]string{"Proxy-Authorization": "Basic secret"})
	tokenWrapper := NewTokenRefreshingRoundTripperWrapperConstructor(getter, "ns", "name", "my-token")
	chain := tokenWrapper(proxyWrapper(inner))

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	assert.NoError(t, err)
	_, err = chain.RoundTrip(req)
	assert.NoError(t, err)

	assert.Equal(t, "Bearer my-token", capturedReq.Header.Get("Authorization"),
		"token wrapper must set the Authorization header")
}

func TestTokenRefreshingRoundTripper_ConcurrentRefreshDeduplicated(t *testing.T) {
	rec := &recordingRoundTripper{}
	var calls atomic.Int32
	getter := func(string, string) (*corev1.Secret, error) {
		calls.Add(1)
		return tokenSecret("token-B"), nil
	}
	rt := newTokenRT(rec, getter, "token-A")
	rt.cacheExpiry = time.Now().Add(-time.Minute)

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			doGet(t, rt)
		})
	}
	wg.Wait()

	assert.Equal(t, int32(1), calls.Load(), "the Secret must be read exactly once per TTL window despite concurrent requests")
}
