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
	defaultPort        = 10357
)

// Options contains everything necessary to create and run controller-manager.
type Options struct {
	HostNamespace  string
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
	// BindAddress is the IP address on which to listen for the --secure-port port.
	BindAddress string
	// SecurePort is the port that the the server serves at.
	// Note: We hope support https in the future once controller-runtime provides the functionality.
	SecurePort int
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	return &Options{}
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (o *Options) Complete() {
	if len(o.HostNamespace) == 0 {
		o.HostNamespace = "default"
		klog.Infof("Set default value: Options.HostNamespace = %s", "default")
	}

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
}
