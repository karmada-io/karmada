package addons

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	addoninit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	"github.com/karmada-io/karmada/pkg/version"
)

var (
	enableExample = templates.Examples(`
	# Enable Karmada all addons except karmada-scheduler-estimator to Kubernetes cluster 
	%[1]s enable all
	
	# Enable Karmada search to the Kubernetes cluster
	%[1]s enable karmada-search
	
	# Enable karmada search and descheduler to the kubernetes cluster
	%[1]s enable karmada-search karmada-descheduler

	# Enable karmada search and scheduler-estimator of cluster member1 to the kubernetes cluster
	%[1]s enable karmada-search karmada-scheduler-estimator -C member1 --member-kubeconfig /etc/karmada/member.config --member-context member1

	# Specify the host cluster kubeconfig
	%[1]s enable karmada-search --kubeconfig /root/.kube/config

	# Specify the Karmada control plane kubeconfig
	%[1]s enable karmada-search --karmada-kubeconfig /etc/karmada/karmada-apiserver.config

	# Specify the karmada-search image
	%[1]s enable karmada-search --karmada-search-image docker.io/karmada/karmada-scheduler-estimator:latest

	# Sepcify the namespace where Karmada components are installed
	%[1]s enable karmada-search --namespace karmada-system
	`)
)

// NewCmdAddonsEnable enable Karmada addons on Kubernetes
func NewCmdAddonsEnable(parentCommand string) *cobra.Command {
	opts := addoninit.CommandAddonsEnableOption{}
	cmd := &cobra.Command{
		Use:                   "enable",
		Short:                 "Enable Karmada addons from Kubernetes",
		Long:                  "Enable Karmada addons from Kubernetes",
		Example:               fmt.Sprintf(enableExample, parentCommand),
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

	releaseVer, err := version.ParseGitVersion(version.Get().GitVersion)
	if err != nil {
		klog.Infof("No default release version found. build version: %s", version.Get().String())
		releaseVer = &version.ReleaseVersion{} // initialize to avoid panic
	}

	flags := cmd.Flags()
	opts.GlobalCommandOptions.AddFlags(flags)
	flags.IntVar(&opts.WaitComponentReadyTimeout, "pod-timeout", options.WaitComponentReadyTimeout, "Wait pod ready timeout.")
	flags.IntVar(&opts.WaitAPIServiceReadyTimeout, "apiservice-timeout", 30, "Wait apiservice ready timeout.")
	flags.StringVar(&opts.KarmadaSearchImage, "karmada-search-image", fmt.Sprintf("docker.io/karmada/karmada-search:%s", releaseVer.PatchRelease()), "karmada search image")
	flags.Int32Var(&opts.KarmadaSearchReplicas, "karmada-search-replicas", 1, "Karmada search replica set")
	flags.StringVar(&opts.KarmadaDeschedulerImage, "karmada-descheduler-image", fmt.Sprintf("docker.io/karmada/karmada-descheduler:%s", releaseVer.PatchRelease()), "karmada descheduler image")
	flags.Int32Var(&opts.KarmadaDeschedulerReplicas, "karmada-descheduler-replicas", 1, "Karmada descheduler replica set")
	flags.StringVar(&opts.KarmadaSchedulerEstimatorImage, "karmada-scheduler-estimator-image", fmt.Sprintf("docker.io/karmada/karmada-scheduler-estimator:%s", releaseVer.PatchRelease()), "karmada scheduler-estimator image")
	flags.Int32Var(&opts.KarmadaEstimatorReplicas, "karmada-estimator-replicas", 1, "Karmada scheduler estimator replica set")
	flags.StringVar(&opts.MemberKubeConfig, "member-kubeconfig", "", "Member cluster's kubeconfig which to deploy scheduler estimator")
	flags.StringVar(&opts.MemberContext, "member-context", "", "Member cluster's context which to deploy scheduler estimator")
	return cmd
}
