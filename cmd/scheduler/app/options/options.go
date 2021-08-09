package options

import (
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"

	"github.com/karmada-io/karmada/pkg/util"
)

const (
	defaultBindAddress = "0.0.0.0"
	defaultPort        = 10351
)

var (
	defaultElectionLeaseDuration = metav1.Duration{Duration: 15 * time.Second}
	defaultElectionRenewDeadline = metav1.Duration{Duration: 10 * time.Second}
	defaultElectionRetryPeriod   = metav1.Duration{Duration: 2 * time.Second}
)

// Options contains everything necessary to create and run controller-manager.
type Options struct {
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
	KubeConfig     string
	Master         string
	// BindAddress is the IP address on which to listen for the --secure-port port.
	BindAddress string
	// SecurePort is the port that the server serves at.
	SecurePort int

	// Failover indicates if scheduler should reschedule on cluster failure.
	Failover bool
}

// NewOptions builds an default scheduler options.
func NewOptions() *Options {
	return &Options{
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: util.NamespaceKarmadaSystem,
			ResourceName:      "karmada-scheduler",
			LeaseDuration:     defaultElectionLeaseDuration,
			RenewDeadline:     defaultElectionRenewDeadline,
			RetryPeriod:       defaultElectionRetryPeriod,
		},
	}
}

// AddFlags adds flags of scheduler to the specified FlagSet
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}

	fs.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", true, "Enable leader election, which must be true when running multi instances.")
	fs.StringVar(&o.KubeConfig, "kubeconfig", o.KubeConfig, "Path to a KubeConfig. Only required if out-of-cluster.")
	fs.StringVar(&o.Master, "master", o.Master, "The address of the Kubernetes API server. Overrides any value in KubeConfig. Only required if out-of-cluster.")
	fs.StringVar(&o.BindAddress, "bind-address", defaultBindAddress, "The IP address on which to listen for the --secure-port port.")
	fs.IntVar(&o.SecurePort, "secure-port", defaultPort, "The secure port on which to serve HTTPS.")
	fs.BoolVar(&o.Failover, "failover", false, "Reschedule on cluster failure.")
}
