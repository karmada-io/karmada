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

package exec

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubectlexec "k8s.io/kubectl/pkg/cmd/exec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

const (
	defaultPodExecTimeout = 60 * time.Second
)

var (
	execLong = templates.LongDesc(`
		Execute a command in a container.`)

	execExample = templates.Examples(`
		# Get output from running the 'date' command from pod mypod, using the first container by default
		%[1]s exec mypod -- date

		# Get output from running the 'date' command from pod mypod, using the first container by default in cluster(member1)
		%[1]s exec mypod --operation-scope=members --cluster=member1 -- date

		# Get output from running the 'date' command in ruby-container from pod mypod in cluster(member1)
		%[1]s exec mypod -c ruby-container --operation-scope=members --cluster=member1 -- date

		# Get output from running the 'date' command in ruby-container from pod mypod in cluster(member1)
		%[1]s exec mypod -c ruby-container --operation-scope=members --cluster=member1 -- date

		# Switch to raw terminal mode; sends stdin to 'bash' in ruby-container from pod mypod
		# and sends stdout/stderr from 'bash' back to the client
		%[1]s exec mypod -c ruby-container -i -t -- bash -il

		# Get output from running 'date' command from the first pod of the deployment mydeployment, using the first container by default in cluster(member1)
		%[1]s exec deploy/mydeployment --operation-scope=members --cluster=member1 -- date

		# Get output from running 'date' command from the first pod of the service myservice, using the first container by default in cluster(member1)
		%[1]s exec svc/myservice --operation-scope=members --cluster=member1 -- date`)
)

// NewCmdExec new exec command.
func NewCmdExec(f util.Factory, parentCommand string, streams genericiooptions.IOStreams) *cobra.Command {
	o := &CommandExecOptions{
		KubectlExecOptions: &kubectlexec.ExecOptions{
			StreamOptions: kubectlexec.StreamOptions{
				IOStreams: streams,
			},
			Executor: &kubectlexec.DefaultRemoteExecutor{},
		},
	}

	cmd := &cobra.Command{
		Use:                   "exec (POD | TYPE/NAME) [-c CONTAINER] (-C CLUSTER) -- COMMAND [args...]",
		Short:                 "Execute a command in a container in a cluster",
		Long:                  execLong,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		Example:               fmt.Sprintf(execExample, parentCommand),
		ValidArgsFunction:     utilcomp.PodResourceNameCompletionFunc(f),
		RunE: func(cmd *cobra.Command, args []string) error {
			argsLenAtDash := cmd.ArgsLenAtDash()
			if err := o.Complete(f, cmd, args, argsLenAtDash); err != nil {
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

	o.OperationScope = options.KarmadaControlPlane
	flags := cmd.Flags()
	options.AddKubeConfigFlags(flags)
	options.AddNamespaceFlag(flags)
	cmdutil.AddPodRunningTimeoutFlag(cmd, defaultPodExecTimeout)
	cmdutil.AddJsonFilenameFlag(flags, &o.KubectlExecOptions.FilenameOptions.Filenames, "to use to exec into the resource")
	cmdutil.AddContainerVarFlags(cmd, &o.KubectlExecOptions.ContainerName, o.KubectlExecOptions.ContainerName)

	flags.BoolVarP(&o.KubectlExecOptions.Stdin, "stdin", "i", o.KubectlExecOptions.Stdin, "Pass stdin to the container")
	flags.BoolVarP(&o.KubectlExecOptions.TTY, "tty", "t", o.KubectlExecOptions.TTY, "Stdin is a TTY")
	flags.BoolVarP(&o.KubectlExecOptions.Quiet, "quiet", "q", o.KubectlExecOptions.Quiet, "Only print output from the remote session")
	flags.VarP(&o.OperationScope, "operation-scope", "s", "Used to control the operation scope of the command. The optional values are karmada and members. Defaults to karmada.")
	flags.StringVar(&o.Cluster, "cluster", "", "Used to specify a target member cluster and only takes effect when the command's operation scope is members, for example: --operation-scope=members --cluster=member1")

	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc("container", utilcomp.ContainerCompletionFunc(f)))
	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(cmd, f)
	utilcomp.RegisterCompletionFuncForOperationScopeFlag(cmd, options.KarmadaControlPlane, options.Members)
	utilcomp.RegisterCompletionFuncForClusterFlag(cmd)

	return cmd
}

// CommandExecOptions declare the arguments accepted by the Exec command
type CommandExecOptions struct {
	// flags specific to exec
	KubectlExecOptions *kubectlexec.ExecOptions
	Cluster            string
	OperationScope     options.OperationScope
}

// Complete verifies command line arguments and loads data from the command environment
func (o *CommandExecOptions) Complete(f util.Factory, cmd *cobra.Command, argsIn []string, argsLenAtDash int) error {
	var execFactory cmdutil.Factory = f
	if o.OperationScope == options.Members && len(o.Cluster) != 0 {
		memberFactory, err := f.FactoryForMemberCluster(o.Cluster)
		if err != nil {
			return err
		}
		execFactory = memberFactory
	}
	return o.KubectlExecOptions.Complete(execFactory, cmd, argsIn, argsLenAtDash)
}

// Validate checks that the provided exec options are specified.
func (o *CommandExecOptions) Validate() error {
	err := options.VerifyOperationScopeFlags(o.OperationScope, options.KarmadaControlPlane, options.Members)
	if err != nil {
		return err
	}
	if o.OperationScope == options.Members && len(o.Cluster) == 0 {
		return errors.New("must specify a member cluster")
	}
	return o.KubectlExecOptions.Validate()
}

// Run executes a validated remote execution against a pod.
func (o *CommandExecOptions) Run() error {
	return o.KubectlExecOptions.Run()
}
