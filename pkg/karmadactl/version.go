package karmadactl

import (
	"fmt"
	"io"

	"github.com/karmada-io/karmada/pkg/version"
	"github.com/spf13/cobra"
)

var (
	versionLong = `Version prints the version info of this command.`

	versionExample = `
		# Print karmadactl command version
		karmadactl version`
)

// NewCmdVersion prints out the release version info for this command binary.
func NewCmdVersion(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Print the version info",
		Long:    versionLong,
		Example: versionExample,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(out, "karmadactl version: %s\n", fmt.Sprintf("%#v", version.Get()))
		},
	}

	return cmd
}
