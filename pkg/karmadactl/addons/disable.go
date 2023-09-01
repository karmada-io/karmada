package addons

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	addoninit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
)

var (
	disableExample = templates.Examples(`
	# Disable Karmada all addons except karmada-scheduler-estimator on Kubernetes cluster
	%[1]s disable all
	
	# Disable Karmada search on Kubernetes cluster
	%[1]s disable karmada-search
	
	# Disable Karmada search and descheduler on Kubernetes cluster
	%[1]s disable karmada-search karmada-descheduler
	
	# Disable karmada search and scheduler-estimator of member1 cluster to the kubernetes cluster
	%[1]s disable karmada-search karmada-scheduler-estimator --cluster member1

	# Specify the host cluster kubeconfig
	%[1]s disable Karmada-search --kubeconfig /root/.kube/config

	# Specify the Karmada control plane kubeconfig
	%[1]s disable karmada-search --karmada-kubeconfig /etc/karmada/karmada-apiserver.config

	# Specify the namespace where Karmada components are installed
	%[1]s disable karmada-search --namespace karmada-system
	`)
)

// NewCmdAddonsDisable disable Karmada addons on Kubernetes
func NewCmdAddonsDisable(parentCommand string) *cobra.Command {
	opts := addoninit.CommandAddonsDisableOption{}
	cmd := &cobra.Command{
		Use:                   "disable",
		Short:                 "Disable karmada addons from Kubernetes",
		Long:                  "Disable Karmada addons from Kubernetes",
		Example:               fmt.Sprintf(disableExample, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(); err != nil {
				return err
			}
			if err := opts.Validate(args); err != nil {
				return err
			}
			if err := opts.Run(args); err != nil {
				return err
			}
			return nil
		},
	}
	flags := cmd.Flags()
	opts.GlobalCommandOptions.AddFlags(flags)
	flags.BoolVarP(&opts.Force, "force", "f", false, "Disable addons without prompting for confirmation.")
	return cmd
}
