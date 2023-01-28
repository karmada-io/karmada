package options

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/features"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"

	"github.com/karmada-io/karmada/pkg/aggregatedapiserver"
	clusterscheme "github.com/karmada-io/karmada/pkg/apis/cluster/scheme"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	pkgfeatures "github.com/karmada-io/karmada/pkg/features"
	clientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informers "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/util/lifted"
	"github.com/karmada-io/karmada/pkg/version"
)

const defaultEtcdPathPrefix = "/registry"

// Options contains everything necessary to create and run aggregated-apiserver.
type Options struct {
	RecommendedOptions    *genericoptions.RecommendedOptions
	SharedInformerFactory informers.SharedInformerFactory

	// KubeAPIQPS is the QPS to use while talking with karmada-apiserver.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with karmada-apiserver.
	KubeAPIBurst int

	ProfileOpts profileflag.Options
}

// NewOptions returns a new Options.
func NewOptions() *Options {
	o := &Options{
		RecommendedOptions: genericoptions.NewRecommendedOptions(
			defaultEtcdPathPrefix,
			clusterscheme.Codecs.LegacyCodec(clusterv1alpha1.SchemeGroupVersion)),
	}
	o.RecommendedOptions.Etcd.StorageConfig.EncodeVersioner = runtime.NewMultiGroupVersioner(clusterv1alpha1.SchemeGroupVersion, schema.GroupKind{Group: clusterv1alpha1.GroupName})
	return o
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	o.RecommendedOptions.AddFlags(flags)
	flags.Lookup("kubeconfig").Usage = "Path to karmada control plane kubeconfig file."

	flags.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with karmada-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	flags.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with karmada-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	_ = utilfeature.DefaultMutableFeatureGate.Add(pkgfeatures.DefaultFeatureGates)
	utilfeature.DefaultMutableFeatureGate.AddFlag(flags)
	o.ProfileOpts.AddFlags(flags)
}

// Complete fills in fields required to have valid data.
func (o *Options) Complete() error {
	return nil
}

// Run runs the aggregated-apiserver with options. This should never exit.
func (o *Options) Run(ctx context.Context) error {
	klog.Infof("karmada-aggregated-apiserver version: %s", version.Get())

	profileflag.ListenAndServe(o.ProfileOpts)

	config, err := o.Config()
	if err != nil {
		return err
	}

	restConfig := config.GenericConfig.ClientConfig
	restConfig.QPS, restConfig.Burst = o.KubeAPIQPS, o.KubeAPIBurst
	kubeClientSet := kubernetes.NewForConfigOrDie(restConfig)

	server, err := config.Complete().New(kubeClientSet)
	if err != nil {
		return err
	}

	server.GenericAPIServer.AddPostStartHookOrDie("start-aggregated-server-informers", func(context genericapiserver.PostStartHookContext) error {
		config.GenericConfig.SharedInformerFactory.Start(context.StopCh)
		o.SharedInformerFactory.Start(context.StopCh)
		return nil
	})

	return server.GenericAPIServer.PrepareRun().Run(ctx.Done())
}

// Config returns config for the api server given Options
func (o *Options) Config() (*aggregatedapiserver.Config, error) {
	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{netutils.ParseIPSloppy("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	o.RecommendedOptions.Etcd.StorageConfig.Paging = utilfeature.DefaultFeatureGate.Enabled(features.APIListChunking)

	o.RecommendedOptions.ExtraAdmissionInitializers = func(c *genericapiserver.RecommendedConfig) ([]admission.PluginInitializer, error) {
		client, err := clientset.NewForConfig(c.LoopbackClientConfig)
		if err != nil {
			return nil, err
		}
		informerFactory := informers.NewSharedInformerFactory(client, c.LoopbackClientConfig.Timeout)
		o.SharedInformerFactory = informerFactory
		return []admission.PluginInitializer{}, nil
	}
	o.RecommendedOptions.Features = &genericoptions.FeatureOptions{EnableProfiling: false}

	serverConfig := genericapiserver.NewRecommendedConfig(clusterscheme.Codecs)
	serverConfig.LongRunningFunc = customLongRunningRequestCheck(sets.NewString("watch", "proxy"),
		sets.NewString("attach", "exec", "proxy", "log", "portforward"))
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(clusterscheme.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "Karmada"
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	config := &aggregatedapiserver.Config{
		GenericConfig: serverConfig,
		ExtraConfig:   aggregatedapiserver.ExtraConfig{},
	}
	return config, nil
}

func customLongRunningRequestCheck(longRunningVerbs, longRunningSubresources sets.String) apirequest.LongRunningRequestCheck {
	return func(r *http.Request, requestInfo *apirequest.RequestInfo) bool {
		reqClone := r.Clone(context.Background())
		p := reqClone.URL.Path
		currentParts := lifted.SplitPath(p)
		if isClusterProxy(currentParts) {
			currentParts = currentParts[6:]
			reqClone.URL.Path = "/" + strings.Join(currentParts, "/")
			requestInfo = lifted.NewRequestInfo(reqClone)
		}

		return genericfilters.BasicLongRunningRequestCheck(longRunningVerbs, longRunningSubresources)(r, requestInfo)
	}
}

func isClusterProxy(pathParts []string) bool {
	// cluster/proxy url path format: /apis/cluster.karmada.io/v1alpha1/clusters/{cluster}/proxy/...
	return len(pathParts) >= 6 && pathParts[1] == "cluster.karmada.io" && pathParts[5] == "proxy"
}
