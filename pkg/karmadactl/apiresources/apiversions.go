/*
Copyright 2024 The Karmada Authors.

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

package apiresources

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubectlapiresources "k8s.io/kubectl/pkg/cmd/apiresources"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

var (
	apiversionsExample = templates.Examples(`
		# Print the supported API versions
		%[1]s api-versions

		# Print the supported API versions in cluster(member1) 
		%[1]s api-versions --operation-scope=members --cluster=member1`)
)

// NewCmdAPIVersions creates the api-versions command
func NewCmdAPIVersions(f util.Factory, parentCommand string, ioStreams genericiooptions.IOStreams) *cobra.Command {
	var o CommandAPIVersionsOptions
	o.APIVersionsOptions = kubectlapiresources.NewAPIVersionsOptions(ioStreams)
	cmd := &cobra.Command{
		Use:                   "api-versions",
		Short:                 "Print the supported API versions on the server, in the form of \"group/version\"",
		Long:                  "Print the supported API versions on the server, in the form of \"group/version\".",
		Example:               fmt.Sprintf(apiversionsExample, parentCommand),
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunAPIVersions())
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupOtherCommands,
		},
	}

	o.OperationScope = options.KarmadaControlPlane
	options.AddKubeConfigFlags(cmd.Flags())
	cmd.Flags().VarP(&o.OperationScope, "operation-scope", "s", "Used to control the operation scope of the command. The optional values are karmada and members. Defaults to karmada.")
	cmd.Flags().StringVar(&o.Cluster, "cluster", "", "Used to specify a target member cluster and only takes effect when the command's operation scope is members, for example: --operation-scope=members --cluster=member1")

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForOperationScopeFlag(cmd, options.KarmadaControlPlane, options.Members)
	utilcomp.RegisterCompletionFuncForClusterFlag(cmd)

	return cmd
}

// CommandAPIVersionsOptions contains the input to the api-versions command.
type CommandAPIVersionsOptions struct {
	// flags specific to api-versions
	*kubectlapiresources.APIVersionsOptions
	Cluster        string
	OperationScope options.OperationScope
}

// Complete adapts from the command line args and factory to the data required
func (o *CommandAPIVersionsOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	var apiFactory cmdutil.Factory = f
	if o.OperationScope == options.Members && len(o.Cluster) != 0 {
		memberFactory, err := f.FactoryForMemberCluster(o.Cluster)
		if err != nil {
			return err
		}
		apiFactory = memberFactory
	}
	return o.APIVersionsOptions.Complete(apiFactory, cmd, args)
}

// Validate checks to the APIVersionsOptions to see if there is sufficient information run the command
func (o *CommandAPIVersionsOptions) Validate() error {
	err := options.VerifyOperationScopeFlags(o.OperationScope, options.KarmadaControlPlane, options.Members)
	if err != nil {
		return err
	}
	if o.OperationScope == options.Members && len(o.Cluster) == 0 {
		return fmt.Errorf("must specify a member cluster")
	}
	return nil
}

// Run does the work
func (o *CommandAPIVersionsOptions) Run() error {
	return o.APIVersionsOptions.RunAPIVersions()
}
