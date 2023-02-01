package options

import (
	"github.com/spf13/pflag"

	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
)

const (
	defaultBindAddress   = "0.0.0.0"
	defaultPort          = 8443
	defaultCertDir       = "/tmp/k8s-webhook-server/serving-certs"
	defaultTLSMinVersion = "1.3"
)

// Options contains everything necessary to create and run webhook server.
type Options struct {
	// BindAddress is the IP address on which to listen for the --secure-port port.
	// Default is "0.0.0.0".
	BindAddress string
	// SecurePort is the port that the webhook server serves at.
	// Default is 8443.
	SecurePort int
	// CertDir is the directory that contains the server key and certificate.
	// if not set, webhook server would look up the server key and certificate in {TempDir}/k8s-webhook-server/serving-certs.
	CertDir string
	// CertName is the server certificate name. Defaults to tls.crt.
	CertName string
	// KeyName is the server key name. Defaults to tls.key.
	KeyName string
	// TLSMinVersion is the minimum version of TLS supported. Possible values: 1.0, 1.1, 1.2, 1.3.
	// Some environments have automated security scans that trigger on TLS versions or insecure cipher suites, and
	// setting TLS to 1.3 would solve both problems.
	// Defaults to 1.3.
	TLSMinVersion string
	// KubeAPIQPS is the QPS to use while talking with karmada-apiserver.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with karmada-apiserver.
	KubeAPIBurst int
	// MetricsBindAddress is the TCP address that the controller should bind to
	// for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	// Defaults to ":8080".
	MetricsBindAddress string
	// HealthProbeBindAddress is the TCP address that the controller should bind to
	// for serving health probes
	// Defaults to ":8000".
	HealthProbeBindAddress string

	DefaultNotReadyTolerationSeconds    int64
	DefaultUnreachableTolerationSeconds int64

	ProfileOpts profileflag.Options
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	return &Options{}
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.Lookup("kubeconfig").Usage = "Path to karmada control plane kubeconfig file."

	flags.StringVar(&o.BindAddress, "bind-address", defaultBindAddress,
		"The IP address on which to listen for the --secure-port port.")
	flags.IntVar(&o.SecurePort, "secure-port", defaultPort,
		"The secure port on which to serve HTTPS.")
	flags.StringVar(&o.CertDir, "cert-dir", defaultCertDir,
		"The directory that contains the server key and certificate.")
	flags.StringVar(&o.CertName, "tls-cert-file-name", "tls.crt", "The name of server certificate.")
	flags.StringVar(&o.KeyName, "tls-private-key-file-name", "tls.key", "The name of server key.")
	flags.StringVar(&o.TLSMinVersion, "tls-min-version", defaultTLSMinVersion, "Minimum TLS version supported. Possible values: 1.0, 1.1, 1.2, 1.3.")
	flags.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with karmada-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	flags.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with karmada-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	flags.StringVar(&o.MetricsBindAddress, "metrics-bind-address", ":8080", "The TCP address that the controller should bind to for serving prometheus metrics(e.g. 127.0.0.1:8080, :8080). It can be set to \"0\" to disable the metrics serving.")
	flags.StringVar(&o.HealthProbeBindAddress, "health-probe-bind-address", ":8000", "The TCP address that the controller should bind to for serving health probes(e.g. 127.0.0.1:8000, :8000)")

	// webhook flags
	flags.Int64Var(&o.DefaultNotReadyTolerationSeconds, "default-not-ready-toleration-seconds", 300, "Indicates the tolerationSeconds of the propagation policy toleration for notReady:NoExecute that is added by default to every propagation policy that does not already have such a toleration.")
	flags.Int64Var(&o.DefaultUnreachableTolerationSeconds, "default-unreachable-toleration-seconds", 300, "Indicates the tolerationSeconds of the propagation policy toleration for unreachable:NoExecute that is added by default to every propagation policy that does not already have such a toleration.")

	o.ProfileOpts.AddFlags(flags)
}
