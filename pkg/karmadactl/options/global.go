package options

import (
	"time"

	"github.com/spf13/pflag"
)

// DefaultKarmadaClusterNamespace defines the default namespace where the member cluster secrets are stored.
const DefaultKarmadaClusterNamespace = "karmada-cluster"

// DefaultKarmadactlCommandDuration defines the default timeout for karmadactl execute
const DefaultKarmadactlCommandDuration = 60 * time.Second

// GlobalCommandOptions holds the configuration shared by the all sub-commands of `karmadactl`.
type GlobalCommandOptions struct {
	// KubeConfig holds the control plane KUBECONFIG file path.
	KubeConfig string

	// ClusterContext is the name of the cluster context in control plane KUBECONFIG file.
	// Default value is the current-context.
	KarmadaContext string
}

// AddFlags adds flags to the specified FlagSet.
func (o *GlobalCommandOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Path to the control plane kubeconfig file.")
	flags.StringVar(&o.KarmadaContext, "karmada-context", "", "Name of the cluster context in control plane kubeconfig file.")
}
