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

package explain

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubectlexplain "k8s.io/kubectl/pkg/cmd/explain"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

var (
	explainLong = templates.LongDesc(`
		Describe fields and structure of various resources in Karmada control plane or a member cluster.

		This command describes the fields associated with each supported API resource.
		Fields are identified via a simple JSONPath identifier:

			<type>.<fieldName>[.<fieldName>]

		Information about each field is retrieved from the server in OpenAPI format.`)

	explainExamples = templates.Examples(`
		# Get the documentation of the resource and its fields in Karmada control plane
		%[1]s explain propagationpolicies

		# Get all the fields in the resource in member cluster member1
		%[1]s explain pods --recursive --operation-scope=members --cluster=member1

		# Get the explanation for resourcebindings in supported api versions in Karmada control plane
		%[1]s explain resourcebindings --api-version=work.karmada.io/v1alpha1

		# Get the documentation of a specific field of a resource in member cluster member1
		%[1]s explain pods.spec.containers --operation-scope=members --cluster=member1
		
		# Get the documentation of resources in different format in Karmada control plane
		%[1]s explain clusterpropagationpolicies --output=plaintext-openapiv2`)
)

// NewCmdExplain new explain command.
func NewCmdExplain(f util.Factory, parentCommand string, streams genericiooptions.IOStreams) *cobra.Command {
	var o CommandExplainOptions
	o.ExplainFlags = kubectlexplain.NewExplainFlags(streams)

	cmd := &cobra.Command{
		Use:                   "explain TYPE [--recursive=FALSE|TRUE] [--api-version=api-version-group] [--output=plaintext|plaintext-openapiv2] ",
		DisableFlagsInUseLine: true,
		Short:                 "Get documentation for a resource",
		Long:                  fmt.Sprintf(explainLong, parentCommand),
		Example:               fmt.Sprintf(explainExamples, parentCommand),
		Run: func(_ *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, parentCommand, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupBasic,
		},
	}

	o.ExplainFlags.AddFlags(cmd)

	flags := cmd.Flags()
	o.OperationScope = options.KarmadaControlPlane
	options.AddKubeConfigFlags(flags)
	options.AddNamespaceFlag(flags)
	flags.VarP(&o.OperationScope, "operation-scope", "s", "Used to control the operation scope of the command. The optional values are karmada and members. Defaults to karmada.")
	flags.StringVar(&o.Cluster, "cluster", "", "Used to specify a target member cluster and only takes effect when the command's operation scope is member clusters, for example: --operation-scope=all --cluster=member1")

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(cmd, f)
	utilcomp.RegisterCompletionFuncForOperationScopeFlag(cmd, options.KarmadaControlPlane, options.Members)
	utilcomp.RegisterCompletionFuncForClusterFlag(cmd)
	return cmd
}

// CommandExplainOptions contains the input to the explain command.
type CommandExplainOptions struct {
	// flags specific to explain
	*kubectlexplain.ExplainOptions
	*kubectlexplain.ExplainFlags
	Cluster        string
	OperationScope options.OperationScope
}

// Complete ensures that options are valid and marshals them if necessary
func (o *CommandExplainOptions) Complete(f util.Factory, parentCommand string, args []string) error {
	var explainFactory cmdutil.Factory = f
	if o.OperationScope == options.Members && len(o.Cluster) != 0 {
		memberFactory, err := f.FactoryForMemberCluster(o.Cluster)
		if err != nil {
			return err
		}
		explainFactory = memberFactory
	}

	var err error
	o.ExplainOptions, err = o.ExplainFlags.ToOptions(explainFactory, parentCommand, args)
	return err
}

// Validate checks that the provided explain options are specified
func (o *CommandExplainOptions) Validate() error {
	err := options.VerifyOperationScopeFlags(o.OperationScope, options.KarmadaControlPlane, options.Members)
	if err != nil {
		return err
	}
	if o.OperationScope == options.Members && len(o.Cluster) == 0 {
		return fmt.Errorf("must specify a member cluster")
	}
	return o.ExplainOptions.Validate()
}

// Run executes the appropriate steps to print a model's documentation
func (o *CommandExplainOptions) Run() error {
	return o.ExplainOptions.Run()
}
