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
