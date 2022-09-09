package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/metrics"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/scheme"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	clusterlisters "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	searchlisters "github.com/karmada-io/karmada/pkg/generated/listers/search/v1alpha1"
	"github.com/karmada-io/karmada/pkg/search/proxy/store"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/lifted"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

const workKey = "key"

// Controller syncs Cluster and GlobalResource.
type Controller struct {
	restMapper           meta.RESTMapper
	negotiatedSerializer runtime.NegotiatedSerializer

	secretLister   listcorev1.SecretLister
	clusterLister  clusterlisters.ClusterLister
	registryLister searchlisters.ResourceRegistryLister
	worker         util.AsyncWorker
	store          store.Cache

	// proxy
	karmadaProxy *karmadaProxy
	clusterProxy *clusterProxy
	cacheProxy   *cacheProxy
}

// NewController create a controller for proxy
func NewController(restConfig *restclient.Config, restMapper meta.RESTMapper,
	kubeFactory informers.SharedInformerFactory, karmadaFactory informerfactory.SharedInformerFactory,
	minRequestTimeout time.Duration) (*Controller, error) {
	kp, err := newKarmadaProxy(restConfig)
	if err != nil {
		return nil, err
	}

	ctl := &Controller{
		restMapper:           restMapper,
		negotiatedSerializer: scheme.Codecs.WithoutConversion(),
		secretLister:         kubeFactory.Core().V1().Secrets().Lister(),
		clusterLister:        karmadaFactory.Cluster().V1alpha1().Clusters().Lister(),
		registryLister:       karmadaFactory.Search().V1alpha1().ResourceRegistries().Lister(),
	}
	s := store.NewMultiClusterCache(ctl.dynamicClientForCluster, restMapper)

	ctl.store = s
	ctl.cacheProxy = newCacheProxy(s, restMapper, minRequestTimeout)
	ctl.clusterProxy = newClusterProxy(s, ctl.clusterLister, ctl.secretLister)
	ctl.karmadaProxy = kp

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

// Stop shutdown cache
func (ctl *Controller) Stop() {
	ctl.store.Stop()
}

// reconcile cache
func (ctl *Controller) reconcile(util.QueueKey) error {
	clusters, err := ctl.clusterLister.List(labels.Everything())
	if err != nil {
		return err
	}

	registries, err := ctl.registryLister.List(labels.Everything())
	if err != nil {
		return err
	}

	resourcesByClusters := make(map[string]map[schema.GroupVersionResource]struct{})
	for _, registry := range registries {
		matchedResources := make(map[schema.GroupVersionResource]struct{}, len(registry.Spec.ResourceSelectors))
		for _, selector := range registry.Spec.ResourceSelectors {
			gvr, err := restmapper.GetGroupVersionResource(ctl.restMapper, schema.FromAPIVersionAndKind(selector.APIVersion, selector.Kind))
			if err != nil {
				klog.Errorf("failed to get gvr: %v", err)
				continue
			}
			matchedResources[gvr] = struct{}{}
		}

		if len(matchedResources) == 0 {
			continue
		}

		for _, cluster := range clusters {
			if !util.ClusterMatches(cluster, registry.Spec.TargetCluster) {
				continue
			}
			if _, exist := resourcesByClusters[cluster.Name]; !exist {
				resourcesByClusters[cluster.Name] = make(map[schema.GroupVersionResource]struct{})
			}

			for resource := range matchedResources {
				resourcesByClusters[cluster.Name][resource] = struct{}{}
			}
		}
	}

	return ctl.store.UpdateCache(resourcesByClusters)
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
			h = http.HandlerFunc(func(delegate http.ResponseWriter, req *http.Request) {
				// Write error into delegate ResponseWriter, wrapped in metrics.InstrumentHandlerFunc, so metrics can record this error.
				gv := schema.GroupVersion{
					Group:   requestInfo.APIGroup,
					Version: requestInfo.Verb,
				}
				responsewriters.ErrorNegotiated(err, ctl.negotiatedSerializer, gv, delegate, req)
			})
		}
		h = metrics.InstrumentHandlerFunc(requestInfo.Verb, requestInfo.APIGroup, requestInfo.APIVersion, requestInfo.Resource, requestInfo.Subresource,
			"", "karmada-search", false, "", h.ServeHTTP)
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
		return ctl.cacheProxy.connect(ctx)
	}

	// 3. The remaining requests are:
	// - writing resources.
	// - or subresource requests, e.g. `pods/log`
	// We firstly find the resource from cache, and get the located cluster. Then redirect the request to the cluster.
	return ctl.clusterProxy.connect(ctx, requestInfo, gvr, path, responder)
}

// TODO: reuse with karmada/pkg/util/membercluster_client.go
func (ctl *Controller) dynamicClientForCluster(clusterName string) (dynamic.Interface, error) {
	cluster, err := ctl.clusterLister.Get(clusterName)
	if err != nil {
		return nil, err
	}

	apiEndpoint := cluster.Spec.APIEndpoint
	if apiEndpoint == "" {
		return nil, fmt.Errorf("the api endpoint of cluster %s is empty", clusterName)
	}

	if cluster.Spec.SecretRef == nil {
		return nil, fmt.Errorf("cluster %s does not have a secret", clusterName)
	}

	secret, err := ctl.secretLister.Secrets(cluster.Spec.SecretRef.Namespace).Get(cluster.Spec.SecretRef.Name)
	if err != nil {
		return nil, err
	}

	token, tokenFound := secret.Data[clusterv1alpha1.SecretTokenKey]
	if !tokenFound || len(token) == 0 {
		return nil, fmt.Errorf("the secret for cluster %s is missing a non-empty value for %q", clusterName, clusterv1alpha1.SecretTokenKey)
	}

	clusterConfig, err := clientcmd.BuildConfigFromFlags(apiEndpoint, "")
	if err != nil {
		return nil, err
	}

	clusterConfig.BearerToken = string(token)

	if cluster.Spec.InsecureSkipTLSVerification {
		clusterConfig.TLSClientConfig.Insecure = true
	} else {
		clusterConfig.CAData = secret.Data[clusterv1alpha1.SecretCADataKey]
	}

	if cluster.Spec.ProxyURL != "" {
		proxy, err := url.Parse(cluster.Spec.ProxyURL)
		if err != nil {
			klog.Errorf("parse proxy error. %v", err)
			return nil, err
		}
		clusterConfig.Proxy = http.ProxyURL(proxy)
	}

	return dynamic.NewForConfig(clusterConfig)
}
