/*
Copyright 2022 The Karmada Authors.

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

package cluster

import (
	"bytes"
	"context"
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	listcorev1 "k8s.io/client-go/listers/core/v1"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
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

	// For creating request, cluster proxy doesn't know which cluster to create, so responses MethodNotSupported error.
	// While for subresource request, having resource name request (like pods attach, exec and port-forward),
	// proxy it to the cluster the resource located.
	if requestInfo.Verb == "create" && requestInfo.Name == "" {
		return nil, apierrors.NewMethodNotSupported(request.GroupVersionResource.GroupResource(), requestInfo.Verb)
	}

	_, clusterName, err := c.store.GetResourceFromCache(ctx, request.GroupVersionResource, requestInfo.Namespace, requestInfo.Name)
	if err != nil {
		return nil, err
	}

	cls, err := c.clusterLister.Get(clusterName)
	if err != nil {
		return nil, err
	}

	cluster := &clusterapis.Cluster{}
	err = clusterv1alpha1.Convert_v1alpha1_Cluster_To_cluster_Cluster(cls, cluster, nil)
	if err != nil {
		return nil, err
	}

	secretGetter := func(_ context.Context, namespace, name string) (*corev1.Secret, error) {
		return c.secretLister.Secrets(namespace).Get(name)
	}

	h, err := proxy.ConnectCluster(ctx, cluster, request.ProxyPath, secretGetter, request.Responder)
	if err != nil {
		return nil, err
	}

	if requestInfo.Verb != "update" {
		return h, nil
	}

	// Objects get by client via proxy are edited some fields, different from objects in member clusters.
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
		req.Body = io.NopCloser(body)
		req.ContentLength = int64(body.Len())
	}()

	obj := &unstructured.Unstructured{}
	_, _, err = unstructured.UnstructuredJSONScheme.Decode(body.Bytes(), nil, obj)
	if err != nil {
		// ignore error
		return nil
	}

	changed := store.RemoveCacheSourceAnnotation(obj)
	changed = store.RecoverClusterResourceVersion(obj, cluster) || changed

	if changed {
		// write changed object into body
		body.Reset()
		return unstructured.UnstructuredJSONScheme.Encode(obj, body)
	}

	return nil
}
