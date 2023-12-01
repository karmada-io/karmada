/*
Copyright 2023 The Karmada Authors.

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

package config

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/completion"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	useContextExample = templates.Examples(`
		# Use the context for the minikube cluster
		%[1]s config use-context minikube`)
)

type useContextOptions struct {
	configAccess clientcmd.ConfigAccess
	streams      genericclioptions.IOStreams
	config       clientcmdapi.Config
	contextName  string
}

// NewCmdConfigUseContext returns a Command instance for 'config use-context' sub command
func NewCmdConfigUseContext(parentCommand string, streams genericclioptions.IOStreams, configAccess clientcmd.ConfigAccess) *cobra.Command {
	options := &useContextOptions{
		configAccess: configAccess,
		streams:      streams,
	}

	cmd := &cobra.Command{
		Use:                   "use-context CONTEXT_NAME",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Set the current-context in a kubeconfig file"),
		Aliases:               []string{"use"},
		Long:                  `Set the current-context in a kubeconfig file.`,
		Example:               fmt.Sprintf(useContextExample, parentCommand),
		ValidArgsFunction:     completion.ContextCompletionFunc,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(options.complete(cmd))
			cmdutil.CheckErr(options.validate())
			cmdutil.CheckErr(options.run())
			fmt.Fprintf(streams.Out, "Switched to context %q.\n", options.contextName)
		},
	}

	return cmd
}

func (o *useContextOptions) complete(cmd *cobra.Command) error {
	endingArgs := cmd.Flags().Args()
	if len(endingArgs) != 1 {
		err := cmd.Help()
		if err != nil {
			fmt.Fprintf(o.streams.Out, "puts out the help for the command karmadactl config use-context failed: %s", err.Error())
		}
		return fmt.Errorf("Unexpected args: %v", endingArgs)
	}

	o.contextName = endingArgs[0]
	return nil
}

func (o *useContextOptions) validate() error {
	if len(o.contextName) == 0 {
		return errors.New("empty context names are not allowed")
	}

	config, err := o.configAccess.GetStartingConfig()
	if err != nil {
		return err
	}
	o.config = *config

	for name := range config.Contexts {
		if name == o.contextName {
			return nil
		}
	}

	return fmt.Errorf("no context exists with the name: %q", o.contextName)
}

func (o *useContextOptions) run() error {
	o.config.CurrentContext = o.contextName

	return clientcmd.ModifyConfig(o.configAccess, o.config, true)
}
