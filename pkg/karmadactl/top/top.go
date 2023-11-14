package top

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	metricsapi "k8s.io/metrics/pkg/apis/metrics"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
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

func NewCmdTop(f util.Factory, parentCommand string, streams genericclioptions.IOStreams) *cobra.Command {
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

func GenClusterList(clientSet karmadaclientset.Interface, clusters []string) ([]string, error) {
	if len(clusters) != 0 {
		return clusters, nil
	}

	clusterList, err := clientSet.ClusterV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list all member clusters in control plane, err: %w", err)
	}

	for i := range clusterList.Items {
		clusters = append(clusters, clusterList.Items[i].Name)
	}
	return clusters, nil
}

func GetMemberAndMetricsClientSet(f util.Factory,
	cluster string, useProtocolBuffers bool) (*kubernetes.Clientset, *metricsclientset.Clientset, error) {
	memberFactory, err := f.FactoryForMemberCluster(cluster)
	if err != nil {
		return nil, nil, err
	}
	clientset, err := memberFactory.KubernetesClientSet()
	if err != nil {
		return nil, nil, err
	}
	discoveryClient := clientset.DiscoveryClient
	apiGroups, err := discoveryClient.ServerGroups()
	if err != nil {
		return nil, nil, err
	}
	metricsAPIAvailable := SupportedMetricsAPIVersionAvailable(apiGroups)
	if !metricsAPIAvailable {
		return nil, nil, fmt.Errorf("Metrics API not available")
	}

	config, err := memberFactory.ToRESTConfig()
	if err != nil {
		return nil, nil, err
	}
	if useProtocolBuffers {
		config.ContentType = "application/vnd.kubernetes.protobuf"
	}
	metricsClient, err := metricsclientset.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return clientset, metricsClient, nil
}
