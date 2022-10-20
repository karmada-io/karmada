package cluster

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	listcorev1 "k8s.io/client-go/listers/core/v1"

	clusterlisters "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/search/proxy/framework"
	pluginruntime "github.com/karmada-io/karmada/pkg/search/proxy/framework/runtime"
	"github.com/karmada-io/karmada/pkg/search/proxy/store"
	"github.com/karmada-io/karmada/pkg/util/proxy"
)

const (
	// We keep a big gap between in-tree plugins, to allow users to insert custom plugins between them.
	order = 2000
)

// Cluster proxies the remaining requests to member clusters:
// - writing resources.
// - or subresource requests, e.g. `pods/log`
// We firstly find the resource from cache, and get the located cluster. Then redirect the request to the cluster.
type Cluster struct {
	store         store.Store
	clusterLister clusterlisters.ClusterLister
	secretLister  listcorev1.SecretLister
}

var _ framework.Plugin = (*Cluster)(nil)

// New creates an instance of Cluster
func New(dep pluginruntime.PluginDependency) (framework.Plugin, error) {
	secretLister := dep.KubeFactory.Core().V1().Secrets().Lister()
	clusterLister := dep.KarmadaFactory.Cluster().V1alpha1().Clusters().Lister()

	return &Cluster{
		store:         dep.Store,
		clusterLister: clusterLister,
		secretLister:  secretLister,
	}, nil
}

// Order implements Plugin
func (c *Cluster) Order() int {
	return order
}

// SupportRequest implements Plugin
func (c *Cluster) SupportRequest(request framework.ProxyRequest) bool {
	return request.RequestInfo.IsResourceRequest && c.store.HasResource(request.GroupVersionResource)
}

// Connect implements Plugin
func (c *Cluster) Connect(ctx context.Context, request framework.ProxyRequest) (http.Handler, error) {
	requestInfo := request.RequestInfo

	if requestInfo.Verb == "create" {
		return nil, apierrors.NewMethodNotSupported(request.GroupVersionResource.GroupResource(), requestInfo.Verb)
	}

	_, clusterName, err := c.store.GetResourceFromCache(ctx, request.GroupVersionResource, requestInfo.Namespace, requestInfo.Name)
	if err != nil {
		return nil, err
	}

	h, err := proxy.ConnectCluster(ctx, c.clusterLister, c.secretLister, clusterName, request.ProxyPath, request.Responder)
	if err != nil {
		return nil, err
	}

	if requestInfo.Verb != "update" {
		return h, nil
	}

	// Objects get by client via proxy are edited some fields, different from objets in member clusters.
	// So before update, we shall recover these fields.
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err = modifyRequest(req, clusterName); err != nil {
			request.Responder.Error(err)
			return
		}
		h.ServeHTTP(rw, req)
	}), nil
}

func modifyRequest(req *http.Request, cluster string) error {
	if req.ContentLength == 0 {
		return nil
	}

	body := bytes.NewBuffer(make([]byte, 0, req.ContentLength))
	_, err := io.Copy(body, req.Body)
	if err != nil {
		return err
	}
	_ = req.Body.Close()

	defer func() {
		req.Body = ioutil.NopCloser(body)
		req.ContentLength = int64(body.Len())
	}()

	obj := &unstructured.Unstructured{}
	_, _, err = unstructured.UnstructuredJSONScheme.Decode(body.Bytes(), nil, obj)
	if err != nil {
		// ignore error
		return nil
	}

	changed := false
	changed = store.RemoveCacheSourceAnnotation(obj) || changed
	changed = store.RecoverClusterResourceVersion(obj, cluster) || changed

	if changed {
		// write changed object into body
		body.Reset()
		return unstructured.UnstructuredJSONScheme.Encode(obj, body)
	}

	return nil
}
