/*
Copyright 2022 The Karmada Authors.

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

package describe

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubectldescribe "k8s.io/kubectl/pkg/cmd/describe"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

var (
	describeLong = templates.LongDesc(`
		Show details of a specific resource or group of resources in Karmada control plane or a member cluster.

		Print a detailed description of the selected resources, including related
		resources such as events or controllers. You may select a single object by name,
		all objects of that type, provide a name prefix, or label selector. For example:

		$ %[1]s describe TYPE NAME_PREFIX

		will first check for an exact match on TYPE and NAME_PREFIX. If no such
		resource exists, it will output details for every resource that has a name
		prefixed with NAME_PREFIX.`)

	describeExample = templates.Examples(`
		# Describe a deployment in Karmada control plane
		%[1]s describe deployment/nginx

		# Describe a pod in cluster(member1)
		%[1]s describe pods/nginx --operation-scope=members --cluster=member1

		# Describe all pods in cluster(member1)
		%[1]s describe pods --operation-scope=members --cluster=member1

		# Describe a pod identified by type and name in "pod.json" in cluster(member1)
		%[1]s describe -f pod.json --operation-scope=members --cluster=member1

		# Describe pods by label name=myLabel in cluster(member1)
		%[1]s describe po -l name=myLabel --operation-scope=members --cluster=member1

		# Describe all pods managed by the 'frontend' replication controller in cluster(member1)
		# (rc-created pods get the name of the rc as a prefix in the pod name)
		%[1]s describe pods frontend --operation-scope=members --cluster=member1`)
)

// NewCmdDescribe new describe command.
func NewCmdDescribe(f util.Factory, parentCommand string, streams genericiooptions.IOStreams) *cobra.Command {
	o := &CommandDescribeOptions{}
	kubedescribeFlags := kubectldescribe.NewDescribeFlags(f, streams)

	cmd := &cobra.Command{
		Use:                   "describe (-f FILENAME | TYPE [NAME_PREFIX | -l label] | TYPE/NAME) (--operation-scope=SCOPE --cluster=CLUSTER)",
		Short:                 "Show details of a specific resource or group of resources in Karmada control plane or a member cluster",
		Long:                  fmt.Sprintf(describeLong, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		ValidArgsFunction:     utilcomp.ResourceTypeAndNameCompletionFunc(f),
		Example:               fmt.Sprintf(describeExample, parentCommand),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := o.Complete(f, args, kubedescribeFlags, parentCommand); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}
			return nil
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterTroubleshootingAndDebugging,
		},
	}

	flags := cmd.Flags()
	kubedescribeFlags.AddFlags(cmd)

	options.AddKubeConfigFlags(flags)
	options.AddNamespaceFlag(flags)
	o.OperationScope = options.KarmadaControlPlane
	flags.VarP(&o.OperationScope, "operation-scope", "s", "Used to control the operation scope of the command. The optional values are karmada and members. Defaults to karmada.")
	flags.StringVarP(&o.Cluster, "cluster", "C", "", "Used to specify a target member cluster and only takes effect when the command's operation scope is members, for example: --operation-scope=members --cluster=member1")

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(cmd, f)
	utilcomp.RegisterCompletionFuncForOperationScopeFlag(cmd, options.KarmadaControlPlane, options.Members)
	utilcomp.RegisterCompletionFuncForClusterFlag(cmd)
	return cmd
}

// CommandDescribeOptions contains the input to the describe command.
type CommandDescribeOptions struct {
	// flags specific to describe
	KubectlDescribeOptions *kubectldescribe.DescribeOptions
	Cluster                string
	OperationScope         options.OperationScope
}

// Complete ensures that options are valid and marshals them if necessary
func (o *CommandDescribeOptions) Complete(f util.Factory, args []string, describeFlag *kubectldescribe.DescribeFlags, parentCommand string) error {
	if o.OperationScope == options.KarmadaControlPlane {
		describeFlag.Factory = f
	}
	if o.OperationScope == options.Members && len(o.Cluster) != 0 {
		memberFactory, err := f.FactoryForMemberCluster(o.Cluster)
		if err != nil {
			return err
		}
		describeFlag.Factory = memberFactory
	}

	var err error
	o.KubectlDescribeOptions, err = describeFlag.ToOptions(parentCommand, args)
	if err != nil {
		return err
	}
	return nil
}

// Validate checks if the parameters are valid
func (o *CommandDescribeOptions) Validate() error {
	err := options.VerifyOperationScopeFlags(o.OperationScope, options.KarmadaControlPlane, options.Members)
	if err != nil {
		return err
	}
	if o.OperationScope == options.Members && len(o.Cluster) == 0 {
		return errors.New("must specify a member cluster")
	}
	return nil
}

// Run describe information of resources
func (o *CommandDescribeOptions) Run() error {
	return o.KubectlDescribeOptions.Run()
}
