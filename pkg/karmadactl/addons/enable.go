/*
Copyright 2022 The Karmada Authors.

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

package addons

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	addoninit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	globaloptions "github.com/karmada-io/karmada/pkg/karmadactl/options"
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
	%[1]s enable karmada-search --karmada-search-image docker.io/karmada/karmada-search:latest

	# Specify the namespace where Karmada components are installed
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
		RunE: func(_ *cobra.Command, args []string) error {
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
	flags.StringVarP(&opts.ImageRegistry, "private-image-registry", "", "", "Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.")
	flags.IntVar(&opts.WaitComponentReadyTimeout, "pod-timeout", options.WaitComponentReadyTimeout, "Wait pod ready timeout.")
	flags.IntVar(&opts.WaitAPIServiceReadyTimeout, "apiservice-timeout", 30, "Wait apiservice ready timeout.")
	flags.StringVar(&opts.MemberKubeConfig, "member-kubeconfig", "", "Member cluster's kubeconfig which to deploy scheduler estimator")
	flags.StringVar(&opts.MemberContext, "member-context", "", "Member cluster's context which to deploy scheduler estimator")
	flags.StringVar(&opts.HostClusterDomain, "host-cluster-domain", globaloptions.DefaultHostClusterDomain, "The cluster domain of karmada host cluster. (e.g. --host-cluster-domain=host.karmada)")
	// Karmada-descheduler config
	flags.StringVar(&opts.KarmadaDeschedulerImage, "karmada-descheduler-image", addoninit.DefaultKarmadaDeschedulerImage, "karmada-descheduler image")
	flags.Int32Var(&opts.KarmadaDeschedulerReplicas, "karmada-descheduler-replicas", 1, "karmada descheduler replica set")
	flags.StringVar(&opts.KarmadaDeschedulerPriorityClass, "descheduler-priority-class", "system-node-critical", "The priority class name for the component karmada-descheduler.")

	// Karmada-estimator config
	flags.StringVar(&opts.KarmadaSchedulerEstimatorImage, "karmada-scheduler-estimator-image", addoninit.DefaultKarmadaSchedulerEstimatorImage, "karmada-scheduler-estimator image")
	flags.Int32Var(&opts.KarmadaEstimatorReplicas, "karmada-estimator-replicas", 1, "karmada-scheduler-estimator replica set")
	flags.StringVar(&opts.EstimatorPriorityClass, "estimator-priority-class", "system-node-critical", "The priority class name for the component karmada-scheduler-estimator.")

	// Karmada-metrics-adapter config
	flags.Int32Var(&opts.KarmadaMetricsAdapterReplicas, "karmada-metrics-adapter-replicas", 1, "karmada-metrics-adapter replica set")
	flags.StringVar(&opts.KarmadaMetricsAdapterImage, "karmada-metrics-adapter-image", addoninit.DefaultKarmadaMetricsAdapterImage, "karmada-metrics-adapter image")
	flags.StringVar(&opts.KarmadaMetricsAdapterPriorityClass, "metrics-adapter-priority-class", "system-node-critical", "The priority class name for the component karmada-metrics-adaptor.")

	// Karmada-search config
	flags.Int32Var(&opts.KarmadaSearchReplicas, "karmada-search-replicas", 1, "Karmada-search replica set")
	flags.StringVar(&opts.KarmadaSearchImage, "karmada-search-image", addoninit.DefaultKarmadaSearchImage, "karmada-search image")
	flags.StringVar(&opts.SearchPriorityClass, "search-priority-class", "system-node-critical", "The priority class name for the component karmada-search.")

	return cmd
}
