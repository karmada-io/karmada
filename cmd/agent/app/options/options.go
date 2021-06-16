package options

import (
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Options contains everything necessary to create and run controller-manager.
type Options struct {
	KarmadaKubeConfig string
	ClusterName       string

	// ClusterStatusUpdateFrequency is the frequency that controller computes and report cluster status.
	// It must work with ClusterMonitorGracePeriod(--cluster-monitor-grace-period) in karmada-controller-manager.
	ClusterStatusUpdateFrequency metav1.Duration
	// ClusterLeaseDuration is a duration that candidates for a lease need to wait to force acquire it.
	// This is measure against time of last observed RenewTime.
	ClusterLeaseDuration metav1.Duration
	// ClusterLeaseRenewIntervalFraction is a fraction coordinated with ClusterLeaseDuration that
	// how long the current holder of a lease has last updated the lease.
	ClusterLeaseRenewIntervalFraction float64
}

// NewOptions builds an default scheduler options.
func NewOptions() *Options {
	return &Options{}
}

// AddFlags adds flags of scheduler to the specified FlagSet
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}

	fs.StringVar(&o.KarmadaKubeConfig, "karmada-kubeconfig", o.KarmadaKubeConfig, "Path to karmada kubeconfig.")
	fs.StringVar(&o.ClusterName, "cluster-name", o.ClusterName, "Name of member cluster that the agent serves for.")
	fs.DurationVar(&o.ClusterStatusUpdateFrequency.Duration, "cluster-status-update-frequency", 10*time.Second, "Specifies how often karmada-agent posts cluster status to karmada-apiserver. Note: be cautious when changing the constant, it must work with ClusterMonitorGracePeriod in karmada-controller-manager.")
	fs.DurationVar(&o.ClusterLeaseDuration.Duration, "cluster-lease-duration", 40*time.Second,
		"Specifies the expiration period of a cluster lease.")
	fs.Float64Var(&o.ClusterLeaseRenewIntervalFraction, "cluster-lease-renew-interval-fraction", 0.25,
		"Specifies the cluster lease renew interval fraction.")
}
