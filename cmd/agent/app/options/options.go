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

	// ClusterStatusUpdateFrequency is the frequency that karmada-agent computes cluster status.
	// If cluster lease feature is not enabled, it is also the frequency that karmada-agent posts cluster status
	// to karmada-apiserver. In that case, be cautious when changing the constant, it must work with
	// ClusterMonitorGracePeriod(--cluster-monitor-grace-period) in karmada-controller-manager.
	ClusterStatusUpdateFrequency metav1.Duration
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
}
