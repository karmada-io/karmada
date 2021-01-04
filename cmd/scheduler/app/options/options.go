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

// Options contains everything necessary to create and run controller-manager.
type Options struct {
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
	KubeConfig     string
	Master         string
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	return &Options{}
}

// AddEtcdFlags adds flags related to etcd storage for a specific APIServer to the specified FlagSet
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}

	fs.StringVar(&o.KubeConfig, "kubeconfig", o.KubeConfig, "Path to a KubeConfig. Only required if out-of-cluster.")
	fs.StringVar(&o.Master, "master", o.Master, "The address of the Kubernetes API server. Overrides any value in KubeConfig. Only required if out-of-cluster.")
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
