package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	h, err := c.connectCluster(ctx, clusterName, proxyPath, responder)
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
			responder.Error(err)
			return
		}
		h.ServeHTTP(rw, req)
	}), nil
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
