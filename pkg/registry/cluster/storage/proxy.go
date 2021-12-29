package storage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
)

// ProxyREST implements the proxy subresource for a Cluster.
type ProxyREST struct {
	Store      *genericregistry.Store
	Redirector rest.Redirector

	kubeClient kubernetes.Interface
}

// Implement Connecter
var _ = rest.Connecter(&ProxyREST{})

var proxyMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

// New returns an empty cluster proxy subresource.
func (r *ProxyREST) New() runtime.Object {
	return &clusterapis.ClusterProxyOptions{}
}

// ConnectMethods returns the list of HTTP methods handled by Connect.
func (r *ProxyREST) ConnectMethods() []string {
	return proxyMethods
}

// NewConnectOptions returns versioned resource that represents proxy parameters.
func (r *ProxyREST) NewConnectOptions() (runtime.Object, bool, string) {
	return &clusterapis.ClusterProxyOptions{}, true, "path"
}

// Connect returns a handler for the cluster proxy.
func (r *ProxyREST) Connect(ctx context.Context, id string, options runtime.Object, responder rest.Responder) (http.Handler, error) {
	proxyOpts, ok := options.(*clusterapis.ClusterProxyOptions)
	if !ok {
		return nil, fmt.Errorf("invalid options object: %#v", options)
	}

	location, transport, err := r.Redirector.ResourceLocation(ctx, id)
	if err != nil {
		return nil, err
	}
	location.Path = proxyOpts.Path

	impersonateToken, err := r.getImpersonateToken(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get impresonateToken for cluster %s: %v", id, err)
	}

	return newProxyHandler(location, transport, impersonateToken, responder)
}

func (r *ProxyREST) getImpersonateToken(ctx context.Context, clusterName string) (string, error) {
	cluster, err := getCluster(ctx, r.Store, clusterName)
	if err != nil {
		return "", err
	}

	if cluster.Spec.ImpersonatorSecretRef == nil {
		return "", fmt.Errorf("the impersonatorSecretRef of cluster %s is nil", clusterName)
	}

	secret, err := r.kubeClient.CoreV1().Secrets(cluster.Spec.ImpersonatorSecretRef.Namespace).Get(context.TODO(),
		cluster.Spec.ImpersonatorSecretRef.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

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
			req.Header.Add(authenticationv1.ImpersonateGroupHeader, group)
		}

		req.Header.Set("Authorization", fmt.Sprintf("bearer %s", impersonateToken))

		handler := newThrottledUpgradeAwareProxyHandler(location, transport, true, false, responder)
		handler.ServeHTTP(rw, req)
	}), nil
}

func newThrottledUpgradeAwareProxyHandler(location *url.URL, transport http.RoundTripper, wrapTransport, upgradeRequired bool, responder rest.Responder) *proxy.UpgradeAwareHandler {
	handler := proxy.NewUpgradeAwareHandler(location, transport, wrapTransport, upgradeRequired, proxy.NewErrorResponder(responder))
	return handler
}
