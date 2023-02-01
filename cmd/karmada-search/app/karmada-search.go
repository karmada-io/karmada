package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/endpoints/request"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/karmada-io/karmada/cmd/karmada-search/app/options"
	searchscheme "github.com/karmada-io/karmada/pkg/apis/search/scheme"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
	"github.com/karmada-io/karmada/pkg/search"
	"github.com/karmada-io/karmada/pkg/search/proxy"
	"github.com/karmada-io/karmada/pkg/search/proxy/framework/runtime"
	"github.com/karmada-io/karmada/pkg/sharedcli"
	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/util/lifted"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

// Option configures a framework.Registry.
type Option func(*runtime.Registry)

// NewKarmadaSearchCommand creates a *cobra.Command object with default parameters
func NewKarmadaSearchCommand(ctx context.Context, registryOptions ...Option) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use: "karmada-search",
		Long: `The karmada-search starts an aggregated server. It provides 
capabilities such as global search and resource proxy in a multi-cloud environment.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := run(ctx, opts, registryOptions...); err != nil {
				return err
			}
			return nil
		},
	}

	fss := cliflag.NamedFlagSets{}

	genericFlagSet := fss.FlagSet("generic")
	opts.AddFlags(genericFlagSet)

	// Set klog flags
	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)

	cmd.AddCommand(sharedcommand.NewCmdVersion("karmada-search"))
	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	return cmd
}

// WithPlugin creates an Option based on plugin factory.
// Please don't remove this function: it is used to register out-of-tree plugins,
// hence there are no references to it from the karmada-search code base.
func WithPlugin(factory runtime.PluginFactory) Option {
	return func(registry *runtime.Registry) {
		registry.Register(factory)
	}
}

// `run` runs the karmada-search with options. This should never exit.
func run(ctx context.Context, o *options.Options, registryOptions ...Option) error {
	klog.Infof("karmada-search version: %s", version.Get())

	profileflag.ListenAndServe(o.ProfileOpts)

	config, err := config(o, registryOptions...)
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
		config.ExtraConfig.KarmadaSharedInformerFactory.Start(context.StopCh)
		return nil
	})

	if config.ExtraConfig.Controller != nil {
		server.GenericAPIServer.AddPostStartHookOrDie("start-karmada-search-controller", func(context genericapiserver.PostStartHookContext) error {
			// start ResourceRegistry controller
			config.ExtraConfig.Controller.Start(context.StopCh)
			return nil
		})
	}

	if config.ExtraConfig.ProxyController != nil {
		server.GenericAPIServer.AddPostStartHookOrDie("start-karmada-proxy-controller", func(context genericapiserver.PostStartHookContext) error {
			config.ExtraConfig.ProxyController.Start(context.StopCh)
			return nil
		})

		server.GenericAPIServer.AddPreShutdownHookOrDie("stop-karmada-proxy-controller", func() error {
			config.ExtraConfig.ProxyController.Stop()
			return nil
		})
	}

	return server.GenericAPIServer.PrepareRun().Run(ctx.Done())
}

// `config` returns config for the api server given Options
func config(o *options.Options, outOfTreeRegistryOptions ...Option) (*search.Config, error) {
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

	restMapper, err := apiutil.NewDynamicRESTMapper(serverConfig.ClientConfig)
	if err != nil {
		klog.Errorf("Failed to create REST mapper: %v", err)
		return nil, err
	}

	karmadaClient := karmadaclientset.NewForConfigOrDie(serverConfig.ClientConfig)
	factory := informerfactory.NewSharedInformerFactory(karmadaClient, 0)

	var ctl *search.Controller
	if !o.DisableSearch {
		ctl, err = search.NewController(serverConfig.ClientConfig, factory, restMapper)
		if err != nil {
			return nil, err
		}
	}

	var proxyCtl *proxy.Controller
	if !o.DisableProxy {
		outOfTreeRegistry := make(runtime.Registry, 0, len(outOfTreeRegistryOptions))
		for _, option := range outOfTreeRegistryOptions {
			option(&outOfTreeRegistry)
		}

		proxyCtl, err = proxy.NewController(proxy.NewControllerOption{
			RestConfig:        serverConfig.ClientConfig,
			RestMapper:        restMapper,
			KubeFactory:       serverConfig.SharedInformerFactory,
			KarmadaFactory:    factory,
			MinRequestTimeout: time.Second * time.Duration(serverConfig.Config.MinRequestTimeout),
			OutOfTreeRegistry: outOfTreeRegistry,
		})

		if err != nil {
			return nil, err
		}
	}

	config := &search.Config{
		GenericConfig: serverConfig,
		ExtraConfig: search.ExtraConfig{
			KarmadaSharedInformerFactory: factory,
			Controller:                   ctl,
			ProxyController:              proxyCtl,
		},
	}
	return config, nil
}

// disable `deprecation` check until the underlying genericfilters.BasicLongRunningRequestCheck starts using generic Set.
//
//nolint:staticcheck
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
