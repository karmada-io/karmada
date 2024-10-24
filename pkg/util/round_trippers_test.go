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

// Mock Implementations

type mockRoundTripper struct {
	response *http.Response
	err      error
}

func (m *mockRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return m.response, m.err
}
