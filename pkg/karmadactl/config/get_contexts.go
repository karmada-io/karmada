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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

// contains the assignable options from the args.
type getContextsOptions struct {
	configAccess clientcmd.ConfigAccess
	nameOnly     bool
	showHeaders  bool
	contextNames []string

	outputFormat string
	noHeaders    bool
	print        *ConfigCmdPrinter

	genericclioptions.IOStreams
}

var (
	getContextsLong = templates.LongDesc(i18n.T(`Display one or many contexts from the kubeconfig file.`))

	getContextsExample = templates.Examples(`
		# List all the contexts in your kubeconfig file
		%[1]s config get-contexts

		# Describe one context in your kubeconfig file
		%[1]s config get-contexts my-context`)
)

// NewCmdConfigGetContexts creates a command object for the "get-contexts" action, which
// retrieves one or more contexts from a kubeconfig.
func NewCmdConfigGetContexts(parentCommand string, streams genericclioptions.IOStreams, configAccess clientcmd.ConfigAccess) *cobra.Command {
	options := &getContextsOptions{
		configAccess: configAccess,
		IOStreams:    streams,
	}

	cmd := &cobra.Command{
		Use:                   "get-contexts [(-o|--output=)name)]",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Describe one or many contexts"),
		Long:                  getContextsLong,
		Example:               fmt.Sprintf(getContextsExample, parentCommand),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(options.complete(cmd, args))
			cmdutil.CheckErr(options.runGetContexts())
		},
	}

	cmd.Flags().BoolVar(&options.noHeaders, "no-headers", options.noHeaders, "When using the default or custom-column output format, don't print headers (default print headers).")
	cmd.Flags().StringVarP(&options.outputFormat, "output", "o", options.outputFormat, `Output format. One of: (name).`)
	return cmd
}

// assigns getContextsOptions from the args.
func (o *getContextsOptions) complete(cmd *cobra.Command, args []string) error {
	supportedOutputTypes := sets.NewString("", "name")
	if !supportedOutputTypes.Has(o.outputFormat) {
		return fmt.Errorf("--output %v is not available in kubectl config get-contexts; resetting to default output format", o.outputFormat)
	}

	o.contextNames = args
	o.nameOnly = false
	if o.outputFormat == "name" {
		o.nameOnly = true
	}
	o.showHeaders = true
	if cmdutil.GetFlagBool(cmd, "no-headers") || o.nameOnly {
		o.showHeaders = false
	}

	o.print = NewConfigCmdPrinter(o.Out)

	return nil
}

// implements all the necessary functionality for context retrieval.
func (o *getContextsOptions) runGetContexts() error {
	config, err := o.configAccess.GetStartingConfig()
	if err != nil {
		return err
	}

	// Build a list of context names to print, and warn if any requested contexts are not found.
	// Do this before printing the headers, so it doesn't look ugly.
	var allErrs []error
	var toPrint []string
	if len(o.contextNames) == 0 {
		for name := range config.Contexts {
			toPrint = append(toPrint, name)
		}
	} else {
		for _, name := range o.contextNames {
			_, ok := config.Contexts[name]
			if ok {
				toPrint = append(toPrint, name)
			} else {
				allErrs = append(allErrs, fmt.Errorf("context %s not found", name))
			}
		}
	}

	err = o.print.printGetContexts(toPrint, config, o.showHeaders, o.nameOnly)
	if err != nil {
		allErrs = append(allErrs, err)
	}

	return utilerrors.NewAggregate(allErrs)
}
