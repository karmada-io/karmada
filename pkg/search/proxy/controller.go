package proxy

import (
	"context"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
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
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	clusterlisters "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	searchlisters "github.com/karmada-io/karmada/pkg/generated/listers/search/v1alpha1"
	"github.com/karmada-io/karmada/pkg/search/proxy/framework"
	"github.com/karmada-io/karmada/pkg/search/proxy/framework/plugins"
	pluginruntime "github.com/karmada-io/karmada/pkg/search/proxy/framework/runtime"
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
	store          store.Store

	proxy framework.Proxy
}

// NewControllerOption is the Option for NewController().
type NewControllerOption struct {
	RestConfig *restclient.Config
	RestMapper meta.RESTMapper

	KubeFactory    informers.SharedInformerFactory
	KarmadaFactory informerfactory.SharedInformerFactory

	MinRequestTimeout time.Duration

	OutOfTreeRegistry pluginruntime.Registry
}

// NewController create a controller for proxy
func NewController(option NewControllerOption) (*Controller, error) {
	secretLister := option.KubeFactory.Core().V1().Secrets().Lister()
	clusterLister := option.KarmadaFactory.Cluster().V1alpha1().Clusters().Lister()

	clientFactory := dynamicClientForClusterFunc(clusterLister, secretLister)
	multiClusterStore := store.NewMultiClusterCache(clientFactory, option.RestMapper)

	allPlugins, err := newPlugins(option, multiClusterStore)
	if err != nil {
		return nil, err
	}

	proxy := pluginruntime.NewFramework(allPlugins)

	ctl := &Controller{
		restMapper:           option.RestMapper,
		negotiatedSerializer: scheme.Codecs.WithoutConversion(),
		secretLister:         secretLister,
		clusterLister:        clusterLister,
		registryLister:       option.KarmadaFactory.Search().V1alpha1().ResourceRegistries().Lister(),
		store:                multiClusterStore,
		proxy:                proxy,
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

	option.KarmadaFactory.Cluster().V1alpha1().Clusters().Informer().AddEventHandler(resourceEventHandler)
	option.KarmadaFactory.Search().V1alpha1().ResourceRegistries().Informer().AddEventHandler(resourceEventHandler)

	return ctl, nil
}

func newPlugins(option NewControllerOption, clusterStore store.Store) ([]framework.Plugin, error) {
	pluginDependency := pluginruntime.PluginDependency{
		RestConfig:        option.RestConfig,
		RestMapper:        option.RestMapper,
		KubeFactory:       option.KubeFactory,
		KarmadaFactory:    option.KarmadaFactory,
		MinRequestTimeout: option.MinRequestTimeout,
		Store:             clusterStore,
	}

	registry := plugins.NewInTreeRegistry()
	registry.Merge(option.OutOfTreeRegistry)

	allPlugins := make([]framework.Plugin, 0, len(registry))
	for _, pluginFactory := range registry {
		plugin, err := pluginFactory(pluginDependency)
		if err != nil {
			return nil, err
		}

		allPlugins = append(allPlugins, plugin)
	}

	return allPlugins, nil
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

type errorHTTPHandler struct {
	requestInfo          *request.RequestInfo
	err                  error
	negotiatedSerializer runtime.NegotiatedSerializer
}

func (handler *errorHTTPHandler) ServeHTTP(delegate http.ResponseWriter, req *http.Request) {
	// Write error into delegate ResponseWriter, wrapped in metrics.InstrumentHandlerFunc, so metrics can record this error.
	gv := schema.GroupVersion{
		Group:   handler.requestInfo.APIGroup,
		Version: handler.requestInfo.Verb,
	}
	responsewriters.ErrorNegotiated(handler.err, handler.negotiatedSerializer, gv, delegate, req)
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

		gvr := schema.GroupVersionResource{
			Group:    requestInfo.APIGroup,
			Version:  requestInfo.APIVersion,
			Resource: requestInfo.Resource,
		}

		h, err := ctl.proxy.Connect(newCtx, framework.ProxyRequest{
			RequestInfo:          requestInfo,
			GroupVersionResource: gvr,
			ProxyPath:            proxyPath,
			Responder:            responder,
			HTTPReq:              newReq,
		})

		if err != nil {
			h = &errorHTTPHandler{
				requestInfo:          requestInfo,
				err:                  err,
				negotiatedSerializer: ctl.negotiatedSerializer,
			}
		}

		h = metrics.InstrumentHandlerFunc(requestInfo.Verb, requestInfo.APIGroup, requestInfo.APIVersion, requestInfo.Resource, requestInfo.Subresource,
			"", "karmada-search", false, "", h.ServeHTTP)
		h.ServeHTTP(rw, newReq)
	}), nil
}

func dynamicClientForClusterFunc(clusterLister clusterlisters.ClusterLister,
	secretLister listcorev1.SecretLister) func(string) (dynamic.Interface, error) {
	clusterGetter := func(cluster string) (*clusterv1alpha1.Cluster, error) {
		return clusterLister.Get(cluster)
	}
	secretGetter := func(namespace string, name string) (*corev1.Secret, error) {
		return secretLister.Secrets(namespace).Get(name)
	}

	return func(clusterName string) (dynamic.Interface, error) {
		clusterConfig, err := util.BuildClusterConfig(clusterName, clusterGetter, secretGetter)
		if err != nil {
			return nil, err
		}
		return dynamic.NewForConfig(clusterConfig)
	}
}
