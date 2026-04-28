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

package attach

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubectlattach "k8s.io/kubectl/pkg/cmd/attach"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

var (
	attachExample = templates.Examples(`
		# Get output from running pod mypod in cluster(member1); use the 'kubectl.kubernetes.io/default-container' annotation
		# for selecting the container to be attached or the first container in the pod will be chosen
		%[1]s attach mypod --cluster=member1

		# Get output from ruby-container from pod mypod in cluster(member1)
		%[1]s attach mypod -c ruby-container --cluster=member1

		# Switch to raw terminal mode; sends stdin to 'bash' in ruby-container from pod mypod in Karmada control plane
		# and sends stdout/stderr from 'bash' back to the client
		%[1]s attach mypod -c ruby-container -i -t

		# Get output from the first pod of a replica set named nginx in cluster(member1)
		%[1]s attach rs/nginx --cluster=member1
		`)
)

const (
	defaultPodAttachTimeout = 60 * time.Second
)

// NewCmdAttach new attach command.
func NewCmdAttach(f util.Factory, parentCommand string, streams genericiooptions.IOStreams) *cobra.Command {
	var o CommandAttachOptions
	o.AttachOptions = kubectlattach.NewAttachOptions(streams)

	cmd := &cobra.Command{
		Use:                   "attach (POD | TYPE/NAME) -c CONTAINER",
		DisableFlagsInUseLine: true,
		Short:                 "Attach to a running container",
		Long:                  "Attach to a process that is already running inside an existing container.",
		Example:               fmt.Sprintf(attachExample, parentCommand),
		ValidArgsFunction:     utilcomp.PodResourceNameCompletionFunc(f),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterTroubleshootingAndDebugging,
		},
	}

	cmdutil.AddPodRunningTimeoutFlag(cmd, defaultPodAttachTimeout)
	cmdutil.AddContainerVarFlags(cmd, &o.ContainerName, o.ContainerName)
	options.AddKubeConfigFlags(cmd.Flags())
	options.AddNamespaceFlag(cmd.Flags())
	o.OperationScope = options.KarmadaControlPlane
	cmd.Flags().BoolVarP(&o.Stdin, "stdin", "i", o.Stdin, "Pass stdin to the container")
	cmd.Flags().BoolVarP(&o.TTY, "tty", "t", o.TTY, "Stdin is a TTY")
	cmd.Flags().BoolVarP(&o.Quiet, "quiet", "q", o.Quiet, "Only print output from the remote session")
	cmd.Flags().VarP(&o.OperationScope, "operation-scope", "s", "Used to control the operation scope of the command. The optional values are karmada and members. Defaults to karmada.")
	cmd.Flags().StringVar(&o.Cluster, "cluster", "", "Used to specify a target member cluster and only takes effect when the command's operation scope is members, for example: --operation-scope=members --cluster=member1")

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(cmd, f)
	utilcomp.RegisterCompletionFuncForOperationScopeFlag(cmd, options.KarmadaControlPlane, options.Members)
	utilcomp.RegisterCompletionFuncForClusterFlag(cmd)
	return cmd
}

// CommandAttachOptions declare the arguments accepted by the attach command
type CommandAttachOptions struct {
	// flags specific to attach
	*kubectlattach.AttachOptions
	Cluster        string
	OperationScope options.OperationScope
}

// Complete verifies command line arguments and loads data from the command environment
func (o *CommandAttachOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	var attachFactory cmdutil.Factory = f
	if o.OperationScope == options.Members && len(o.Cluster) != 0 {
		memberFactory, err := f.FactoryForMemberCluster(o.Cluster)
		if err != nil {
			return err
		}
		attachFactory = memberFactory
	}
	return o.AttachOptions.Complete(attachFactory, cmd, args)
}

// Validate checks that the provided attach options are specified.
func (o *CommandAttachOptions) Validate() error {
	err := options.VerifyOperationScopeFlags(o.OperationScope, options.KarmadaControlPlane, options.Members)
	if err != nil {
		return err
	}
	if o.OperationScope == options.Members && len(o.Cluster) == 0 {
		return fmt.Errorf("must specify a member cluster")
	}
	return o.AttachOptions.Validate()
}

// Run executes a validated remote execution against a pod.
func (o *CommandAttachOptions) Run() error {
	return o.AttachOptions.Run()
}
