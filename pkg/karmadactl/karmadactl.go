package karmadactl

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	apiserverflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/addons"
	"github.com/karmada-io/karmada/pkg/karmadactl/apply"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit"
	"github.com/karmada-io/karmada/pkg/karmadactl/cordon"
	"github.com/karmada-io/karmada/pkg/karmadactl/deinit"
	"github.com/karmada-io/karmada/pkg/karmadactl/describe"
	"github.com/karmada-io/karmada/pkg/karmadactl/exec"
	"github.com/karmada-io/karmada/pkg/karmadactl/get"
	"github.com/karmada-io/karmada/pkg/karmadactl/interpret"
	"github.com/karmada-io/karmada/pkg/karmadactl/join"
	"github.com/karmada-io/karmada/pkg/karmadactl/logs"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/promote"
	"github.com/karmada-io/karmada/pkg/karmadactl/register"
	"github.com/karmada-io/karmada/pkg/karmadactl/taint"
	"github.com/karmada-io/karmada/pkg/karmadactl/token"
	"github.com/karmada-io/karmada/pkg/karmadactl/unjoin"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
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
	f := util.NewFactory(options.DefaultConfigFlags)
	ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	groups := templates.CommandGroups{
		{
			Message: "Basic Commands:",
			Commands: []*cobra.Command{
				get.NewCmdGet(f, parentCommand, ioStreams),
			},
		},
		{
			Message: "Cluster Registration Commands:",
			Commands: []*cobra.Command{
				cmdinit.NewCmdInit(parentCommand),
				deinit.NewCmdDeInit(parentCommand),
				addons.NewCmdAddons(parentCommand),
				join.NewCmdJoin(f, parentCommand),
				unjoin.NewCmdUnjoin(f, parentCommand),
				token.NewCmdToken(f, parentCommand, ioStreams),
				register.NewCmdRegister(parentCommand),
			},
		},
		{
			Message: "Cluster Management Commands:",
			Commands: []*cobra.Command{
				cordon.NewCmdCordon(f, parentCommand),
				cordon.NewCmdUncordon(f, parentCommand),
				taint.NewCmdTaint(f, parentCommand),
			},
		},
		{
			Message: "Troubleshooting and Debugging Commands:",
			Commands: []*cobra.Command{
				logs.NewCmdLogs(f, parentCommand, ioStreams),
				exec.NewCmdExec(f, parentCommand, ioStreams),
				describe.NewCmdDescribe(f, parentCommand, ioStreams),
				interpret.NewCmdInterpret(f, parentCommand, ioStreams),
			},
		},
		{
			Message: "Advanced Commands:",
			Commands: []*cobra.Command{
				apply.NewCmdApply(f, parentCommand, ioStreams),
				promote.NewCmdPromote(f, parentCommand),
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

func runHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
