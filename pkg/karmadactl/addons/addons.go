package addons

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/addons/install"
)

var (
	addonsExamples = templates.Examples(`
	# Enable or disable Karmada addons to the karmada-host cluster
	%[1]s addons enable karmada-search
	`)
)

func init() {
	install.Install()
}

// NewCommandAddons enable or disable Karmada addons on karmada-host cluster
func NewCommandAddons(parentCommand string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "addons",
		Short:   "Enable or disable a Karmada addon",
		Long:    "Enable or disable a Karmada addon",
		Example: fmt.Sprintf(addonsExamples, parentCommand),
	}

	addonsParentCommand := fmt.Sprintf("%s %s", parentCommand, "addons")
	cmd.AddCommand(NewCmdAddonsList(addonsParentCommand))
	cmd.AddCommand(NewCmdAddonsEnable(addonsParentCommand))
	cmd.AddCommand(NewCmdAddonsDisable(addonsParentCommand))

	return cmd
}
