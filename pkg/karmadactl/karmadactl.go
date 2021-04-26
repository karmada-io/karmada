package karmadactl

import (
	"flag"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/clientcmd"
	apiserverflag "k8s.io/component-base/cli/flag"
)

// NewKarmadaCtlCommand creates the `karmadactl` command.
func NewKarmadaCtlCommand(out io.Writer) *cobra.Command {
	// Parent command to which all sub-commands are added.
	rootCmd := &cobra.Command{
		Use:   "karmadactl",
		Short: "karmadactl controls a Kubernetes Cluster Federation.",
		Long:  "karmadactl controls a Kubernetes Cluster Federation.",

		RunE: runHelp,
	}

	// Add the command line flags from other dependencies (e.g., klog), but do not
	// warn if they contain underscores.
	pflag.CommandLine.SetNormalizeFunc(apiserverflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	rootCmd.PersistentFlags().AddFlagSet(pflag.CommandLine)

	// From this point and forward we get warnings on flags that contain "_" separators
	rootCmd.SetGlobalNormalizationFunc(apiserverflag.WarnWordSepNormalizeFunc)

	// Prevent klog errors about logging before parsing.
	_ = flag.CommandLine.Parse(nil)

	karmadaConfig := NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
	rootCmd.AddCommand(NewCmdJoin(out, karmadaConfig))
	rootCmd.AddCommand(NewCmdUnjoin(out, karmadaConfig))
	rootCmd.AddCommand(NewCmdVersion(out))
	return rootCmd
}

func runHelp(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
