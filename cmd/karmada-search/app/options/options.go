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

package options

import (
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/client-go/kubernetes"

	searchscheme "github.com/karmada-io/karmada/pkg/apis/search/scheme"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
)

const defaultEtcdPathPrefix = "/registry"

// Options contains command line parameters for karmada-search.
type Options struct {
	Etcd             *genericoptions.EtcdOptions
	SecureServing    *genericoptions.SecureServingOptionsWithLoopback
	Authentication   *genericoptions.DelegatingAuthenticationOptions
	Authorization    *genericoptions.DelegatingAuthorizationOptions
	Audit            *genericoptions.AuditOptions
	Features         *genericoptions.FeatureOptions
	CoreAPI          *genericoptions.CoreAPIOptions
	ServerRunOptions *genericoptions.ServerRunOptions

	// KubeAPIQPS is the QPS to use while talking with karmada-search.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with karmada-search.
	KubeAPIBurst int

	ProfileOpts profileflag.Options

	DisableSearch bool
	DisableProxy  bool
}

// NewOptions returns a new Options.
func NewOptions() *Options {
	o := &Options{
		Etcd:             genericoptions.NewEtcdOptions(storagebackend.NewDefaultConfig(defaultEtcdPathPrefix, searchscheme.Codecs.LegacyCodec(schema.GroupVersion{Group: searchv1alpha1.GroupVersion.Group, Version: searchv1alpha1.GroupVersion.Version}))),
		SecureServing:    genericoptions.NewSecureServingOptions().WithLoopback(),
		Authentication:   genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:    genericoptions.NewDelegatingAuthorizationOptions(),
		Audit:            genericoptions.NewAuditOptions(),
		Features:         genericoptions.NewFeatureOptions(),
		CoreAPI:          genericoptions.NewCoreAPIOptions(),
		ServerRunOptions: genericoptions.NewServerRunOptions(),
	}
	o.Etcd.StorageConfig.EncodeVersioner = runtime.NewMultiGroupVersioner(schema.GroupVersion{Group: searchv1alpha1.GroupVersion.Group, Version: searchv1alpha1.GroupVersion.Version},
		schema.GroupKind{Group: searchv1alpha1.GroupName})
	return o
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	o.Etcd.AddFlags(flags)
	o.SecureServing.AddFlags(flags)
	o.Authentication.AddFlags(flags)
	o.Authorization.AddFlags(flags)
	o.Audit.AddFlags(flags)
	o.Features.AddFlags(flags)
	o.CoreAPI.AddFlags(flags)
	o.ServerRunOptions.AddUniversalFlags(flags)

	flags.Lookup("kubeconfig").Usage = "Path to karmada control plane kubeconfig file."

	flags.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with karmada-apiserver.")
	flags.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with karmada-apiserver.")
	flags.BoolVar(&o.DisableSearch, "disable-search", false, "Disable search feature that would save memory usage significantly.")
	flags.BoolVar(&o.DisableProxy, "disable-proxy", false, "Disable proxy feature that would save memory usage significantly.")

	o.ProfileOpts.AddFlags(flags)
}

// ApplyTo adds Options to the server configuration.
func (o *Options) ApplyTo(config *genericapiserver.RecommendedConfig) error {
	if err := o.Etcd.ApplyTo(&config.Config); err != nil {
		return err
	}
	if err := o.SecureServing.ApplyTo(&config.Config.SecureServing, &config.Config.LoopbackClientConfig); err != nil {
		return err
	}
	if err := o.Authentication.ApplyTo(&config.Config.Authentication, config.SecureServing, config.OpenAPIConfig); err != nil {
		return err
	}
	if err := o.Authorization.ApplyTo(&config.Config.Authorization); err != nil {
		return err
	}
	if err := o.Audit.ApplyTo(&config.Config); err != nil {
		return err
	}
	if err := o.CoreAPI.ApplyTo(config); err != nil {
		return err
	}
	if err := o.ServerRunOptions.ApplyTo(&config.Config); err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(config.ClientConfig)
	if err != nil {
		return err
	}
	if err = o.Features.ApplyTo(&config.Config, kubeClient, config.SharedInformerFactory); err != nil {
		return err
	}
	return nil
}

// Complete fills in fields required to have valid data.
func (o *Options) Complete() error {
	return o.ServerRunOptions.Complete()
}
