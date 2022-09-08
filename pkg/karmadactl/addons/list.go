package addons

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	addoninit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
)

var (
	listExample = templates.Examples(`
	# List Karmada all addons installed in Kubernetes cluster 
	%[1]s list

	# List Karmada all addons included scheduler estimator of member1 installed in Kubernetes cluster 
	%[1]s list -C member1
	
	# Specify the host cluster kubeconfig
	%[1]s list --kubeconfig /root/.kube/config
	
	# Specify the karmada control plane kubeconfig
	%[1]s list --karmada-kubeconfig /etc/karmada/karmada-apiserver.config
	
	# Sepcify the namespace where Karmada components are installed
	%[1]s list --namespace karmada-system
	`)
)

// NewCmdAddonsList list Karmada addons on Kubernetes
func NewCmdAddonsList(parentCommand string) *cobra.Command {
	opts := addoninit.CommandAddonsListOption{}
	cmd := &cobra.Command{
		Use:                   "list",
		Short:                 "List karmada addons from Kubernetes",
		Long:                  "List Karmada addons from Kubernetes",
		Example:               fmt.Sprintf(listExample, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(); err != nil {
				return err
			}
			if err := opts.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	opts.GlobalCommandOptions.AddFlags(cmd.Flags())
	return cmd
}
