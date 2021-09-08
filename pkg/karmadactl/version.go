package karmadactl

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/karmada-io/karmada/pkg/version"
)

var (
	versionShort   = `Print the version info`
	versionLong    = `Version prints the version info of this command.`
	versionExample = `  # Print %s command version
  %s version`
)

// NewCmdVersion prints out the release version info for this command binary.
func NewCmdVersion(out io.Writer, cmdStr string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Short:   versionShort,
		Long:    versionLong,
		Example: fmt.Sprintf(versionExample, cmdStr, cmdStr),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(out, "%s version: %s\n", cmdStr, fmt.Sprintf("%#v", version.Get()))
		},
	}

	return cmd
}
