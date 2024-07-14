/*
Copyright 2017 The Kubernetes Authors.

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

package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mxk/go-flowrate/flowrate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/httpstream"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	proxyutil "k8s.io/apimachinery/pkg/util/proxy"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
)

// UpgradeAwareHandler is an interface for dialing a backend for an upgrade request
type UpgradeAwareHandler struct {
	proxyutil.UpgradeAwareHandler

	UpgradeDialer *UpgradeDialer
}

const defaultFlushInterval = 200 * time.Millisecond

// normalizeLocation returns the result of parsing the full URL, with scheme set to http if missing
func normalizeLocation(location *url.URL) *url.URL {
	normalized, _ := url.Parse(location.String())
	if len(normalized.Scheme) == 0 {
		normalized.Scheme = "http"
	}
	return normalized
}

// NewUpgradeAwareHandler creates a new proxy handler with a default flush interval. Responder is required for returning
// errors to the caller.
func NewUpgradeAwareHandler(location *url.URL, transport http.RoundTripper, wrapTransport, upgradeRequired bool, responder proxyutil.ErrorResponder) *UpgradeAwareHandler {
	return &UpgradeAwareHandler{
		UpgradeAwareHandler: proxyutil.UpgradeAwareHandler{
			Location:        normalizeLocation(location),
			Transport:       transport,
			WrapTransport:   wrapTransport,
			UpgradeRequired: upgradeRequired,
			FlushInterval:   defaultFlushInterval,
			Responder:       responder,
		},
	}
}

func proxyRedirectsforRootPath(path string, w http.ResponseWriter, req *http.Request) bool {
	redirect := false
	method := req.Method

	// From pkg/genericapiserver/endpoints/handlers/proxy.go#ServeHTTP:
	// Redirect requests with an empty path to a location that ends with a '/'
	// This is essentially a hack for https://issue.k8s.io/4958.
	// Note: Keep this code after tryUpgrade to not break that flow.
	if len(path) == 0 && (method == http.MethodGet || method == http.MethodHead) {
		var queryPart string
		if len(req.URL.RawQuery) > 0 {
			queryPart = "?" + req.URL.RawQuery
		}
		w.Header().Set("Location", req.URL.Path+"/"+queryPart)
		w.WriteHeader(http.StatusMovedPermanently)
		redirect = true
	}
	return redirect
}

// ServeHTTP handles the proxy request
//
//nolint:gocyclo
func (h *UpgradeAwareHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if h.tryUpgrade(w, req) {
		return
	}
	if h.UpgradeRequired {
		h.Responder.Error(w, req, apierrors.NewBadRequest("Upgrade request required"))
		return
	}

	loc := *h.Location
	loc.RawQuery = req.URL.RawQuery

	// If original request URL ended in '/', append a '/' at the end of the
	// of the proxy URL
	if !strings.HasSuffix(loc.Path, "/") && strings.HasSuffix(req.URL.Path, "/") {
		loc.Path += "/"
	}

	proxyRedirect := proxyRedirectsforRootPath(loc.Path, w, req)
	if proxyRedirect {
		return
	}

	if h.Transport == nil || h.WrapTransport {
		h.Transport = h.defaultProxyTransport(req.URL, h.Transport)
	}

	// WithContext creates a shallow clone of the request with the same context.
	newReq := req.WithContext(req.Context())
	newReq.Header = utilnet.CloneHeader(req.Header)
	if !h.UseRequestLocation {
		newReq.URL = &loc
	}
	if h.UseLocationHost {
		// exchanging req.Host with the backend location is necessary for backends that act on the HTTP host header (e.g. API gateways),
		// because req.Host has preference over req.URL.Host in filling this header field
		newReq.Host = h.Location.Host
	}

	// create the target location to use for the reverse proxy
	reverseProxyLocation := &url.URL{Scheme: h.Location.Scheme, Host: h.Location.Host}
	if h.AppendLocationPath {
		reverseProxyLocation.Path = h.Location.Path
	}

	proxy := httputil.NewSingleHostReverseProxy(reverseProxyLocation)
	proxy.Transport = h.Transport
	proxy.FlushInterval = h.FlushInterval
	proxy.ErrorLog = log.New(noSuppressPanicError{}, "", log.LstdFlags)
	if h.RejectForwardingRedirects {
		oldModifyResponse := proxy.ModifyResponse
		proxy.ModifyResponse = func(response *http.Response) error {
			code := response.StatusCode
			if code >= 300 && code <= 399 && len(response.Header.Get("Location")) > 0 {
				// close the original response
				response.Body.Close()
				msg := "the backend attempted to redirect this request, which is not permitted"
				// replace the response
				*response = http.Response{
					StatusCode:    http.StatusBadGateway,
					Status:        fmt.Sprintf("%d %s", response.StatusCode, http.StatusText(response.StatusCode)),
					Body:          io.NopCloser(strings.NewReader(msg)),
					ContentLength: int64(len(msg)),
				}
			} else {
				if oldModifyResponse != nil {
					if err := oldModifyResponse(response); err != nil {
						return err
					}
				}
			}
			return nil
		}
	}
	if h.Responder != nil {
		// if an optional error interceptor/responder was provided wire it
		// the custom responder might be used for providing a unified error reporting
		// or supporting retry mechanisms by not sending non-fatal errors to the clients
		proxy.ErrorHandler = h.Responder.Error
	}
	proxy.ServeHTTP(w, newReq)
}

type noSuppressPanicError struct{}

func (noSuppressPanicError) Write(p []byte) (n int, err error) {
	// skip "suppressing panic for copyResponse error in test; copy error" error message
	// that ends up in CI tests on each kube-apiserver termination as noise and
	// everybody thinks this is fatal.
	if strings.Contains(string(p), "suppressing panic") {
		return len(p), nil
	}
	return os.Stderr.Write(p)
}

// tryUpgrade returns true if the request was handled.
//
//nolint:gocyclo
func (h *UpgradeAwareHandler) tryUpgrade(w http.ResponseWriter, req *http.Request) bool {
	if !httpstream.IsUpgradeRequest(req) {
		klog.V(6).Infof("Request was not an upgrade")
		return false
	}

	var (
		backendConn net.Conn
		rawResponse []byte
		err         error
	)

	location := *h.Location
	if h.UseRequestLocation {
		location = *req.URL
		location.Scheme = h.Location.Scheme
		location.Host = h.Location.Host
		if h.AppendLocationPath {
			location.Path = singleJoiningSlash(h.Location.Path, location.Path)
		}
	}

	clone := utilnet.CloneRequest(req)
	// Only append X-Forwarded-For in the upgrade path, since httputil.NewSingleHostReverseProxy
	// handles this in the non-upgrade path.
	utilnet.AppendForwardedForHeader(clone)
	klog.V(6).Infof("Connecting to backend proxy (direct dial) %s\n  Headers: %v", &location, clone.Header)
	if h.UseLocationHost {
		clone.Host = h.Location.Host
	}
	clone.URL = &location

	//backendConn, err = h.DialForUpgrade(clone)
	backendConn, err = h.UpgradeDialer.Dial(clone)
	if err != nil {
		klog.V(6).Infof("Proxy connection error: %v", err)
		h.Responder.Error(w, req, err)
		return true
	}
	defer backendConn.Close()

	// determine the http response code from the backend by reading from rawResponse+backendConn
	backendHTTPResponse, headerBytes, err := getResponse(io.MultiReader(bytes.NewReader(rawResponse), backendConn))
	if err != nil {
		klog.V(6).Infof("Proxy connection error: %v", err)
		h.Responder.Error(w, req, err)
		return true
	}
	if len(headerBytes) > len(rawResponse) {
		// we read beyond the bytes stored in rawResponse, update rawResponse to the full set of bytes read from the backend
		rawResponse = headerBytes
	}

	// If the backend did not upgrade the request, return an error to the client. If the response was
	// an error, the error is forwarded directly after the connection is hijacked. Otherwise, just
	// return a generic error here.
	if backendHTTPResponse.StatusCode != http.StatusSwitchingProtocols && backendHTTPResponse.StatusCode < 400 {
		err := fmt.Errorf("invalid upgrade response: status code %d", backendHTTPResponse.StatusCode)
		klog.Errorf("Proxy upgrade error: %v", err)
		h.Responder.Error(w, req, err)
		return true
	}

	// Once the connection is hijacked, the ErrorResponder will no longer work, so
	// hijacking should be the last step in the upgrade.
	requestHijacker, ok := w.(http.Hijacker)
	if !ok {
		klog.V(6).Infof("Unable to hijack response writer: %T", w)
		h.Responder.Error(w, req, fmt.Errorf("request connection cannot be hijacked: %T", w))
		return true
	}
	requestHijackedConn, _, err := requestHijacker.Hijack()
	if err != nil {
		klog.V(6).Infof("Unable to hijack response: %v", err)
		h.Responder.Error(w, req, fmt.Errorf("error hijacking connection: %v", err))
		return true
	}
	defer requestHijackedConn.Close()

	if backendHTTPResponse.StatusCode != http.StatusSwitchingProtocols {
		// If the backend did not upgrade the request, echo the response from the backend to the client and return, closing the connection.
		klog.V(6).Infof("Proxy upgrade error, status code %d", backendHTTPResponse.StatusCode)
		// set read/write deadlines
		deadline := time.Now().Add(10 * time.Second)
		_ = backendConn.SetReadDeadline(deadline)
		_ = requestHijackedConn.SetWriteDeadline(deadline)
		// write the response to the client
		err := backendHTTPResponse.Write(requestHijackedConn)
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			klog.Errorf("Error proxying data from backend to client: %v", err)
		}
		// Indicate we handled the request
		return true
	}

	// Forward raw response bytes back to client.
	if len(rawResponse) > 0 {
		klog.V(6).Infof("Writing %d bytes to hijacked connection", len(rawResponse))
		if _, err = requestHijackedConn.Write(rawResponse); err != nil {
			utilruntime.HandleError(fmt.Errorf("error proxying response from backend to client: %v", err))
		}
	}

	// Proxy the connection. This is bidirectional, so we need a goroutine
	// to copy in each direction. Once one side of the connection exits, we
	// exit the function which performs cleanup and in the process closes
	// the other half of the connection in the defer.
	writerComplete := make(chan struct{})
	readerComplete := make(chan struct{})

	go func() {
		var writer io.WriteCloser
		if h.MaxBytesPerSec > 0 {
			writer = flowrate.NewWriter(backendConn, h.MaxBytesPerSec)
		} else {
			writer = backendConn
		}
		_, err := io.Copy(writer, requestHijackedConn)
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			klog.Errorf("Error proxying data from client to backend: %v", err)
		}
		close(writerComplete)
	}()

	go func() {
		var reader io.ReadCloser
		if h.MaxBytesPerSec > 0 {
			reader = flowrate.NewReader(backendConn, h.MaxBytesPerSec)
		} else {
			reader = backendConn
		}
		_, err := io.Copy(requestHijackedConn, reader)
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			klog.Errorf("Error proxying data from backend to client: %v", err)
		}
		close(readerComplete)
	}()

	// Wait for one half the connection to exit. Once it does the defer will
	// clean up the other half of the connection.
	select {
	case <-writerComplete:
	case <-readerComplete:
	}
	klog.V(6).Infof("Disconnecting from backend proxy %s\n  Headers: %v", &location, clone.Header)

	return true
}

// FIXME: Taken from net/http/httputil/reverseproxy.go as singleJoiningSlash is not exported to be re-used.
// See-also: https://github.com/golang/go/issues/44290
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

// getResponseCode reads a http response from the given reader, returns the response,
// the bytes read from the reader, and any error encountered
func getResponse(r io.Reader) (*http.Response, []byte, error) {
	rawResponse := bytes.NewBuffer(make([]byte, 0, 256))
	// Save the bytes read while reading the response headers into the rawResponse buffer
	resp, err := http.ReadResponse(bufio.NewReader(io.TeeReader(r, rawResponse)), nil)
	if err != nil {
		return nil, nil, err
	}
	// return the http response and the raw bytes consumed from the reader in the process
	return resp, rawResponse.Bytes(), nil
}

func (h *UpgradeAwareHandler) defaultProxyTransport(url *url.URL, internalTransport http.RoundTripper) http.RoundTripper {
	scheme := url.Scheme
	host := url.Host
	suffix := h.Location.Path
	if strings.HasSuffix(url.Path, "/") && !strings.HasSuffix(suffix, "/") {
		suffix += "/"
	}
	pathPrepend := strings.TrimSuffix(url.Path, suffix)
	rewritingTransport := &proxyutil.Transport{
		Scheme:       scheme,
		Host:         host,
		PathPrepend:  pathPrepend,
		RoundTripper: internalTransport,
	}
	return &corsRemovingTransport{
		RoundTripper: rewritingTransport,
	}
}

// corsRemovingTransport is a wrapper for an internal transport. It removes CORS headers
// from the internal response.
// Implements pkg/util/net.RoundTripperWrapper
type corsRemovingTransport struct {
	http.RoundTripper
}

var _ = utilnet.RoundTripperWrapper(&corsRemovingTransport{})

func (rt *corsRemovingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.RoundTripper.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	removeCORSHeaders(resp)
	return resp, nil
}

func (rt *corsRemovingTransport) WrappedRoundTripper() http.RoundTripper {
	return rt.RoundTripper
}

// removeCORSHeaders strip CORS headers sent from the backend
// This should be called on all responses before returning
func removeCORSHeaders(resp *http.Response) {
	resp.Header.Del("Access-Control-Allow-Credentials")
	resp.Header.Del("Access-Control-Allow-Headers")
	resp.Header.Del("Access-Control-Allow-Methods")
	resp.Header.Del("Access-Control-Allow-Origin")
}
