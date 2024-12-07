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

	"github.com/spf13/pflag"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/cmd/options"
	"sigs.k8s.io/metrics-server/pkg/api"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
	"github.com/karmada-io/karmada/pkg/metricsadapter"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/version"
)

// Options contains everything necessary to create and run metrics-adapter.
type Options struct {
	CustomMetricsAdapterServerOptions *options.CustomMetricsAdapterServerOptions

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
	fs.Float32Var(&o.ClusterAPIQPS, "cluster-api-qps", 40.0, "QPS to use while talking with cluster kube-apiserver.")
	fs.IntVar(&o.ClusterAPIBurst, "cluster-api-burst", 60, "Burst to use while talking with cluster kube-apiserver.")
	fs.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with karmada-apiserver.")
	fs.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with karmada-apiserver.")
	fs.StringVar(&o.KubeConfig, "kubeconfig", o.KubeConfig, "Path to karmada control plane kubeconfig file.")
}

// Config returns config for the metrics-adapter server given Options
func (o *Options) Config(stopCh <-chan struct{}) (*metricsadapter.MetricsServer, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", o.KubeConfig)
	if err != nil {
		klog.Errorf("Unable to build restConfig: %v", err)
		return nil, err
	}
	restConfig.QPS, restConfig.Burst = o.KubeAPIQPS, o.KubeAPIBurst

	karmadaClient := karmadaclientset.NewForConfigOrDie(restConfig)
	factory := informerfactory.NewSharedInformerFactory(karmadaClient, 0)
	kubeClient := kubernetes.NewForConfigOrDie(restConfig)
	kubeFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	metricsController := metricsadapter.NewMetricsController(stopCh, restConfig, factory, kubeFactory, &util.ClientOption{QPS: o.ClusterAPIQPS, Burst: o.ClusterAPIBurst})
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

	profileflag.ListenAndServe(o.ProfileOpts)

	stopCh := ctx.Done()
	metricsServer, err := o.Config(stopCh)
	if err != nil {
		return err
	}

	return metricsServer.StartServer(ctx)
}
