package options

import (
	"github.com/spf13/pflag"

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
	fs.IntVar(&o.SecurePort, "secure-port", defaultHealthzPort, "The secure port on which to serve HTTPS.")
	fs.Float32Var(&o.ClusterAPIQPS, "kube-api-qps", 20.0, "QPS to use while talking with apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	fs.IntVar(&o.ClusterAPIBurst, "kube-api-burst", 30, "Burst to use while talking with apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	fs.IntVar(&o.Parallelism, "parallelism", o.Parallelism, "Parallelism defines the amount of parallelism in algorithms for estimating. Must be greater than 0. Defaults to 16.")
	o.ProfileOpts.AddFlags(fs)
}
