package options

import "github.com/spf13/pflag"

// GlobalCommandOptions holds the configuration shared by the all sub-commands of `karmadactl`.
type GlobalCommandOptions struct {
	// KubeConfig holds the control plane KUBECONFIG file path.
	KubeConfig string

	// KarmadaContext is the name of the cluster context in control plane KUBECONFIG file.
	// Default value is the current-context.
	KarmadaContext string

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool
}

// AddFlags adds flags to the specified FlagSet.
func (o *GlobalCommandOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Path to the control plane kubeconfig file.")
	flags.StringVar(&o.KarmadaContext, "karmada-context", "", "Name of the cluster context in control plane kubeconfig file.")
	flags.BoolVar(&o.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
}
