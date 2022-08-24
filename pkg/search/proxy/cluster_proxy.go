package proxy

import (
	"context"
	"fmt"
	"net/http"
	"path"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	listcorev1 "k8s.io/client-go/listers/core/v1"

	clusterlisters "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/search/proxy/store"
	"github.com/karmada-io/karmada/pkg/util/proxy"
)

// clusterProxy proxy to member clusters
type clusterProxy struct {
	store         store.Cache
	clusterLister clusterlisters.ClusterLister
	secretLister  listcorev1.SecretLister
}

func newClusterProxy(store store.Cache, clusterLister clusterlisters.ClusterLister, secretLister listcorev1.SecretLister) *clusterProxy {
	return &clusterProxy{
		store:         store,
		clusterLister: clusterLister,
		secretLister:  secretLister,
	}
}

func (c *clusterProxy) connect(ctx context.Context, requestInfo *request.RequestInfo, gvr schema.GroupVersionResource, proxyPath string, responder rest.Responder) (http.Handler, error) {
	if requestInfo.Verb == "create" {
		return nil, apierrors.NewMethodNotSupported(gvr.GroupResource(), requestInfo.Verb)
	}

	_, clusterName, err := c.store.GetResourceFromCache(ctx, gvr, requestInfo.Namespace, requestInfo.Name)
	if err != nil {
		return nil, err
	}
	return c.connectCluster(ctx, clusterName, proxyPath, responder)
}

func (c *clusterProxy) connectCluster(ctx context.Context, clusterName string, proxyPath string, responder rest.Responder) (http.Handler, error) {
	cluster, err := c.clusterLister.Get(clusterName)
	if err != nil {
		return nil, err
	}
	location, transport, err := proxy.Location(cluster.Name, cluster.Spec.APIEndpoint, cluster.Spec.ProxyURL)
	if err != nil {
		return nil, err
	}
	location.Path = path.Join(location.Path, proxyPath)

	secretGetter := func(context.Context, string) (*corev1.Secret, error) {
		if cluster.Spec.ImpersonatorSecretRef == nil {
			return nil, fmt.Errorf("the impersonatorSecretRef of cluster %s is nil", cluster.Name)
		}
		return c.secretLister.Secrets(cluster.Spec.ImpersonatorSecretRef.Namespace).Get(cluster.Spec.ImpersonatorSecretRef.Name)
	}
	return proxy.ConnectCluster(ctx, cluster.Name, location, transport, responder, secretGetter)
}
