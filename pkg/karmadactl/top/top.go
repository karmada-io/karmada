package top

import (
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	metricsapi "k8s.io/metrics/pkg/apis/metrics"
)

const (
	sortByCPU    = "cpu"
	sortByMemory = "memory"
)

var (
	supportedMetricsAPIVersions = []string{
		"v1beta1",
	}
	topLong = templates.LongDesc(`
		Display Resource (CPU/Memory) usage of member clusters.

		The top command allows you to see the resource consumption for pods of member clusters.

		This command requires karmada-metrics-adapter to be correctly configured and working on the Karmada control plane and 
		Metrics Server to be correctly configured and working on the member clusters.`)
)

func NewCmdTop(f cmdutil.Factory, parentCommand string, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Display resource (CPU/memory) usage of member clusters",
		Long:  topLong,
		Run:   cmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	// create subcommands
	cmd.AddCommand(NewCmdTopPod(f, parentCommand, nil, streams))

	return cmd
}

func SupportedMetricsAPIVersionAvailable(discoveredAPIGroups *metav1.APIGroupList) bool {
	for _, discoveredAPIGroup := range discoveredAPIGroups.Groups {
		if discoveredAPIGroup.Name != metricsapi.GroupName {
			continue
		}
		for _, version := range discoveredAPIGroup.Versions {
			for _, supportedVersion := range supportedMetricsAPIVersions {
				if version.Version == supportedVersion {
					return true
				}
			}
		}
	}
	return false
}
