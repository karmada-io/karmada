/*
Copyright 2021 The Karmada Authors.

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

	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
)

const (
	defaultBindAddress = "0.0.0.0"
	defaultServerPort  = 10352
	defaultHealthzPort = 10351
)

// Options contains everything necessary to create and run scheduler-estimator.
type Options struct {
	KubeConfig  string
	Master      string
	ClusterName string
	// BindAddress is the IP address on which to listen for the --secure-port port.
	BindAddress string
	// SecurePort is the port that the server serves at.
	SecurePort int
	// ServerPort is the port that the server gRPC serves at.
	ServerPort int
	// InsecureSkipGrpcClientVerify controls whether verifies the grpc client's certificate chain and host name.
	InsecureSkipGrpcClientVerify bool
	// GrpcAuthCertFile SSL certification file used for grpc SSL/TLS connections.
	GrpcAuthCertFile string
	// GrpcAuthKeyFile SSL key file used for grpc SSL/TLS connections.
	GrpcAuthKeyFile string
	// GrpcClientCaFile SSL Certificate Authority file used to verify grpc client certificates on incoming requests.
	GrpcClientCaFile string
	// ClusterAPIQPS is the QPS to use while talking with cluster kube-apiserver.
	ClusterAPIQPS float32
	// ClusterAPIBurst is the burst to allow while talking with cluster kube-apiserver.
	ClusterAPIBurst int
	// Parallelism defines the amount of parallelism in algorithms for estimating. Must be greater than 0. Defaults to 16.
	Parallelism int
	ProfileOpts profileflag.Options
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	return &Options{}
}

// AddFlags adds flags of estimator to the specified FlagSet
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}
	fs.StringVar(&o.KubeConfig, "kubeconfig", o.KubeConfig, "Path to member cluster's kubeconfig file.")
	fs.StringVar(&o.Master, "master", o.Master, "The address of the member Kubernetes API server. Overrides any value in KubeConfig. Only required if out-of-cluster.")
	fs.StringVar(&o.ClusterName, "cluster-name", o.ClusterName, "Name of member cluster that the estimator serves for.")
	fs.StringVar(&o.BindAddress, "bind-address", defaultBindAddress, "The IP address on which to listen for the --secure-port port.")
	fs.IntVar(&o.ServerPort, "server-port", defaultServerPort, "The secure port on which to serve gRPC.")
	fs.StringVar(&o.GrpcAuthCertFile, "grpc-auth-cert-file", "", "SSL certification file used for grpc SSL/TLS connections.")
	fs.StringVar(&o.GrpcAuthKeyFile, "grpc-auth-key-file", "", "SSL key file used for grpc SSL/TLS connections.")
	fs.BoolVar(&o.InsecureSkipGrpcClientVerify, "insecure-skip-grpc-client-verify", false, "If set to true, the estimator will not verify the grpc client's certificate chain and host name. When the relevant certificates are not configured, it will not take effect.")
	fs.StringVar(&o.GrpcClientCaFile, "grpc-client-ca-file", "", "SSL Certificate Authority file used to verify grpc client certificates on incoming requests if --client-cert-auth flag is set.")
	fs.IntVar(&o.SecurePort, "secure-port", defaultHealthzPort, "The secure port on which to serve HTTPS.")
	fs.Float32Var(&o.ClusterAPIQPS, "kube-api-qps", 20.0, "QPS to use while talking with apiserver.")
	fs.IntVar(&o.ClusterAPIBurst, "kube-api-burst", 30, "Burst to use while talking with apiserver.")
	fs.IntVar(&o.Parallelism, "parallelism", o.Parallelism, "Parallelism defines the amount of parallelism in algorithms for estimating. Must be greater than 0. Defaults to 16.")
	features.FeatureGate.AddFlag(fs)

	o.ProfileOpts.AddFlags(fs)
}
