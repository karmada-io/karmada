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
	"maps"
	"net/http"
	"net/textproto"
	"strings"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/transport"
)

type proxyHeaderRoundTripper struct {
	proxyHeaders http.Header
	roundTripper http.RoundTripper
}

var _ http.RoundTripper = &proxyHeaderRoundTripper{}
var _ utilnet.RoundTripperWrapper = &proxyHeaderRoundTripper{}

// NewProxyHeaderRoundTripperWrapperConstructor returns a RoundTripper wrapper that's usable within restConfig.WrapTransport.
func NewProxyHeaderRoundTripperWrapperConstructor(wt transport.WrapperFunc, headers map[string]string) transport.WrapperFunc {
	proxyHeaders := parseProxyHeaders(headers)
	return func(rt http.RoundTripper) http.RoundTripper {
		if wt != nil {
			rt = wt(rt)
		}

		if len(proxyHeaders) > 0 {
			if tr, ok := rt.(*http.Transport); ok {
				ownedTr := tr.Clone()
				ownedTr.ProxyConnectHeader = proxyHeaders.Clone()
				rt = ownedTr
			} else if tr := extractTransport(rt); tr != nil {
				if tr.ProxyConnectHeader == nil {
					tr.ProxyConnectHeader = proxyHeaders.Clone()
				} else {
					maps.Copy(tr.ProxyConnectHeader, proxyHeaders)
				}
			}
		}

		return &proxyHeaderRoundTripper{
			proxyHeaders: proxyHeaders,
			roundTripper: rt,
		}
	}
}

// RoundTrip implements the http.RoundTripper interface.
func (r *proxyHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return r.roundTripper.RoundTrip(req)
}

// WrappedRoundTripper returns the wrapped http.RoundTripper, implementing utilnet.RoundTripperWrapper.
func (r *proxyHeaderRoundTripper) WrappedRoundTripper() http.RoundTripper {
	return r.roundTripper
}

// Unwrap returns the underlying http.RoundTripper.
func (r *proxyHeaderRoundTripper) Unwrap() http.RoundTripper {
	return r.roundTripper
}

func extractTransport(rt http.RoundTripper) *http.Transport {
	for rt != nil {
		switch t := rt.(type) {
		case *http.Transport:
			return t
		case utilnet.RoundTripperWrapper:
			rt = t.WrappedRoundTripper()
		case interface{ Unwrap() http.RoundTripper }:
			rt = t.Unwrap()
		default:
			return nil
		}
	}
	return nil
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
