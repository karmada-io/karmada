/*
Copyright 2021 The Karmada Authors.

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

package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	"github.com/karmada-io/karmada/pkg/util/proxy"
)

const matchAllClusters = "*"

// ProxyREST implements the proxy subresource for a Cluster.
type ProxyREST struct {
	restConfig    *restclient.Config
	kubeClient    kubernetes.Interface
	secretLister  listcorev1.SecretLister
	clusterGetter func(ctx context.Context, name string) (*clusterapis.Cluster, error)
	clusterLister func(ctx context.Context) (*clusterapis.ClusterList, error)

	karmadaLocation  *url.URL
	karmadaTransPort http.RoundTripper
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

	secretGetter := func(ctx context.Context, namespace string, name string) (secret *corev1.Secret, err error) {
		if secret, err = r.secretLister.Secrets(namespace).Get(name); err == nil {
			return secret, nil
		}
		if apierrors.IsNotFound(err) {
			return r.kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		}
		return nil, err
	}

	if id == matchAllClusters {
		return r.connectAllClusters(ctx, proxyOpts.Path, secretGetter, responder)
	}

	cluster, err := r.clusterGetter(ctx, id)
	if err != nil {
		return nil, err
	}

	return proxy.ConnectCluster(ctx, cluster, proxyOpts.Path, secretGetter, responder)
}

// Destroy cleans up its resources on shutdown.
func (r *ProxyREST) Destroy() {
	// Given no underlying store, so we don't
	// need to destroy anything.
}
