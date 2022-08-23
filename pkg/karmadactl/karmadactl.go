package karmadactl

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	apiserverflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/addons"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
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
	ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	groups := templates.CommandGroups{
		{
			Message: "Basic Commands:",
			Commands: []*cobra.Command{
				NewCmdGet(karmadaConfig, parentCommand, ioStreams),
			},
		},
		{
			Message: "Cluster Registeration Commands:",
			Commands: []*cobra.Command{
				cmdinit.NewCmdInit(parentCommand),
				NewCmdDeInit(parentCommand),
				addons.NewCommandAddons(parentCommand),
				NewCmdJoin(karmadaConfig, parentCommand),
				NewCmdUnjoin(karmadaConfig, parentCommand),
				NewCmdToken(karmadaConfig, parentCommand, ioStreams),
				NewCmdRegister(parentCommand),
			},
		},
		{
			Message: "Cluster Management Commands:",
			Commands: []*cobra.Command{
				NewCmdCordon(karmadaConfig, parentCommand),
				NewCmdUncordon(karmadaConfig, parentCommand),
				NewCmdTaint(karmadaConfig, parentCommand),
			},
		},
		{
			Message: "Troubleshooting and Debugging Commands:",
			Commands: []*cobra.Command{
				NewCmdLogs(karmadaConfig, parentCommand, ioStreams),
				NewCmdExec(karmadaConfig, parentCommand, ioStreams),
				NewCmdDescribe(karmadaConfig, parentCommand, ioStreams),
			},
		},
		{
			Message: "Advanced Commands:",
			Commands: []*cobra.Command{
				NewCmdApply(karmadaConfig, parentCommand, ioStreams),
				NewCmdPromote(karmadaConfig, parentCommand),
			},
		},
	}
	groups.Add(rootCmd)

	filters := []string{"options"}

	rootCmd.AddCommand(sharedcommand.NewCmdVersion(parentCommand))
	rootCmd.AddCommand(options.NewCmdOptions(parentCommand, ioStreams.Out))

	templates.ActsAsRootCommand(rootCmd, filters, groups...)

	return rootCmd
}

func runHelp(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
