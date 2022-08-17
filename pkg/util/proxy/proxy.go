package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
)

// NewThrottledUpgradeAwareProxyHandler creates a new proxy handler with a default flush interval. Responder is required for returning
// errors to the caller.
func NewThrottledUpgradeAwareProxyHandler(location *url.URL, transport http.RoundTripper, wrapTransport, upgradeRequired bool, responder rest.Responder) *proxy.UpgradeAwareHandler {
	return proxy.NewUpgradeAwareHandler(location, transport, wrapTransport, upgradeRequired, proxy.NewErrorResponder(responder))
}

// ConnectCluster returns a handler for proxy cluster.
func ConnectCluster(ctx context.Context, clusterName string, location *url.URL, transport http.RoundTripper, responder rest.Responder,
	impersonateSecretGetter func(context.Context, string) (*corev1.Secret, error)) (http.Handler, error) {
	secret, err := impersonateSecretGetter(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	impersonateToken, err := getImpersonateToken(clusterName, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get impresonateToken for cluster %s: %v", clusterName, err)
	}

	return newProxyHandler(location, transport, impersonateToken, responder)
}

// Location returns a URL to which one can send traffic for the specified cluster.
func Location(clusterName string, apiEndpoint string, proxyURL string) (*url.URL, http.RoundTripper, error) {
	location, err := constructLocation(clusterName, apiEndpoint)
	if err != nil {
		return nil, nil, err
	}

	transport, err := createProxyTransport(proxyURL)
	if err != nil {
		return nil, nil, err
	}

	return location, transport, nil
}

func constructLocation(clusterName string, apiEndpoint string) (*url.URL, error) {
	if apiEndpoint == "" {
		return nil, fmt.Errorf("API endpoint of cluster %s should not be empty", clusterName)
	}

	uri, err := url.Parse(apiEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse api endpoint %s: %v", apiEndpoint, err)
	}
	return uri, nil
}

func createProxyTransport(proxyURL string) (*http.Transport, error) {
	var proxyDialerFn utilnet.DialFunc
	proxyTLSClientConfig := &tls.Config{InsecureSkipVerify: true} // #nosec
	trans := utilnet.SetTransportDefaults(&http.Transport{
		DialContext:     proxyDialerFn,
		TLSClientConfig: proxyTLSClientConfig,
	})

	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse url of proxy url %s: %v", proxyURL, err)
		}
		trans.Proxy = http.ProxyURL(u)
	}
	return trans, nil
}

func getImpersonateToken(clusterName string, secret *corev1.Secret) (string, error) {
	token, found := secret.Data[clusterapis.SecretTokenKey]
	if !found {
		return "", fmt.Errorf("the impresonate token of cluster %s is empty", clusterName)
	}
	return string(token), nil
}

func newProxyHandler(location *url.URL, transport http.RoundTripper, impersonateToken string, responder rest.Responder) (http.Handler, error) {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		requester, exist := request.UserFrom(req.Context())
		if !exist {
			responsewriters.InternalError(rw, req, errors.New("no user found for request"))
			return
		}
		req.Header.Set(authenticationv1.ImpersonateUserHeader, requester.GetName())
		for _, group := range requester.GetGroups() {
			if !skipGroup(group) {
				req.Header.Add(authenticationv1.ImpersonateGroupHeader, group)
			}
		}

		req.Header.Set("Authorization", fmt.Sprintf("bearer %s", impersonateToken))

		// Retain RawQuery in location because upgrading the request will use it.
		// See https://github.com/karmada-io/karmada/issues/1618#issuecomment-1103793290 for more info.
		location.RawQuery = req.URL.RawQuery

		handler := NewThrottledUpgradeAwareProxyHandler(location, transport, true, false, responder)
		handler.ServeHTTP(rw, req)
	}), nil
}

func skipGroup(group string) bool {
	switch group {
	case user.AllAuthenticated, user.AllUnauthenticated:
		return true
	default:
		return false
	}
}
