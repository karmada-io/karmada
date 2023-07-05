package options

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	optionsExample = templates.Examples(`
		# Print flags inherited by all commands
		%[1]s options`)
)

// NewCmdOptions implements the options command
func NewCmdOptions(parentCommand string, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "options",
		Short:   "Print the list of flags inherited by all commands",
		Long:    "Print the list of flags inherited by all commands",
		Example: fmt.Sprintf(optionsExample, parentCommand),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Usage(); err != nil {
				return err
			}
			return nil
		},
	}

	// The `options` command needs write its output to the `out` stream
	// (typically stdout). Without calling SetOutput here, the Usage()
	// function call will fall back to stderr.
	//
	// See https://github.com/kubernetes/kubernetes/pull/46394 for details.
	cmd.SetOut(out)
	cmd.SetErr(out)

	templates.UseOptionsTemplates(cmd)
	return cmd
}
