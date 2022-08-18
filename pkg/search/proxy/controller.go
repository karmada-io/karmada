package proxy

import (
	"context"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/informers"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	clusterlisters "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	searchlisters "github.com/karmada-io/karmada/pkg/generated/listers/search/v1alpha1"
	"github.com/karmada-io/karmada/pkg/search/proxy/store"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

const workKey = "key"

// Controller syncs Cluster and GlobalResource.
type Controller struct {
	clusterLister   clusterlisters.ClusterLister
	registryLister  searchlisters.ResourceRegistryLister
	worker          util.AsyncWorker
	informerManager genericmanager.MultiClusterInformerManager
	store           *store.MultiClusterCache

	// proxy
	karmadaProxy *karmadaProxy
	clusterProxy *clusterProxy
	cacheProxy   *cacheProxy
}

// NewController create a controller for proxy
func NewController(restConfig *restclient.Config, informerManager genericmanager.MultiClusterInformerManager, factory informers.SharedInformerFactory, karmadaFactory informerfactory.SharedInformerFactory) (*Controller, error) {
	kp, err := newKarmadaProxy(restConfig)
	if err != nil {
		return nil, err
	}

	s := &store.MultiClusterCache{}

	ctl := &Controller{
		clusterLister:   karmadaFactory.Cluster().V1alpha1().Clusters().Lister(),
		registryLister:  karmadaFactory.Search().V1alpha1().ResourceRegistries().Lister(),
		informerManager: informerManager,
		store:           s,
		karmadaProxy:    kp,
		clusterProxy:    newClusterProxy(factory.Core().V1().Secrets().Lister()),
		cacheProxy:      newCacheProxy(s),
	}

	workerOptions := util.Options{
		Name:          "proxy-controller",
		KeyFunc:       nil,
		ReconcileFunc: ctl.reconcile,
	}
	ctl.worker = util.NewAsyncWorker(workerOptions)

	resourceEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(interface{}) {
			ctl.worker.Add(workKey)
		},
		UpdateFunc: func(_, _ interface{}) {
			ctl.worker.Add(workKey)
		},
		DeleteFunc: func(interface{}) {
			ctl.worker.Add(workKey)
		},
	}

	karmadaFactory.Cluster().V1alpha1().Clusters().Informer().AddEventHandler(resourceEventHandler)
	karmadaFactory.Search().V1alpha1().ResourceRegistries().Informer().AddEventHandler(resourceEventHandler)

	return ctl, nil
}

// Start run the proxy controller
func (ctl *Controller) Start(stopCh <-chan struct{}) {
	ctl.worker.Run(1, stopCh)
}

// reconcile cache
func (ctl *Controller) reconcile(util.QueueKey) error {
	return nil
}

// Connect proxy and dispatch handlers
func (ctl *Controller) Connect(ctx context.Context, proxyPath string, responder rest.Responder) (http.Handler, error) {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		newReq := req.Clone(req.Context())
		newReq.URL.Path = proxyPath
		requestInfo := lifted.NewRequestInfo(newReq)

		newCtx := request.WithRequestInfo(ctx, requestInfo)
		newCtx = request.WithNamespace(newCtx, requestInfo.Namespace)
		newReq = newReq.WithContext(newCtx)

		h, err := ctl.connect(newCtx, requestInfo, proxyPath, responder)
		if err != nil {
			responder.Error(err)
			return
		}
		h.ServeHTTP(rw, newReq)
	}), nil
}

func (ctl *Controller) connect(ctx context.Context, requestInfo *request.RequestInfo, path string, responder rest.Responder) (http.Handler, error) {
	gvr := schema.GroupVersionResource{
		Group:    requestInfo.APIGroup,
		Version:  requestInfo.APIVersion,
		Resource: requestInfo.Resource,
	}

	// requests will be redirected to:
	// 1. karmada apiserver
	// 2. cache
	// 3. member clusters
	// see more information from https://github.com/karmada-io/karmada/tree/master/docs/proposals/resource-aggregation-proxy#request-routing

	// 1. For non-resource requests, or resources are not defined in ResourceRegistry,
	// we redirect the requests to karmada apiserver.
	// Usually the request are
	// - api index, e.g.: `/api`, `/apis`
	// - to workload created in karmada controller panel, such as deployments and services.
	if !requestInfo.IsResourceRequest || !ctl.store.HasResource(gvr) {
		return ctl.karmadaProxy.connect(path, responder)
	}

	// 2. For reading requests, we redirect them to cache.
	// Users call these requests to read resources in member clusters, such as pods and nodes.
	if requestInfo.Subresource == "" && (requestInfo.Verb == "get" || requestInfo.Verb == "list" || requestInfo.Verb == "watch") {
		return ctl.cacheProxy.connect(ctx), nil
	}

	// 3. The remaining requests are:
	// - writing resources.
	// - or subresource requests, e.g. `pods/log`
	// We firstly find the resource from cache, and get the located cluster. Then redirect the request to the cluster.
	cluster, err := ctl.store.GetClusterForResource(ctx, gvr)
	if err != nil {
		return nil, err
	}
	return ctl.clusterProxy.connect(ctx, cluster, path, responder)
}
