package options

import (
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/klog/v2"
)

var (
	defaultElectionLeaseDuration = metav1.Duration{Duration: 15 * time.Second}
	defaultElectionRenewDeadline = metav1.Duration{Duration: 10 * time.Second}
	defaultElectionRetryPeriod   = metav1.Duration{Duration: 2 * time.Second}
)

const (
	defaultBindAddress = "0.0.0.0"
	defaultPort        = 8443
	defaultCertDir     = "/tmp/k8s-webhook-server/serving-certs"
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
	// The server key and certificate must be named `tls.key` and `tls.crt`, respectively.
	CertDir        string
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	return &Options{}
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (o *Options) Complete() {
	if len(o.LeaderElection.ResourceLock) == 0 {
		o.LeaderElection.ResourceLock = resourcelock.EndpointsLeasesResourceLock
		klog.Infof("Set default value: Options.LeaderElection.ResourceLock = %s", resourcelock.EndpointsLeasesResourceLock)
	}

	if o.LeaderElection.LeaseDuration.Duration.Seconds() == 0 {
		o.LeaderElection.LeaseDuration = defaultElectionLeaseDuration
		klog.Infof("Set default value: Options.LeaderElection.LeaseDuration = %s", defaultElectionLeaseDuration.Duration.String())
	}

	if o.LeaderElection.RenewDeadline.Duration.Seconds() == 0 {
		o.LeaderElection.RenewDeadline = defaultElectionRenewDeadline
		klog.Infof("Set default value: Options.LeaderElection.RenewDeadline = %s", defaultElectionRenewDeadline.Duration.String())
	}

	if o.LeaderElection.RetryPeriod.Duration.Seconds() == 0 {
		o.LeaderElection.RetryPeriod = defaultElectionRetryPeriod
		klog.Infof("Set default value: Options.LeaderElection.RetryPeriod = %s", defaultElectionRetryPeriod.Duration.String())
	}
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.BindAddress, "bind-address", defaultBindAddress,
		"The IP address on which to listen for the --secure-port port.")
	flags.IntVar(&o.SecurePort, "secure-port", defaultPort,
		"The secure port on which to serve HTTPS.")
	flags.StringVar(&o.CertDir, "cert-dir", defaultCertDir,
		"The directory that contains the server key(named tls.key) and certificate(named tls.crt).")
}
