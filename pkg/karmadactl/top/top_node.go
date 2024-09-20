/*
Copyright 2024 The Karmada Authors.

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

package top

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	metricsapi "k8s.io/metrics/pkg/apis/metrics"
	metricsV1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

// NodeOptions contains all the options for running the top-node cli command.
type NodeOptions struct {
	Clusters           []string
	ResourceName       string
	Selector           string
	SortBy             string
	NoHeaders          bool
	UseProtocolBuffers bool
	ShowCapacity       bool

	Printer       *CmdPrinter
	karmadaClient karmadaclientset.Interface

	metrics            *metricsapi.NodeMetricsList
	availableResources map[string]map[string]corev1.ResourceList

	lock sync.Mutex
	errs []error

	genericiooptions.IOStreams
}

var (
	topNodeLong = templates.LongDesc(i18n.T(`
                Display resource (CPU/memory) usage of nodes in member clusters.

                The top-node command allows you to see the resource consumption of nodes.`))

	topNodeExample = templates.Examples(i18n.T(`
				# Show metrics for all nodes in the default namespace
				%[1]s top node

				# Show metrics for all nodes in the default namespace in member1 cluster
				%[1]s top node --clusters=member1

				# Show metrics for all nodes in the default namespace in member1 and member2 cluster
				%[1]s top node --clusters=member1,member2

				# Show metrics for all nodes in the given namespace
				%[1]s top node --namespace=NAMESPACE

				# Show metrics for the nodes defined by label name=myLabel
				%[1]s top node -l name=myLabel`))
)

// NewCmdTopNode implements the top node command.
func NewCmdTopNode(f util.Factory, parentCommand string, o *NodeOptions, streams genericclioptions.IOStreams) *cobra.Command {
	if o == nil {
		o = &NodeOptions{
			IOStreams:          streams,
			UseProtocolBuffers: true,
		}
	}

	cmd := &cobra.Command{
		Use:                   "node [NAME | -l label]",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Display resource (CPU/memory) usage of nodes"),
		Long:                  topNodeLong,
		Example:               fmt.Sprintf(topNodeExample, parentCommand),
		ValidArgsFunction:     utilcomp.ResourceNameCompletionFunc(f, "node"),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunTopNode(f))
		},
		Aliases: []string{"nodes", "no"},
		Annotations: map[string]string{
			"parent": "top", // used for completion code to set default operation scope.
		},
	}
	cmdutil.AddLabelSelectorFlagVar(cmd, &o.Selector)
	options.AddKubeConfigFlags(cmd.Flags())
	cmd.Flags().StringVar(&o.SortBy, "sort-by", o.SortBy, "If non-empty, sort nodes list using specified field. The field can be either 'cpu' or 'memory'.")
	cmd.Flags().StringSliceVar(&o.Clusters, "clusters", []string{}, "Used to specify target member clusters, for example: --clusters=member1,member2")
	cmd.Flags().BoolVar(&o.NoHeaders, "no-headers", o.NoHeaders, "If present, print output without headers")
	cmd.Flags().BoolVar(&o.UseProtocolBuffers, "use-protocol-buffers", o.UseProtocolBuffers, "Enables using protocol-buffers to access Metrics API.")
	cmd.Flags().BoolVar(&o.ShowCapacity, "show-capacity", o.ShowCapacity, "Print node resources based on Capacity instead of Allocatable(default) of the nodes.")

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForClustersFlag(cmd)
	return cmd
}

// Complete completes all the required options.
func (o *NodeOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	if len(args) == 1 {
		o.ResourceName = args[0]
	} else if len(args) > 1 {
		return cmdutil.UsageErrorf(cmd, "%s", cmd.Use)
	}

	o.Printer = NewTopCmdPrinter(o.Out)
	o.lock = sync.Mutex{}
	o.errs = make([]error, 0)

	karmadaClient, err := f.KarmadaClientSet()
	if err != nil {
		return err
	}
	o.karmadaClient = karmadaClient

	o.metrics = &metricsapi.NodeMetricsList{}
	o.availableResources = make(map[string]map[string]corev1.ResourceList)
	return nil
}

// Validate checks the validity of the options.
func (o *NodeOptions) Validate() error {
	if len(o.SortBy) > 0 {
		if o.SortBy != sortByCPU && o.SortBy != sortByMemory {
			return errors.New("--sort-by accepts only cpu or memory")
		}
	}
	if len(o.ResourceName) > 0 && len(o.Selector) > 0 {
		return errors.New("only one of NAME or --selector can be provided")
	}

	if len(o.Clusters) > 0 {
		clusters, err := o.karmadaClient.ClusterV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		return util.VerifyClustersExist(o.Clusters, clusters)
	}

	return nil
}

// RunTopNode runs the top node command.
func (o *NodeOptions) RunTopNode(f util.Factory) error {
	var err error
	var wg sync.WaitGroup

	selector := labels.Everything()
	if len(o.Selector) > 0 {
		selector, err = labels.Parse(o.Selector)
		if err != nil {
			return err
		}
	}

	o.Clusters, err = GenClusterList(o.karmadaClient, o.Clusters)
	if err != nil {
		return err
	}

	wg.Add(len(o.Clusters))
	for _, cluster := range o.Clusters {
		go func(f util.Factory, cluster string, selector labels.Selector) {
			defer wg.Done()
			o.runTopNodePerCluster(f, cluster, selector)
		}(f, cluster, selector)
	}
	wg.Wait()

	if len(o.metrics.Items) == 0 && len(o.errs) == 0 {
		// if we had no errors, be sure we output something.
		fmt.Fprintln(o.ErrOut, "No resources found")
	}

	err = o.Printer.PrintNodeMetrics(o.metrics.Items, o.availableResources, o.NoHeaders, o.SortBy)
	if err != nil {
		o.errs = append(o.errs, err)
	}

	return utilerrors.NewAggregate(o.errs)
}

func (o *NodeOptions) runTopNodePerCluster(f util.Factory, cluster string, selector labels.Selector) {
	var err error
	defer func() {
		if err != nil {
			o.lock.Lock()
			o.errs = append(o.errs, fmt.Errorf("cluster(%s): %s", cluster, err))
			o.lock.Unlock()
		}
	}()

	clusterClient, metricsClient, err := GetMemberAndMetricsClientSet(f, cluster, o.UseProtocolBuffers)
	if err != nil {
		return
	}

	metrics, err := getNodeMetricsFromMetricsAPI(metricsClient, o.ResourceName, selector)
	if err != nil {
		return
	}

	if len(metrics.Items) == 0 {
		err = fmt.Errorf("metrics not available yet")
		return
	}

	nodes, err := getNodes(clusterClient, selector, o.ResourceName)
	if err != nil {
		return
	}

	o.lock.Lock()
	for _, item := range metrics.Items {
		if item.Annotations == nil {
			item.Annotations = map[string]string{autoscalingv1alpha1.QuerySourceAnnotationKey: cluster}
		} else {
			item.Annotations[autoscalingv1alpha1.QuerySourceAnnotationKey] = cluster
		}
		o.metrics.Items = append(o.metrics.Items, item)
	}

	resourceMap := make(map[string]corev1.ResourceList, len(nodes))
	for _, n := range nodes {
		if !o.ShowCapacity {
			resourceMap[n.Name] = n.Status.Allocatable
		} else {
			resourceMap[n.Name] = n.Status.Capacity
		}
	}
	o.availableResources[cluster] = resourceMap
	o.lock.Unlock()
}

func getNodeMetricsFromMetricsAPI(metricsClient *metricsclientset.Clientset, resourceName string, selector labels.Selector) (*metricsapi.NodeMetricsList, error) {
	var err error
	versionedMetrics := &metricsV1beta1api.NodeMetricsList{}
	mc := metricsClient.MetricsV1beta1()
	nm := mc.NodeMetricses()
	if resourceName != "" {
		m, err := nm.Get(context.TODO(), resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		versionedMetrics.Items = []metricsV1beta1api.NodeMetrics{*m}
	} else {
		versionedMetrics, err = nm.List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return nil, err
		}
	}
	metrics := &metricsapi.NodeMetricsList{}
	err = metricsV1beta1api.Convert_v1beta1_NodeMetricsList_To_metrics_NodeMetricsList(versionedMetrics, metrics, nil)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func getNodes(clusterClient *kubernetes.Clientset, selector labels.Selector, resourceName string) ([]corev1.Node, error) {
	var nodes []corev1.Node
	if len(resourceName) > 0 {
		node, err := clusterClient.CoreV1().Nodes().Get(context.TODO(), resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, *node)
	} else {
		nodeList, err := clusterClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, nodeList.Items...)
	}
	return nodes, nil
}
