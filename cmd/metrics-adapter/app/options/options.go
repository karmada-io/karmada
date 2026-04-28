/*
Copyright 2023 The Karmada Authors.

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

package options

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/spf13/pflag"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/cmd/options"
	"sigs.k8s.io/metrics-server/pkg/api"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
	versionmetrics "github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/metricsadapter"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/version"
)

const (
	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	// References:
	// - https://en.wikipedia.org/wiki/Slowloris_(computer_security)
	ReadHeaderTimeout = 32 * time.Second
	// WriteTimeout is the amount of time allowed to write the
	// request data.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	WriteTimeout = 5 * time.Minute
	// ReadTimeout is the amount of time allowed to read
	// response data.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	ReadTimeout = 5 * time.Minute
)

// Options contains everything necessary to create and run metrics-adapter.
type Options struct {
	CustomMetricsAdapterServerOptions *options.CustomMetricsAdapterServerOptions

	// MetricsBindAddress is the TCP address that the server should bind to
	// for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	// Defaults to ":8080".
	MetricsBindAddress string

	KubeConfig string
	// ClusterAPIQPS is the QPS to use while talking with cluster kube-apiserver.
	ClusterAPIQPS float32
	// ClusterAPIBurst is the burst to allow while talking with cluster kube-apiserver.
	ClusterAPIBurst int
	// KubeAPIQPS is the QPS to use while talking with karmada-apiserver.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with karmada-apiserver.
	KubeAPIBurst int
	ProfileOpts  profileflag.Options
}

// NewOptions builds a default metrics-adapter options.
func NewOptions() *Options {
	o := &Options{
		CustomMetricsAdapterServerOptions: options.NewCustomMetricsAdapterServerOptions(),
	}

	return o
}

// Complete fills in fields required to have valid data.
func (o *Options) Complete() error {
	return nil
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	o.CustomMetricsAdapterServerOptions.AddFlags(fs)
	o.ProfileOpts.AddFlags(fs)
	fs.StringVar(&o.MetricsBindAddress, "metrics-bind-address", ":8080", "The TCP address that the server should bind to for serving prometheus metrics(e.g. 127.0.0.1:8080, :8080). It can be set to \"0\" to disable the metrics serving. Defaults to 0.0.0.0:8080.")
	fs.Float32Var(&o.ClusterAPIQPS, "cluster-api-qps", 40.0, "QPS to use while talking with cluster kube-apiserver.")
	fs.IntVar(&o.ClusterAPIBurst, "cluster-api-burst", 60, "Burst to use while talking with cluster kube-apiserver.")
	fs.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with karmada-apiserver.")
	fs.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with karmada-apiserver.")
	fs.StringVar(&o.KubeConfig, "kubeconfig", o.KubeConfig, "Path to karmada control plane kubeconfig file.")
}

// Config returns config for the metrics-adapter server given Options
func (o *Options) Config(ctx context.Context) (*metricsadapter.MetricsServer, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", o.KubeConfig)
	if err != nil {
		klog.Errorf("Unable to build restConfig: %v", err)
		return nil, err
	}
	restConfig.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(o.KubeAPIQPS, o.KubeAPIBurst)

	karmadaClient := karmadaclientset.NewForConfigOrDie(restConfig)
	factory := informerfactory.NewSharedInformerFactory(karmadaClient, 0)
	kubeClient := kubernetes.NewForConfigOrDie(restConfig)
	kubeFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	limiterGetter := util.GetClusterRateLimiterGetter().SetDefaultLimits(o.ClusterAPIQPS, o.ClusterAPIBurst)
	metricsController := metricsadapter.NewMetricsController(ctx, restConfig, factory, kubeFactory, &util.ClientOption{RateLimiterGetter: limiterGetter.GetRateLimiter})
	metricsAdapter := metricsadapter.NewMetricsAdapter(metricsController, o.CustomMetricsAdapterServerOptions)
	metricsAdapter.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(api.Scheme))
	metricsAdapter.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(generatedopenapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(api.Scheme))
	metricsAdapter.OpenAPIConfig.Info.Title = names.KarmadaMetricsAdapterComponentName
	metricsAdapter.OpenAPIConfig.Info.Version = "1.0.0"

	// Explicitly specify the remote kubeconfig file here to solve the issue that metrics adapter requires to build
	// informer against karmada-apiserver started from custom-metrics-apiserver@v1.29.0.
	// See https://github.com/karmada-io/karmada/pull/4884#issuecomment-2095109485 for more details.
	//
	// For karmada-metrics-adapter, the kubeconfig file of karmada-apiserver is "remote", not the in-cluster one.
	metricsAdapter.RemoteKubeConfigFile = o.KubeConfig

	server, err := metricsAdapter.Server()
	if err != nil {
		klog.Errorf("Unable to construct metrics adapter: %v", err)
		return nil, err
	}

	err = server.GenericAPIServer.AddPostStartHook("start-karmada-informers", func(context genericapiserver.PostStartHookContext) error {
		kubeFactory.Core().V1().Secrets().Informer()
		kubeFactory.Start(context.Done())
		kubeFactory.WaitForCacheSync(context.Done())
		factory.Start(context.Done())
		return nil
	})
	if err != nil {
		klog.Errorf("Unable to add post hook: %v", err)
		return nil, err
	}

	if err := api.Install(metricsAdapter, metricsAdapter.PodLister, metricsAdapter.NodeLister, server.GenericAPIServer, nil); err != nil {
		klog.Errorf("Unable to install resource metrics adapter: %v", err)
		return nil, err
	}

	return metricsadapter.NewMetricsServer(metricsController, metricsAdapter), nil
}

// Run runs the metrics-adapter with options. This should never exit.
func (o *Options) Run(ctx context.Context) error {
	klog.Infof("karmada-metrics-adapter version: %s", version.Get())
	legacyregistry.RawMustRegister(versionmetrics.NewBuildInfoCollector())
	if o.MetricsBindAddress != "0" {
		go serveMetrics(o.MetricsBindAddress)
	}

	profileflag.ListenAndServe(o.ProfileOpts)

	metricsServer, err := o.Config(ctx)
	if err != nil {
		return err
	}

	return metricsServer.StartServer(ctx)
}

func serveMetrics(address string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", metricsHandler())
	serveHTTP(address, mux, "metrics")
}

func metricsHandler() http.Handler {
	return metrics.HandlerFor(legacyregistry.DefaultGatherer, metrics.HandlerOpts{
		ErrorHandling: metrics.HTTPErrorOnError,
	})
}

func serveHTTP(address string, handler http.Handler, name string) {
	httpServer := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: ReadHeaderTimeout,
		WriteTimeout:      WriteTimeout,
		ReadTimeout:       ReadTimeout,
	}

	klog.Infof("Starting %s server on %s", name, address)
	if err := httpServer.ListenAndServe(); err != nil {
		klog.Errorf("Failed to serve %s on %s: %v", name, address, err)
		os.Exit(1)
	}
}
