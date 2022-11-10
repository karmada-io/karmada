package addons

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/addons/install"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
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

// NewCmdAddons enable or disable Karmada addons on karmada-host cluster
func NewCmdAddons(parentCommand string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "addons",
		Short:   "Enable or disable a Karmada addon",
		Long:    "Enable or disable a Karmada addon",
		Example: fmt.Sprintf(addonsExamples, parentCommand),
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterRegistration,
		},
	}

	addonsParentCommand := fmt.Sprintf("%s %s", parentCommand, "addons")
	cmd.AddCommand(NewCmdAddonsList(addonsParentCommand))
	cmd.AddCommand(NewCmdAddonsEnable(addonsParentCommand))
	cmd.AddCommand(NewCmdAddonsDisable(addonsParentCommand))

	return cmd
}
