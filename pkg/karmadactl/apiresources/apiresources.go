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
	apiresourcesExample = templates.Examples(`
		# Print the supported API resources in Karmada control plane
		%[1]s api-resources

		# Print the supported API resources with more information in cluster(member1)
		%[1]s api-resources -o wide --operation-scope=members --cluster=member1

		# Print the supported API resources sorted by a column in Karmada control plane
		%[1]s api-resources --sort-by=name

		# Print the supported namespaced resources in Karmada control plane
		%[1]s api-resources --namespaced=true

		# Print the supported non-namespaced resources in Karmada control plane
		%[1]s api-resources --namespaced=false

		# Print the supported API resources with a specific APIGroup in Karmada control plane
		%[1]s api-resources --api-group=rbac.authorization.k8s.io`)
)

// NewCmdAPIResources creates the api-resources command
func NewCmdAPIResources(f util.Factory, parentCommand string, ioStreams genericiooptions.IOStreams) *cobra.Command {
	var o CommandAPIResourcesOptions
	o.APIResourceOptions = kubectlapiresources.NewAPIResourceOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "api-resources",
		Short:   "Print the supported API resources on the server",
		Long:    "Print the supported API resources on the server.",
		Example: fmt.Sprintf(apiresourcesExample, parentCommand),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunAPIResources())
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupOtherCommands,
		},
	}

	o.OperationScope = options.KarmadaControlPlane
	options.AddKubeConfigFlags(cmd.Flags())
	cmd.Flags().VarP(&o.OperationScope, "operation-scope", "s", "Used to control the operation scope of the command. The optional values are karmada and members. Defaults to karmada.")
	cmd.Flags().StringVar(&o.Cluster, "cluster", "", "Used to specify a target member cluster and only takes effect when the command's operation scope is members, for example: --operation-scope=members --cluster=member1")
	cmd.Flags().BoolVar(&o.NoHeaders, "no-headers", o.NoHeaders, "When using the default or custom-column output format, don't print headers (default print headers).")
	cmd.Flags().StringVarP(&o.Output, "output", "o", o.Output, `Output format. One of: (wide, name).`)

	cmd.Flags().StringVar(&o.APIGroup, "api-group", o.APIGroup, "Limit to resources in the specified API group.")
	cmd.Flags().BoolVar(&o.Namespaced, "namespaced", o.Namespaced, "If false, non-namespaced resources will be returned, otherwise returning namespaced resources by default.")
	cmd.Flags().StringSliceVar(&o.Verbs, "verbs", o.Verbs, "Limit to resources that support the specified verbs.")
	cmd.Flags().StringVar(&o.SortBy, "sort-by", o.SortBy, "If non-empty, sort list of resources using specified field. The field can be either 'name' or 'kind'.")
	cmd.Flags().BoolVar(&o.Cached, "cached", o.Cached, "Use the cached list of resources if available.")
	cmd.Flags().StringSliceVar(&o.Categories, "categories", o.Categories, "Limit to resources that belong to the specified categories.")

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForOperationScopeFlag(cmd, options.KarmadaControlPlane, options.Members)
	utilcomp.RegisterCompletionFuncForClusterFlag(cmd)
	return cmd
}

// CommandAPIResourcesOptions contains the input to the api-resources command.
type CommandAPIResourcesOptions struct {
	// flags specific to describe
	*kubectlapiresources.APIResourceOptions
	Cluster        string
	OperationScope options.OperationScope
}

// Complete adapts from the command line args and validates them
func (o *CommandAPIResourcesOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	var apiFactory cmdutil.Factory = f
	if o.OperationScope == options.Members && len(o.Cluster) != 0 {
		memberFactory, err := f.FactoryForMemberCluster(o.Cluster)
		if err != nil {
			return err
		}
		apiFactory = memberFactory
	}
	return o.APIResourceOptions.Complete(apiFactory, cmd, args)
}

// Validate checks to the APIResourceOptions to see if there is sufficient information run the command
func (o *CommandAPIResourcesOptions) Validate() error {
	err := options.VerifyOperationScopeFlags(o.OperationScope, options.KarmadaControlPlane, options.Members)
	if err != nil {
		return err
	}
	if o.OperationScope == options.Members && len(o.Cluster) == 0 {
		return fmt.Errorf("must specify a member cluster")
	}
	return o.APIResourceOptions.Validate()
}

// Run does the work
func (o *CommandAPIResourcesOptions) Run() error {
	return o.APIResourceOptions.RunAPIResources()
}
