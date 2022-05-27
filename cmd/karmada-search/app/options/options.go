package options

import (
	"context"
	"fmt"
	"net"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/features"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	netutils "k8s.io/utils/net"

	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
	"github.com/karmada-io/karmada/pkg/search"
)

const defaultEtcdPathPrefix = "/registry"

// Options contains everything necessary to create and run karmada-search.
type Options struct {
	RecommendedOptions *genericoptions.RecommendedOptions

	// KubeAPIQPS is the QPS to use while talking with karmada-search.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with karmada-search.
	KubeAPIBurst int
}

// NewOptions returns a new Options.
func NewOptions() *Options {
	o := &Options{
		RecommendedOptions: genericoptions.NewRecommendedOptions(
			defaultEtcdPathPrefix,
			search.Codecs.LegacyCodec(searchv1alpha1.SchemeGroupVersion)),
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
}

// Complete fills in fields required to have valid data.
func (o *Options) Complete() error {
	return nil
}

// Validate validates Options.
func (o *Options) Validate() error {
	var errs []error
	errs = append(errs, o.RecommendedOptions.Validate()...)
	return utilerrors.NewAggregate(errs)
}

// Run runs the aggregated-apiserver with options. This should never exit.
func (o *Options) Run(ctx context.Context) error {
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

	server.GenericAPIServer.AddPostStartHookOrDie("start-karmada-search-controller", func(context genericapiserver.PostStartHookContext) error {
		// start ResourceRegistry controller
		config.Controller.Start(context.StopCh)
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

	serverConfig := genericapiserver.NewRecommendedConfig(search.Codecs)
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(search.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "karmada-search"
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	serverConfig.ClientConfig.QPS = o.KubeAPIQPS
	serverConfig.ClientConfig.Burst = o.KubeAPIBurst
	ctl, err := search.NewController(serverConfig.ClientConfig, search.CachedResourceHandler())
	if err != nil {
		return nil, err
	}

	config := &search.Config{
		GenericConfig: serverConfig,
		Controller:    ctl,
	}
	return config, nil
}
