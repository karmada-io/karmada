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
	"io"
	"sort"
	"strings"

	"k8s.io/cli-runtime/pkg/printers"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	getContextColumns = []string{"CURRENT", "NAME", "CLUSTER", "AUTHINFO", "NAMESPACE"}
)

type ConfigCmdPrinter struct {
	out io.Writer
}

func NewConfigCmdPrinter(out io.Writer) *ConfigCmdPrinter {
	return &ConfigCmdPrinter{out: out}
}

func (p *ConfigCmdPrinter) printGetContexts(names []string, config *clientcmdapi.Config, showHeaders, nameOnly bool) error {
	w := printers.GetNewTabWriter(p.out)
	defer w.Flush()

	if showHeaders {
		err := printContextHeaders(w, nameOnly)
		if err != nil {
			return err
		}
	}

	sort.Strings(names)
	for _, name := range names {
		err := printContext(name, config.Contexts[name], w, nameOnly, config.CurrentContext == name)
		if err != nil {
			return err
		}
	}

	return nil
}

func printContextHeaders(out io.Writer, nameOnly bool) error {
	columnNames := getContextColumns
	if nameOnly {
		columnNames = columnNames[:1]
	}
	_, err := fmt.Fprintf(out, "%s\n", strings.Join(columnNames, "\t"))
	return err
}

func printContext(name string, context *clientcmdapi.Context, w io.Writer, nameOnly, current bool) error {
	if nameOnly {
		_, err := fmt.Fprintf(w, "%s\n", name)
		return err
	}
	prefix := " "
	if current {
		prefix = "*"
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", prefix, name, context.Cluster, context.AuthInfo, context.Namespace)
	return err
}
