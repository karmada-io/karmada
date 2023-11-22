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
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

// holds the command-line options for 'config current-context' sub command
type currentContextOptions struct {
	configAccess clientcmd.ConfigAccess
	streams      genericclioptions.IOStreams
}

var (
	currentContextLong = templates.LongDesc(i18n.T(`
		Display the current-context.`))

	currentContextExample = templates.Examples(`
		# Display the current-context
		%[1]s config current-context`)
)

// NewCmdConfigCurrentContext returns a Command instance for 'config current-context' sub command
func NewCmdConfigCurrentContext(parentCommand string, streams genericclioptions.IOStreams, configAccess clientcmd.ConfigAccess) *cobra.Command {
	options := &currentContextOptions{
		configAccess: configAccess,
		streams:      streams,
	}

	cmd := &cobra.Command{
		Use:     "current-context",
		Short:   i18n.T("Display the current-context"),
		Long:    currentContextLong,
		Example: fmt.Sprintf(currentContextExample, parentCommand),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(options.run())
		},
	}

	return cmd
}

func (o *currentContextOptions) run() error {
	config, err := o.configAccess.GetStartingConfig()
	if err != nil {
		return err
	}

	if config.CurrentContext == "" {
		err = fmt.Errorf("current-context is not set")
		return err
	}

	fmt.Fprintf(o.streams.Out, "%s\n", config.CurrentContext)
	return nil
}
