package options

import (
	"github.com/spf13/pflag"
)

// Options contains everything necessary to create and run controller-manager.
type Options struct {
	KarmadaKubeConfig string
	ClusterName       string
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
}
