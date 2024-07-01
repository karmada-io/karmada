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
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	searchscheme "github.com/karmada-io/karmada/pkg/apis/search/scheme"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
)

const defaultEtcdPathPrefix = "/registry"

// Options contains command line parameters for karmada-search.
type Options struct {
	RecommendedOptions *genericoptions.RecommendedOptions

	// KubeAPIQPS is the QPS to use while talking with karmada-search.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with karmada-search.
	KubeAPIBurst int
	// MemberClusterKubeAPIQPS is the QPS to use while talking to member cluster.
	MemberClusterKubeAPIQPS float32
	// MemberClusterKubeAPIBurst is the burst to allow while talking to member cluster.
	MemberClusterKubeAPIBurst int

	ProfileOpts profileflag.Options

	DisableSearch bool
	DisableProxy  bool
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
	return o
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	o.RecommendedOptions.AddFlags(flags)
	flags.Lookup("kubeconfig").Usage = "Path to karmada control plane kubeconfig file."

	flags.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with karmada-apiserver.")
	flags.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with karmada-apiserver.")
	flags.Float32Var(&o.MemberClusterKubeAPIQPS, "member-client-kube-api-qps", 40.0, "QPS to use while talking to member client.")
	flags.IntVar(&o.MemberClusterKubeAPIBurst, "member-client-kube-api-burst", 60, "Burst to use while talking to member client.")
	flags.BoolVar(&o.DisableSearch, "disable-search", false, "Disable search feature that would save memory usage significantly.")
	flags.BoolVar(&o.DisableProxy, "disable-proxy", false, "Disable proxy feature that would save memory usage significantly.")

	utilfeature.DefaultMutableFeatureGate.AddFlag(flags)
	o.ProfileOpts.AddFlags(flags)
}

// Complete fills in fields required to have valid data.
func (o *Options) Complete() error {
	return nil
}
