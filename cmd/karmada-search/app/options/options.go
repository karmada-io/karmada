package options

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/features"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"

	searchscheme "github.com/karmada-io/karmada/pkg/apis/search/scheme"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
	"github.com/karmada-io/karmada/pkg/search"
	"github.com/karmada-io/karmada/pkg/search/proxy"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/lifted"
	"github.com/karmada-io/karmada/pkg/version"
)

const defaultEtcdPathPrefix = "/registry"

// Options contains everything necessary to create and run karmada-search.
type Options struct {
	RecommendedOptions *genericoptions.RecommendedOptions

	// KubeAPIQPS is the QPS to use while talking with karmada-search.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with karmada-search.
	KubeAPIBurst int

	ProfileOpts profileflag.Options
}

// NewOptions returns a new Options.
func NewOptions() *Options {
	o := &Options{
		RecommendedOptions: genericoptions.NewRecommendedOptions(
			defaultEtcdPathPrefix,
			searchscheme.Codecs.LegacyCodec(searchv1alpha1.SchemeGroupVersion)),
	}
	o.RecommendedOptions.Etcd.StorageConfig.EncodeVersioner = runtime.NewMultiGroupVersioner(searchv1alpha1.SchemeGroupVersion,
		schema.GroupKind{Group: searchv1alpha1.GroupName})
	o.RecommendedOptions.Etcd.StorageConfig.Paging = utilfeature.DefaultFeatureGate.Enabled(features.APIListChunking)
	return o
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	o.RecommendedOptions.AddFlags(flags)
	flags.Lookup("kubeconfig").Usage = "Path to karmada control plane kubeconfig file."

	flags.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with karmada-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	flags.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with karmada-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")

	utilfeature.DefaultMutableFeatureGate.AddFlag(flags)
	o.ProfileOpts.AddFlags(flags)
}

// Complete fills in fields required to have valid data.
func (o *Options) Complete() error {
	return nil
}

// Run runs the aggregated-apiserver with options. This should never exit.
func (o *Options) Run(ctx context.Context) error {
	klog.Infof("karmada-search version: %s", version.Get())

	profileflag.ListenAndServe(o.ProfileOpts)

	config, err := o.Config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	server.GenericAPIServer.AddPostStartHookOrDie("start-karmada-search-informers", func(context genericapiserver.PostStartHookContext) error {
		config.GenericConfig.SharedInformerFactory.Start(context.StopCh)
		return nil
	})

	server.GenericAPIServer.AddPostStartHookOrDie("start-karmada-informers", func(context genericapiserver.PostStartHookContext) error {
		config.KarmadaSharedInformerFactory.Start(context.StopCh)
		return nil
	})

	server.GenericAPIServer.AddPostStartHookOrDie("start-karmada-search-controller", func(context genericapiserver.PostStartHookContext) error {
		// start ResourceRegistry controller
		config.Controller.Start(context.StopCh)
		return nil
	})

	server.GenericAPIServer.AddPostStartHookOrDie("start-karmada-proxy-controller", func(context genericapiserver.PostStartHookContext) error {
		// start ResourceRegistry controller
		config.ProxyController.Start(context.StopCh)
		return nil
	})

	return server.GenericAPIServer.PrepareRun().Run(ctx.Done())
}

// Config returns config for the api server given Options
func (o *Options) Config() (*search.Config, error) {
	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{netutils.ParseIPSloppy("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	o.RecommendedOptions.Features = &genericoptions.FeatureOptions{EnableProfiling: false}

	serverConfig := genericapiserver.NewRecommendedConfig(searchscheme.Codecs)
	serverConfig.LongRunningFunc = customLongRunningRequestCheck(
		sets.NewString("watch", "proxy"),
		sets.NewString("attach", "exec", "proxy", "log", "portforward"))
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(searchscheme.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "karmada-search"
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	serverConfig.ClientConfig.QPS = o.KubeAPIQPS
	serverConfig.ClientConfig.Burst = o.KubeAPIBurst
	ctl, err := search.NewController(serverConfig.ClientConfig)
	if err != nil {
		return nil, err
	}

	karmadaClient := karmadaclientset.NewForConfigOrDie(serverConfig.ClientConfig)
	factory := informerfactory.NewSharedInformerFactory(karmadaClient, 0)

	proxyCtl, err := proxy.NewController(serverConfig.ClientConfig, genericmanager.GetInstance(), serverConfig.SharedInformerFactory, factory)
	if err != nil {
		return nil, err
	}

	config := &search.Config{
		GenericConfig:                serverConfig,
		Controller:                   ctl,
		ProxyController:              proxyCtl,
		KarmadaSharedInformerFactory: factory,
	}
	return config, nil
}

func customLongRunningRequestCheck(longRunningVerbs, longRunningSubresources sets.String) request.LongRunningRequestCheck {
	return func(r *http.Request, requestInfo *request.RequestInfo) bool {
		if requestInfo.APIGroup == "search.karmada.io" && requestInfo.Resource == "proxying" {
			reqClone := r.Clone(context.TODO())
			// requestInfo.Parts is like [proxying foo proxy api v1 nodes]
			reqClone.URL.Path = "/" + path.Join(requestInfo.Parts[3:]...)
			requestInfo = lifted.NewRequestInfo(reqClone)
		}
		return genericfilters.BasicLongRunningRequestCheck(longRunningVerbs, longRunningSubresources)(r, requestInfo)
	}
}
