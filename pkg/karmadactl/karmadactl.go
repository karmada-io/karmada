package karmadactl

import (
	"flag"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/client-go/tools/clientcmd"
	apiserverflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

var (
	rootCmdShort = "%s controls a Kubernetes Cluster Federation."
	rootCmdLong  = "%s controls a Kubernetes Cluster Federation."
)

// NewKarmadaCtlCommand creates the `karmadactl` command.
func NewKarmadaCtlCommand(out io.Writer, cmdUse, parentCommand string) *cobra.Command {
	// Parent command to which all sub-commands are added.
	rootCmd := &cobra.Command{
		Use:   cmdUse,
		Short: fmt.Sprintf(rootCmdShort, parentCommand),
		Long:  fmt.Sprintf(rootCmdLong, parentCommand),

		RunE: runHelp,
	}

	// Init log flags
	klog.InitFlags(flag.CommandLine)

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
	rootCmd.AddCommand(NewCmdJoin(out, karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdUnjoin(out, karmadaConfig, parentCommand))
	rootCmd.AddCommand(sharedcommand.NewCmdVersion(out, parentCommand))
	rootCmd.AddCommand(NewCmdCordon(out, karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdUncordon(out, karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdGet(out, karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdTaint(out, karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdPromote(out, karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdLogs(out, karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdExec(out, karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdDescribe(out, karmadaConfig, parentCommand))
	rootCmd.AddCommand(cmdinit.NewCmdInit(out, parentCommand))
	rootCmd.AddCommand(NewCmdDeInit(out, parentCommand))

	return rootCmd
}

func runHelp(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
