/*
Copyright The Karmada Authors.

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
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/clientcmd"
	apiserverflag "k8s.io/component-base/cli/flag"

	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

var (
	rootCmdShort = "%s controls a Kubernetes Cluster Federation."
	rootCmdLong  = "%s controls a Kubernetes Cluster Federation."
)

// NewKarmadaCtlCommand creates the `karmadactl` command.
func NewKarmadaCtlCommand(out io.Writer, cmdUse, cmdStr string) *cobra.Command {
	// Parent command to which all sub-commands are added.
	rootCmd := &cobra.Command{
		Use:   cmdUse,
		Short: fmt.Sprintf(rootCmdShort, cmdStr),
		Long:  fmt.Sprintf(rootCmdLong, cmdStr),

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
	rootCmd.AddCommand(NewCmdJoin(out, karmadaConfig, cmdStr))
	rootCmd.AddCommand(NewCmdUnjoin(out, karmadaConfig, cmdStr))
	rootCmd.AddCommand(sharedcommand.NewCmdVersion(out, cmdStr))
	rootCmd.AddCommand(NewCmdCordon(out, karmadaConfig, cmdStr))
	rootCmd.AddCommand(NewCmdUncordon(out, karmadaConfig, cmdStr))
	rootCmd.AddCommand(NewCmdGet(out, karmadaConfig))

	return rootCmd
}

func runHelp(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}
