package storage

import (
	"context"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	"github.com/karmada-io/karmada/pkg/util/proxy"
)

// ProxyREST implements the proxy subresource for a Cluster.
type ProxyREST struct {
	kubeClient    kubernetes.Interface
	clusterGetter func(ctx context.Context, name string) (*clusterapis.Cluster, error)
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

	cluster, err := r.clusterGetter(ctx, id)
	if err != nil {
		return nil, err
	}

	secretGetter := func(ctx context.Context, namespace string, name string) (*corev1.Secret, error) {
		return r.kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	}
	return proxy.ConnectCluster(ctx, cluster, proxyOpts.Path, secretGetter, responder)
}

// Destroy cleans up its resources on shutdown.
func (r *ProxyREST) Destroy() {
	// Given no underlying store, so we don't
	// need to destroy anything.
}
