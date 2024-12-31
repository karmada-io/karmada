/*
Copyright 2020 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package karmadactl

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	apiserverflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/addons"
	"github.com/karmada-io/karmada/pkg/karmadactl/annotate"
	"github.com/karmada-io/karmada/pkg/karmadactl/apiresources"
	"github.com/karmada-io/karmada/pkg/karmadactl/apply"
	"github.com/karmada-io/karmada/pkg/karmadactl/attach"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit"
	"github.com/karmada-io/karmada/pkg/karmadactl/completion"
	"github.com/karmada-io/karmada/pkg/karmadactl/cordon"
	"github.com/karmada-io/karmada/pkg/karmadactl/create"
	"github.com/karmada-io/karmada/pkg/karmadactl/deinit"
	karmadactldelete "github.com/karmada-io/karmada/pkg/karmadactl/delete"
	"github.com/karmada-io/karmada/pkg/karmadactl/describe"
	"github.com/karmada-io/karmada/pkg/karmadactl/edit"
	"github.com/karmada-io/karmada/pkg/karmadactl/exec"
	"github.com/karmada-io/karmada/pkg/karmadactl/explain"
	"github.com/karmada-io/karmada/pkg/karmadactl/get"
	"github.com/karmada-io/karmada/pkg/karmadactl/interpret"
	"github.com/karmada-io/karmada/pkg/karmadactl/join"
	"github.com/karmada-io/karmada/pkg/karmadactl/label"
	"github.com/karmada-io/karmada/pkg/karmadactl/logs"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/patch"
	"github.com/karmada-io/karmada/pkg/karmadactl/promote"
	"github.com/karmada-io/karmada/pkg/karmadactl/register"
	"github.com/karmada-io/karmada/pkg/karmadactl/taint"
	"github.com/karmada-io/karmada/pkg/karmadactl/token"
	"github.com/karmada-io/karmada/pkg/karmadactl/top"
	"github.com/karmada-io/karmada/pkg/karmadactl/unjoin"
	"github.com/karmada-io/karmada/pkg/karmadactl/unregister"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
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
	ioStreams := genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	// Avoid import cycle by setting ValidArgsFunction here instead of in NewCmdGet()
	getCmd := get.NewCmdGet(f, parentCommand, ioStreams)
	getCmd.ValidArgsFunction = utilcomp.ResourceTypeAndNameCompletionFunc(f)
	utilcomp.RegisterCompletionFuncForClustersFlag(getCmd)
	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(getCmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(getCmd, f)
	utilcomp.RegisterCompletionFuncForOperationScopeFlag(getCmd)

	groups := templates.CommandGroups{
		{
			Message: "Basic Commands:",
			Commands: []*cobra.Command{
				explain.NewCmdExplain(f, parentCommand, ioStreams),
				getCmd,
				create.NewCmdCreate(f, parentCommand, ioStreams),
				karmadactldelete.NewCmdDelete(f, parentCommand, ioStreams),
				edit.NewCmdEdit(f, parentCommand, ioStreams),
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
				unregister.NewCmdUnregister(parentCommand),
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
				attach.NewCmdAttach(f, parentCommand, ioStreams),
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
				top.NewCmdTop(f, parentCommand, ioStreams),
				patch.NewCmdPatch(f, parentCommand, ioStreams),
			},
		},
		{
			Message: "Settings Commands:",
			Commands: []*cobra.Command{
				label.NewCmdLabel(f, parentCommand, ioStreams),
				annotate.NewCmdAnnotate(f, parentCommand, ioStreams),
				completion.NewCmdCompletion(parentCommand, ioStreams.Out, ""),
			},
		},
		{
			Message: "Other Commands:",
			Commands: []*cobra.Command{
				apiresources.NewCmdAPIResources(f, parentCommand, ioStreams),
				apiresources.NewCmdAPIVersions(f, parentCommand, ioStreams),
			},
		},
	}
	groups.Add(rootCmd)

	filters := []string{"options"}

	rootCmd.AddCommand(sharedcommand.NewCmdVersion(parentCommand))
	rootCmd.AddCommand(options.NewCmdOptions(parentCommand, ioStreams.Out))

	templates.ActsAsRootCommand(rootCmd, filters, groups...)

	utilcomp.SetFactoryForCompletion(f)

	return rootCmd
}

func runHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
