package karmadactl

import (
	"flag"
	"fmt"

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
func NewKarmadaCtlCommand(cmdUse, parentCommand string) *cobra.Command {
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
	rootCmd.AddCommand(NewCmdJoin(karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdUnjoin(karmadaConfig, parentCommand))
	rootCmd.AddCommand(sharedcommand.NewCmdVersion(parentCommand))
	rootCmd.AddCommand(NewCmdCordon(karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdUncordon(karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdGet(karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdApply(karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdTaint(karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdPromote(karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdLogs(karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdExec(karmadaConfig, parentCommand))
	rootCmd.AddCommand(NewCmdDescribe(karmadaConfig, parentCommand))
	rootCmd.AddCommand(cmdinit.NewCmdInit(parentCommand))
	rootCmd.AddCommand(NewCmdDeInit(parentCommand))

	return rootCmd
}

func runHelp(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
