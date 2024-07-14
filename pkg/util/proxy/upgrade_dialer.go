/*
Copyright 2015 The Kubernetes Authors.

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
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/third_party/forked/golang/netutil"
)

// UpgradeDialer knows how to upgrade an HTTP request to one that supports
// multiplexed streams. After Dial() is invoked, Conn will be usable.
type UpgradeDialer struct {
	//tlsConfig holds the TLS configuration settings to use when connecting
	//to the remote server.
	tlsConfig *tls.Config

	// Dialer is the dialer used to connect.  Used if non-nil.
	Dialer *net.Dialer

	// header holds the HTTP request headers for dialing.
	header http.Header

	// proxier knows which proxy to use given a request.
	proxier func(req *http.Request) (*url.URL, error)

	// pingPeriod is a period for sending Ping frames over established
	// connections.
	pingPeriod time.Duration
}

var _ utilnet.TLSClientConfigHolder = &UpgradeDialer{}
var _ utilnet.Dialer = &UpgradeDialer{}

// NewUpgradeDialerWithConfig creates a new UpgradeRoundTripper with the specified
// configuration.
func NewUpgradeDialerWithConfig(cfg UpgradeDialerWithConfig) *UpgradeDialer {
	if cfg.Proxier == nil {
		cfg.Proxier = utilnet.NewProxierWithNoProxyCIDR(http.ProxyFromEnvironment)
	}
	return &UpgradeDialer{
		tlsConfig:  cfg.TLS,
		proxier:    cfg.Proxier,
		pingPeriod: cfg.PingPeriod,
		header:     cfg.Header,
	}
}

// UpgradeDialerWithConfig is a set of options for an UpgradeDialer.
type UpgradeDialerWithConfig struct {
	// TLS configuration used by the round tripper.
	TLS *tls.Config
	// Header holds the HTTP request headers for dialing. Optional.
	Header http.Header
	// Proxier is a proxy function invoked on each request. Optional.
	Proxier func(*http.Request) (*url.URL, error)
	// PingPeriod is a period for sending Pings on the connection.
	// Optional.
	PingPeriod time.Duration
}

// TLSClientConfig implements pkg/util/net.TLSClientConfigHolder for proper TLS checking during
// proxying with a roundtripper.
func (u *UpgradeDialer) TLSClientConfig() *tls.Config {
	return u.tlsConfig
}

// Dial implements k8s.io/apimachinery/pkg/util/net.Dialer.
func (u *UpgradeDialer) Dial(req *http.Request) (net.Conn, error) {
	conn, err := u.dial(req)
	if err != nil {
		return nil, err
	}

	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// dial dials the host specified by req, using TLS if appropriate.
func (u *UpgradeDialer) dial(req *http.Request) (net.Conn, error) {
	proxyURL, err := u.proxier(req)
	if err != nil {
		return nil, err
	}

	if proxyURL == nil {
		return u.dialWithoutProxy(req.Context(), req.URL)
	}

	switch proxyURL.Scheme {
	case "socks5":
		return u.dialWithSocks5Proxy(req, proxyURL)
	case "https", "http", "":
		return u.dialWithHTTPProxy(req, proxyURL)
	}

	return nil, fmt.Errorf("proxy URL scheme not supported: %s", proxyURL.Scheme)
}

// dialWithHTTPProxy dials the host specified by url through an http or an https proxy.
func (u *UpgradeDialer) dialWithHTTPProxy(req *http.Request, proxyURL *url.URL) (net.Conn, error) {
	// ensure we use a canonical host with proxyReq
	targetHost := netutil.CanonicalAddr(req.URL)

	// proxying logic adapted from http://blog.h6t.eu/post/74098062923/golang-websocket-with-http-proxy-support
	proxyReq := http.Request{
		Method: "CONNECT",
		URL:    &url.URL{},
		Host:   targetHost,
		Header: u.header,
	}

	proxyReq = *proxyReq.WithContext(req.Context())

	if pa := u.proxyAuth(proxyURL); pa != "" {
		proxyReq.Header.Set("Proxy-Authorization", pa)
	}

	proxyDialConn, err := u.dialWithoutProxy(proxyReq.Context(), proxyURL)
	if err != nil {
		return nil, err
	}

	//nolint:staticcheck // SA1019 ignore deprecated httputil.NewProxyClientConn
	proxyClientConn := httputil.NewProxyClientConn(proxyDialConn, nil)
	response, err := proxyClientConn.Do(&proxyReq)
	//nolint:staticcheck // SA1019 ignore deprecated httputil.ErrPersistEOF: it might be
	// returned from the invocation of proxyClientConn.Do
	if err != nil && err != httputil.ErrPersistEOF {
		return nil, err
	}
	if response != nil && response.StatusCode >= 300 || response.StatusCode < 200 {
		return nil, fmt.Errorf("CONNECT request to %s returned response: %s", proxyURL.Redacted(), response.Status)
	}

	rwc, _ := proxyClientConn.Hijack()

	if req.URL.Scheme == "https" {
		return u.tlsConn(proxyReq.Context(), rwc, targetHost)
	}
	return rwc, nil
}

// dialWithSocks5Proxy dials the host specified by url through a socks5 proxy.
func (u *UpgradeDialer) dialWithSocks5Proxy(req *http.Request, proxyURL *url.URL) (net.Conn, error) {
	// ensure we use a canonical host with proxyReq
	targetHost := netutil.CanonicalAddr(req.URL)
	proxyDialAddr := netutil.CanonicalAddr(proxyURL)

	var auth *proxy.Auth
	if proxyURL.User != nil {
		pass, _ := proxyURL.User.Password()
		auth = &proxy.Auth{
			User:     proxyURL.User.Username(),
			Password: pass,
		}
	}

	dialer := u.Dialer
	if dialer == nil {
		dialer = &net.Dialer{
			Timeout: 30 * time.Second,
		}
	}

	proxyDialer, err := proxy.SOCKS5("tcp", proxyDialAddr, auth, dialer)
	if err != nil {
		return nil, err
	}

	// According to the implementation of proxy.SOCKS5, the type assertion will always succeed
	contextDialer, ok := proxyDialer.(proxy.ContextDialer)
	if !ok {
		return nil, errors.New("SOCKS5 Dialer must implement ContextDialer")
	}

	proxyDialConn, err := contextDialer.DialContext(req.Context(), "tcp", targetHost)
	if err != nil {
		return nil, err
	}

	if req.URL.Scheme == "https" {
		return u.tlsConn(req.Context(), proxyDialConn, targetHost)
	}
	return proxyDialConn, nil
}

// tlsConn returns a TLS client side connection using rwc as the underlying transport.
func (u *UpgradeDialer) tlsConn(ctx context.Context, rwc net.Conn, targetHost string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(targetHost)
	if err != nil {
		return nil, err
	}

	tlsConfig := u.tlsConfig
	switch {
	case tlsConfig == nil:
		tlsConfig = &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	case len(tlsConfig.ServerName) == 0:
		tlsConfig = tlsConfig.Clone()
		tlsConfig.ServerName = host
	}

	tlsConn := tls.Client(rwc, tlsConfig)

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		tlsConn.Close()
		return nil, err
	}

	return tlsConn, nil
}

// dialWithoutProxy dials the host specified by url, using TLS if appropriate.
func (u *UpgradeDialer) dialWithoutProxy(ctx context.Context, url *url.URL) (net.Conn, error) {
	dialAddr := netutil.CanonicalAddr(url)
	dialer := u.Dialer
	if dialer == nil {
		dialer = &net.Dialer{}
	}

	if url.Scheme == "http" {
		return dialer.DialContext(ctx, "tcp", dialAddr)
	}

	tlsDialer := tls.Dialer{
		NetDialer: dialer,
		Config:    u.tlsConfig,
	}
	return tlsDialer.DialContext(ctx, "tcp", dialAddr)
}

// proxyAuth returns, for a given proxy URL, the value to be used for the Proxy-Authorization header
func (u *UpgradeDialer) proxyAuth(proxyURL *url.URL) string {
	if proxyURL == nil || proxyURL.User == nil {
		return ""
	}
	username := proxyURL.User.Username()
	password, _ := proxyURL.User.Password()
	auth := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}
