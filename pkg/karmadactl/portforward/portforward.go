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

package portforward

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	kubectlportforward "k8s.io/kubectl/pkg/cmd/portforward"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/completion"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

const (
	// Amount of time to wait until at least one pod is running
	defaultPodPortForwardWaitTimeout = 60 * time.Second
)

var (
	portforwardLong = templates.LongDesc(`
                Forward one or more local ports to a pod.

                Use resource type/name such as deployment/mydeployment to select a pod. Resource type defaults to 'pod' if omitted.

                If there are multiple pods matching the criteria, a pod will be selected automatically. The
                forwarding session ends when the selected pod terminates, and a rerun of the command is needed
                to resume forwarding.`)

	portforwardExample = templates.Examples(`
		# Listen on ports 5000 and 6000 locally, forwarding data to/from ports 5000 and 6000 in the pod
		%[1]s port-forward pod/mypod 5000 6000

		# Listen on ports 5000 and 6000 locally, forwarding data to/from ports 5000 and 6000 in a pod selected by the deployment
		%[1]s port-forward deployment/mydeployment 5000 6000

		# Listen on port 8443 locally, forwarding to the targetPort of the service's port named "https" in a pod selected by the service
		%[1]s port-forward service/myservice 8443:https

		# Listen on port 8888 locally, forwarding to 5000 in the pod
		%[1]s port-forward pod/mypod 8888:5000

		# Listen on port 8888 on all addresses, forwarding to 5000 in the pod
		%[1]s port-forward --address 0.0.0.0 pod/mypod 8888:5000

		# Listen on port 8888 on localhost and selected IP, forwarding to 5000 in the pod
		%[1]s port-forward --address localhost,10.19.21.23 pod/mypod 8888:5000

		# Listen on a random port locally, forwarding to 5000 in the pod
		%[1]s port-forward pod/mypod :5000`)
)

// NewCmdPortForWard new port-forward command.
func NewCmdPortForWard(f util.Factory, parentCommand string, streams genericiooptions.IOStreams) *cobra.Command {
	opts := &CommandPortForwardOptions{
		KubectlPortForwardOptions: &kubectlportforward.PortForwardOptions{
			PortForwarder: &defaultPortForwarder{
				IOStreams: streams,
			},
		}}

	cmd := &cobra.Command{
		Use:                   "port-forward TYPE/NAME [options] [LOCAL_PORT:]REMOTE_PORT [...[LOCAL_PORT_N:]REMOTE_PORT_N]",
		DisableFlagsInUseLine: true,
		Short:                 "Forward one or more local ports to a pod",
		Long:                  portforwardLong,
		Example:               fmt.Sprintf(portforwardExample, parentCommand),
		ValidArgsFunction:     completion.PodResourceNameCompletionFunc(f),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.PreCheck())
			cmdutil.CheckErr(opts.Complete(f, cmd, args))
			cmdutil.CheckErr(opts.Validate())
			cmdutil.CheckErr(opts.Run(context.Background()))
		},
	}
	cmdutil.AddPodRunningTimeoutFlag(cmd, defaultPodPortForwardWaitTimeout)
	cmd.Flags().StringSliceVar(&opts.KubectlPortForwardOptions.Address, "address", []string{"localhost"}, "Addresses to listen on (comma separated). Only accepts IP addresses or localhost as a value. When localhost is supplied, kubectl will try to bind on both 127.0.0.1 and ::1 and will fail if neither of these addresses are available to bind.")
	cmd.Flags().StringVarP(options.DefaultConfigFlags.Namespace, "namespace", "n", *options.DefaultConfigFlags.Namespace, "If present, the namespace scope for this CLI request")
	opts.OperationScope = options.KarmadaControlPlane
	cmd.Flags().Var(&opts.OperationScope, "operation-scope", "Used to control the operation scope of the command. The optional values are karmada and members. Defaults to karmada.")
	cmd.Flags().StringVarP(&opts.Cluster, "cluster", "C", "", "Used to specify a target member cluster and only takes effect when the command's operation scope is members, for example: --operation-scope=members --cluster=member1")

	return cmd
}

// CommandPortForwardOptions contains the input to the port-forward command.
type CommandPortForwardOptions struct {
	KubectlPortForwardOptions *kubectlportforward.PortForwardOptions
	Cluster                   string
	OperationScope            options.OperationScope
}

type defaultPortForwarder struct {
	genericiooptions.IOStreams
}

// ForwardPorts port-forward to specify url.
func (f *defaultPortForwarder) ForwardPorts(method string, url *url.URL, opts kubectlportforward.PortForwardOptions) error {
	transport, upgrader, err := spdy.RoundTripperFor(opts.Config)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, url)
	fw, err := portforward.NewOnAddresses(dialer, opts.Address, opts.Ports, opts.StopChannel, opts.ReadyChannel, f.Out, f.ErrOut)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

// PreCheck precondition check operation-scope and other flags.
func (o *CommandPortForwardOptions) PreCheck() error {
	err := options.VerifyOperationScopeFlags(o.OperationScope, options.KarmadaControlPlane, options.Members)
	if err != nil {
		return err
	}
	if o.OperationScope == options.Members && len(o.Cluster) == 0 {
		return fmt.Errorf("must specify a member cluster")
	}
	return nil
}

// Complete ensures that options are valid and marshals them if necessary
func (o *CommandPortForwardOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	var portForwardFactory cmdutil.Factory = f
	if o.OperationScope == options.KarmadaControlPlane {
		portForwardFactory = f
	}
	if o.OperationScope == options.Members && len(o.Cluster) != 0 {
		memberFactory, err := f.FactoryForMemberCluster(o.Cluster)
		if err != nil {
			return err
		}
		portForwardFactory = memberFactory
	}

	return o.KubectlPortForwardOptions.Complete(portForwardFactory, cmd, args)
}

// Validate checks if the parameters are valid
func (o *CommandPortForwardOptions) Validate() error {
	return o.KubectlPortForwardOptions.Validate()
}

// Run implements all the necessary functionality for port-forward cmd.
// It ends portforwarding when an error is received from the backend, or an os.Interrupt
// signal is received, or the provided context is done.
func (o *CommandPortForwardOptions) Run(ctx context.Context) error {
	return o.KubectlPortForwardOptions.RunPortForwardContext(ctx)
}
