/*
Copyright The Karmada Authors.

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

package sharedcommand

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/karmada-io/karmada/pkg/version"
)

var (
	versionShort   = `Print the version information.`
	versionLong    = `Print the version information.`
	versionExample = `  # Print %s command version
  %s version`
)

// NewCmdVersion prints out the release version info for this command binary.
// It is used as a subcommand of a parent command.
func NewCmdVersion(out io.Writer, parentCommand string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Short:   versionShort,
		Long:    versionLong,
		Example: fmt.Sprintf(versionExample, parentCommand, parentCommand),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(out, "%s version: %s\n", parentCommand, version.Get())
		},
	}

	return cmd
}
