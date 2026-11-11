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

package util

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/transport"
)

func TestNewProxyHeaderRoundTripperWrapperConstructor(t *testing.T) {
	tests := []struct {
		name           string
		wrapperFunc    transport.WrapperFunc
		headers        map[string]string
		expectedEmpty  bool
		expectedCount  int
		expectedHeader string
		expectedValues []string
	}{
		{
			name:          "nil wrapper with empty headers",
			wrapperFunc:   nil,
			headers:       nil,
			expectedEmpty: true,
		},
		{
			name:        "nil wrapper with single header",
			wrapperFunc: nil,
			headers: map[string]string{
				"Proxy-Authorization": "Basic xyz",
			},
			expectedCount:  1,
			expectedHeader: "Proxy-Authorization",
			expectedValues: []string{"Basic xyz"},
		},
		{
			name:        "nil wrapper with multiple comma-separated values",
			wrapperFunc: nil,
			headers: map[string]string{
				"X-Custom-Header": "value1,value2,value3",
			},
			expectedCount:  1,
			expectedHeader: "X-Custom-Header",
			expectedValues: []string{"value1", "value2", "value3"},
		},
		{
			name: "with wrapper func",
			wrapperFunc: func(rt http.RoundTripper) http.RoundTripper {
				return rt
			},
			headers: map[string]string{
				"Proxy-Authorization": "Basic abc",
			},
			expectedCount:  1,
			expectedHeader: "Proxy-Authorization",
			expectedValues: []string{"Basic abc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapper := NewProxyHeaderRoundTripperWrapperConstructor(tt.wrapperFunc, tt.headers)
			assert.NotNil(t, wrapper, "wrapper should not be nil")

			mockRT := &mockRoundTripper{}
			rt := wrapper(mockRT)
			phrt, ok := rt.(*proxyHeaderRoundTripper)
			assert.True(t, ok, "should be able to cast to proxyHeaderRoundTripper")

			if tt.expectedEmpty {
				assert.Empty(t, phrt.proxyHeaders, "proxy headers should be empty")
				return
			}

			assert.Equal(t, tt.expectedCount, len(phrt.proxyHeaders), "should have expected number of headers")
			assert.Equal(t, tt.expectedValues, phrt.proxyHeaders[tt.expectedHeader], "should have expected header values")
		})
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name           string
		roundTripper   http.RoundTripper
		headers        map[string]string
		expectedError  bool
		expectedStatus int
	}{
		{
			name: "with http transport",
			roundTripper: &http.Transport{
				ProxyConnectHeader: make(http.Header),
			},
			headers: map[string]string{
				"Proxy-Authorization": "Basic xyz",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "with custom round tripper",
			roundTripper: &mockRoundTripper{
				response: &http.Response{
					StatusCode: http.StatusOK,
				},
			},
			headers: map[string]string{
				"Custom-Header": "value",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "with error",
			roundTripper: &mockRoundTripper{
				err: assert.AnError,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phrt := &proxyHeaderRoundTripper{
				proxyHeaders: parseProxyHeaders(tt.headers),
				roundTripper: tt.roundTripper,
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			req, err := http.NewRequest(http.MethodGet, server.URL, nil)
			assert.NoError(t, err, "should create request without error")

			resp, err := phrt.RoundTrip(req)

			if tt.expectedError {
				assert.Error(t, err, "should return error")
				assert.Nil(t, resp, "response should be nil")
				return
			}

			assert.NoError(t, err, "should not return error")
			assert.NotNil(t, resp, "response should not be nil")
			assert.Equal(t, tt.expectedStatus, resp.StatusCode, "should have expected status code")
		})
	}
}

func TestParseProxyHeaders(t *testing.T) {
	tests := []struct {
		name           string
		headers        map[string]string
		expectedEmpty  bool
		expectedCount  int
		expectedHeader string
		expectedValues []string
	}{
		{
			name:          "nil headers",
			headers:       nil,
			expectedEmpty: true,
		},
		{
			name:          "empty headers",
			headers:       map[string]string{},
			expectedEmpty: true,
		},
		{
			name: "single header",
			headers: map[string]string{
				"proxy-authorization": "Basic xyz",
			},
			expectedCount:  1,
			expectedHeader: "Proxy-Authorization",
			expectedValues: []string{"Basic xyz"},
		},
		{
			name: "multiple comma-separated values",
			headers: map[string]string{
				"x-custom-header": "value1,value2,value3",
			},
			expectedCount:  1,
			expectedHeader: "X-Custom-Header",
			expectedValues: []string{"value1", "value2", "value3"},
		},
		{
			name: "multiple headers",
			headers: map[string]string{
				"proxy-authorization": "Basic xyz",
				"x-custom-header":     "value1,value2",
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseProxyHeaders(tt.headers)

			if tt.expectedEmpty {
				assert.Nil(t, result, "headers should be nil")
				return
			}

			assert.Equal(t, tt.expectedCount, len(result), "should have expected number of headers")

			if tt.expectedHeader != "" {
				assert.Equal(t, tt.expectedValues, result[tt.expectedHeader], "should have expected header values")
			}
		})
	}
}

func TestProxyHeaderRoundTripper_PreservesTransportAndPool(t *testing.T) {
	headers := map[string]string{
		"Proxy-Authorization": "Basic test-auth",
		"X-Proxy-Header":      "val1,val2",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	baseTransport := &http.Transport{}
	wrapper := NewProxyHeaderRoundTripperWrapperConstructor(nil, headers)
	rt := wrapper(baseTransport)

	phrt, ok := rt.(*proxyHeaderRoundTripper)
	assert.True(t, ok, "should be *proxyHeaderRoundTripper")

	// Base transport from caller/cache must remain unmutated (isolated).
	assert.Nil(t, baseTransport.ProxyConnectHeader, "base transport must not be mutated in-place")

	// Wrapper must own an isolated transport cloned once at construction time.
	ownedTransport, ok := phrt.roundTripper.(*http.Transport)
	assert.True(t, ok, "roundTripper must be *http.Transport")
	assert.NotSame(t, baseTransport, ownedTransport, "wrapper must own an isolated transport clone")
	assert.Equal(t, []string{"Basic test-auth"}, ownedTransport.ProxyConnectHeader["Proxy-Authorization"])
	assert.Equal(t, []string{"val1", "val2"}, ownedTransport.ProxyConnectHeader["X-Proxy-Header"])

	// Verify transport is NOT cloned on subsequent requests; pointer identity remains constant.
	for range 3 {
		req, err := http.NewRequest(http.MethodGet, server.URL, nil)
		assert.NoError(t, err)

		resp, err := rt.RoundTrip(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		_ = resp.Body.Close()

		// Assert ownedTransport retains the exact same pointer across requests.
		assert.Same(t, ownedTransport, phrt.roundTripper, "owned transport instance must be preserved across requests")
	}
}

func TestProxyHeaderRoundTripper_TransportIsolation(t *testing.T) {
	baseTransport := &http.Transport{}

	headers1 := map[string]string{
		"Proxy-Authorization": "Basic cluster-1-token",
	}
	headers2 := map[string]string{
		"Proxy-Authorization": "Basic cluster-2-token",
	}

	wrapper1 := NewProxyHeaderRoundTripperWrapperConstructor(nil, headers1)
	wrapper2 := NewProxyHeaderRoundTripperWrapperConstructor(nil, headers2)

	rt1 := wrapper1(baseTransport)
	rt2 := wrapper2(baseTransport)

	// Caller's shared/cached base transport must remain pristine.
	assert.Nil(t, baseTransport.ProxyConnectHeader, "base transport must not be mutated")

	phrt1, ok1 := rt1.(*proxyHeaderRoundTripper)
	phrt2, ok2 := rt2.(*proxyHeaderRoundTripper)
	assert.True(t, ok1)
	assert.True(t, ok2)

	tr1, ok1 := phrt1.roundTripper.(*http.Transport)
	tr2, ok2 := phrt2.roundTripper.(*http.Transport)
	assert.True(t, ok1)
	assert.True(t, ok2)

	// Both wrappers must own distinct transports with their respective credentials.
	assert.NotSame(t, tr1, tr2, "each wrapper instance must have its own isolated transport")
	assert.Equal(t, []string{"Basic cluster-1-token"}, tr1.ProxyConnectHeader["Proxy-Authorization"])
	assert.Equal(t, []string{"Basic cluster-2-token"}, tr2.ProxyConnectHeader["Proxy-Authorization"])
}

func TestProxyHeaderRoundTripper_WithMiddleware(t *testing.T) {
	headers := map[string]string{
		"Proxy-Authorization": "Basic middleware-test",
	}

	var middlewareReceivedRT http.RoundTripper
	middleware := func(rt http.RoundTripper) http.RoundTripper {
		middlewareReceivedRT = rt
		return &mockWrapperRoundTripper{inner: rt}
	}

	baseTransport := &http.Transport{}
	wrapper := NewProxyHeaderRoundTripperWrapperConstructor(middleware, headers)
	rt := wrapper(baseTransport)

	phrt, ok := rt.(*proxyHeaderRoundTripper)
	assert.True(t, ok)

	// Base transport must not be mutated.
	assert.Nil(t, baseTransport.ProxyConnectHeader)

	// Middleware must have received the owned transport with ProxyConnectHeader already configured.
	assert.NotNil(t, middlewareReceivedRT)
	ownedTransport, ok := middlewareReceivedRT.(*http.Transport)
	assert.True(t, ok)
	assert.NotSame(t, baseTransport, ownedTransport)
	assert.Equal(t, []string{"Basic middleware-test"}, ownedTransport.ProxyConnectHeader["Proxy-Authorization"])

	// The outer round tripper wraps the middleware.
	wrapped, ok := phrt.roundTripper.(*mockWrapperRoundTripper)
	assert.True(t, ok)
	assert.Same(t, ownedTransport, wrapped.inner)
}

func TestProxyHeaderRoundTripper_WrappedTransport(t *testing.T) {
	headers := map[string]string{
		"Proxy-Authorization": "Basic token",
	}

	baseTransport := &http.Transport{}
	wrappedRT := &mockWrapperRoundTripper{inner: baseTransport}
	wrapper := NewProxyHeaderRoundTripperWrapperConstructor(nil, headers)

	rt := wrapper(wrappedRT)
	phrt, ok := rt.(*proxyHeaderRoundTripper)
	assert.True(t, ok, "should be *proxyHeaderRoundTripper")

	// Base transport must remain unmutated.
	assert.Nil(t, baseTransport.ProxyConnectHeader, "base transport must not be mutated")

	// Wrapped transport inner must have been updated to the owned transport.
	ownedTransport, ok := wrappedRT.inner.(*http.Transport)
	assert.True(t, ok)
	assert.NotSame(t, baseTransport, ownedTransport)
	assert.Equal(t, []string{"Basic token"}, ownedTransport.ProxyConnectHeader["Proxy-Authorization"])

	// Assert WrappedRoundTripper and Unwrap methods.
	assert.Same(t, wrappedRT, phrt.WrappedRoundTripper())
	assert.Same(t, wrappedRT, phrt.Unwrap())
}

func TestProxyHeaderRoundTripper_HeaderMerge(t *testing.T) {
	initialHeader := make(http.Header)
	initialHeader["Existing-Header"] = []string{"existing-val"}

	baseTransport := &http.Transport{
		ProxyConnectHeader: initialHeader,
	}

	headers := map[string]string{
		"Proxy-Authorization": "Basic 123",
	}

	wrapper := NewProxyHeaderRoundTripperWrapperConstructor(nil, headers)
	rt := wrapper(baseTransport)

	// Base transport must retain only its initial headers and not be polluted with the new ones.
	assert.Equal(t, []string{"existing-val"}, baseTransport.ProxyConnectHeader["Existing-Header"])
	assert.Nil(t, baseTransport.ProxyConnectHeader["Proxy-Authorization"])

	// Cloned owned transport must contain both existing and new headers.
	phrt := rt.(*proxyHeaderRoundTripper)
	ownedTransport := phrt.roundTripper.(*http.Transport)
	assert.Equal(t, []string{"existing-val"}, ownedTransport.ProxyConnectHeader["Existing-Header"])
	assert.Equal(t, []string{"Basic 123"}, ownedTransport.ProxyConnectHeader["Proxy-Authorization"])
}

func TestProxyHeaderRoundTripper_DirectConstructionRoundTrip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mockRT := &mockRoundTripper{
		response: &http.Response{
			StatusCode: http.StatusOK,
		},
	}
	phrt := &proxyHeaderRoundTripper{
		proxyHeaders: parseProxyHeaders(map[string]string{
			"Proxy-Authorization": "Basic xyz",
		}),
		roundTripper: mockRT,
	}

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	assert.NoError(t, err)

	resp, err := phrt.RoundTrip(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Mock Implementations

type mockRoundTripper struct {
	response *http.Response
	err      error
}

func (m *mockRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return m.response, m.err
}

type mockWrapperRoundTripper struct {
	inner http.RoundTripper
}

func (m *mockWrapperRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.inner.RoundTrip(req)
}

func (m *mockWrapperRoundTripper) WrappedRoundTripper() http.RoundTripper {
	return m.inner
}

func (m *mockWrapperRoundTripper) Unwrap() http.RoundTripper {
	return m.inner
}

func (m *mockWrapperRoundTripper) SetWrappedRoundTripper(rt http.RoundTripper) {
	m.inner = rt
}
