/*
Copyright 2023 The Karmada Authors.

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
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	metricsapi "k8s.io/metrics/pkg/apis/metrics"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

// PodOptions contains the options to the top command.
type PodOptions struct {
	ResourceName       string
	Namespace          string
	Clusters           []string
	LabelSelector      string
	FieldSelector      string
	SortBy             string
	AllNamespaces      bool
	PrintContainers    bool
	NoHeaders          bool
	UseProtocolBuffers bool
	Sum                bool
	lock               sync.Mutex
	errs               []error

	Printer       *CmdPrinter
	metrics       *metricsapi.PodMetricsList
	karmadaClient karmadaclientset.Interface

	genericiooptions.IOStreams
}

const metricsCreationDelay = 2 * time.Minute

var (
	topPodLong = templates.LongDesc(i18n.T(`
		Display resource (CPU/memory) usage of pods.

		The 'top pod' command allows you to see the resource consumption of pods of member clusters.

		Due to the metrics pipeline delay, they may be unavailable for a few minutes
		since pod creation.`))

	topPodExample = templates.Examples(i18n.T(`
		# Show metrics for all pods in the default namespace
		%[1]s top pod

		# Show metrics for all pods in the default namespace in member1 cluster
		%[1]s top pod --clusters=member1

		# Show metrics for all pods in the default namespace in member1 and member2 cluster
		%[1]s top pod --clusters=member1,member2

		# Show metrics for all pods in the given namespace
		%[1]s top pod --namespace=NAMESPACE

		# Show metrics for a given pod and its containers
		%[1]s top pod POD_NAME --containers

		# Show metrics for the pods defined by label name=myLabel
		%[1]s top pod -l name=myLabel`))
)

// NewCmdTopPod implements the top pod command.
func NewCmdTopPod(f util.Factory, parentCommand string, o *PodOptions, streams genericiooptions.IOStreams) *cobra.Command {
	if o == nil {
		o = &PodOptions{
			IOStreams:          streams,
			UseProtocolBuffers: true,
		}
	}

	cmd := &cobra.Command{
		Use:                   "pod [NAME | -l label]",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Display resource (CPU/memory) usage of pods of member clusters"),
		Long:                  topPodLong,
		Example:               fmt.Sprintf(topPodExample, parentCommand),
		ValidArgsFunction:     utilcomp.ResourceNameCompletionFunc(f, "pod"),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunTopPod(f))
		},
		Aliases: []string{"pods", "po"},
		Annotations: map[string]string{
			"parent": "top", // used for completion code to set default operation scope.
		},
	}
	cmdutil.AddLabelSelectorFlagVar(cmd, &o.LabelSelector)
	options.AddKubeConfigFlags(cmd.Flags())
	options.AddNamespaceFlag(cmd.Flags())
	cmd.Flags().StringSliceVarP(&o.Clusters, "clusters", "C", []string{}, "-C=member1,member2")
	cmd.Flags().StringVar(&o.FieldSelector, "field-selector", o.FieldSelector, "Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.")
	cmd.Flags().StringVar(&o.SortBy, "sort-by", o.SortBy, "If non-empty, sort pods list using specified field. The field can be either 'cpu' or 'memory'.")
	cmd.Flags().BoolVar(&o.PrintContainers, "containers", o.PrintContainers, "If present, print usage of containers within a pod.")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().BoolVar(&o.NoHeaders, "no-headers", o.NoHeaders, "If present, print output without headers.")
	cmd.Flags().BoolVar(&o.UseProtocolBuffers, "use-protocol-buffers", o.UseProtocolBuffers, "Enables using protocol-buffers to access Metrics API.")
	cmd.Flags().BoolVar(&o.Sum, "sum", o.Sum, "Print the sum of the resource usage")

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(cmd, f)
	utilcomp.RegisterCompletionFuncForClustersFlag(cmd)
	return cmd
}

// Complete completes all the required options.
func (o *PodOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	var err error
	if len(args) == 1 {
		o.ResourceName = args[0]
	} else if len(args) > 1 {
		return cmdutil.UsageErrorf(cmd, "%s", cmd.Use)
	}

	o.lock = sync.Mutex{}
	o.errs = make([]error, 0)

	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.metrics = &metricsapi.PodMetricsList{}
	o.Printer = NewTopCmdPrinter(o.Out)

	karmadaClient, err := f.KarmadaClientSet()
	if err != nil {
		return err
	}
	o.karmadaClient = karmadaClient

	return nil
}

// Validate checks the validity of the options.
func (o *PodOptions) Validate() error {
	if len(o.SortBy) > 0 {
		if o.SortBy != sortByCPU && o.SortBy != sortByMemory {
			return errors.New("--sort-by accepts only cpu or memory")
		}
	}
	if len(o.ResourceName) > 0 && (len(o.LabelSelector) > 0 || len(o.FieldSelector) > 0) {
		return errors.New("only one of NAME or selector can be provided")
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

// RunTopPod runs the top pod command.
func (o *PodOptions) RunTopPod(f util.Factory) error {
	var err error
	var wg sync.WaitGroup

	labelSelector := labels.Everything()
	if len(o.LabelSelector) > 0 {
		labelSelector, err = labels.Parse(o.LabelSelector)
		if err != nil {
			return err
		}
	}
	fieldSelector := fields.Everything()
	if len(o.FieldSelector) > 0 {
		fieldSelector, err = fields.ParseSelector(o.FieldSelector)
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
		go func(f util.Factory,
			cluster string, labelSelector labels.Selector, fieldSelector fields.Selector) {
			defer wg.Done()
			o.runTopPodPerCluster(f, cluster, labelSelector, fieldSelector)
		}(f, cluster, labelSelector, fieldSelector)
	}
	wg.Wait()

	if len(o.metrics.Items) == 0 && len(o.errs) == 0 {
		// if we had no errors, be sure we output something.
		if o.AllNamespaces {
			fmt.Fprintln(o.ErrOut, "No resources found")
		} else {
			fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		}
	}

	err = o.Printer.PrintPodMetrics(o.metrics.Items, o.PrintContainers, o.AllNamespaces, o.NoHeaders, o.SortBy, o.Sum)
	if err != nil {
		o.errs = append(o.errs, err)
	}

	return utilerrors.NewAggregate(o.errs)
}

func (o *PodOptions) runTopPodPerCluster(f util.Factory,
	cluster string, labelSelector labels.Selector, fieldSelector fields.Selector) {
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
	metrics, err := getMetricsFromMetricsAPI(metricsClient, o.Namespace, o.ResourceName, o.AllNamespaces, labelSelector, fieldSelector)
	if err != nil {
		return
	}
	// First we check why no metrics have been received
	if len(metrics.Items) == 0 {
		// If the API server query is successful but all the pods are newly created,
		// the metrics are probably not ready yet, so we return the error here in the first place.
		err = verifyEmptyMetrics(o, clusterClient, labelSelector, fieldSelector)
		if err != nil {
			return
		}
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
	o.lock.Unlock()
}

func getMetricsFromMetricsAPI(metricsClient *metricsclientset.Clientset, namespace, resourceName string, allNamespaces bool, labelSelector labels.Selector, fieldSelector fields.Selector) (*metricsapi.PodMetricsList, error) {
	var err error
	ns := metav1.NamespaceAll
	if !allNamespaces {
		ns = namespace
	}
	versionedMetrics := &metricsv1beta1api.PodMetricsList{}
	if len(resourceName) != 0 {
		m, err := metricsClient.MetricsV1beta1().PodMetricses(ns).Get(context.TODO(), resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		versionedMetrics.Items = []metricsv1beta1api.PodMetrics{*m}
	} else {
		versionedMetrics, err = metricsClient.MetricsV1beta1().PodMetricses(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String(), FieldSelector: fieldSelector.String()})
		if err != nil {
			return nil, err
		}
	}
	metrics := &metricsapi.PodMetricsList{}
	err = metricsv1beta1api.Convert_v1beta1_PodMetricsList_To_metrics_PodMetricsList(versionedMetrics, metrics, nil)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func verifyEmptyMetrics(o *PodOptions, clusterClient *kubernetes.Clientset, labelSelector labels.Selector, fieldSelector fields.Selector) error {
	if len(o.ResourceName) > 0 {
		pod, err := clusterClient.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.ResourceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if err = checkPodAge(pod); err != nil {
			return err
		}
	} else {
		pods, err := clusterClient.CoreV1().Pods(o.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector.String(),
			FieldSelector: fieldSelector.String(),
		})
		if err != nil {
			return err
		}
		if len(pods.Items) == 0 {
			return nil
		}
		for i := range pods.Items {
			if err = checkPodAge(&pods.Items[i]); err != nil {
				return err
			}
		}
	}
	return errors.New("metrics not available yet")
}

func checkPodAge(pod *corev1.Pod) error {
	age := time.Since(pod.CreationTimestamp.Time)
	if age > metricsCreationDelay {
		message := fmt.Sprintf("Metrics not available for pod %s/%s, age: %s", pod.Namespace, pod.Name, age.String())
		return errors.New(message)
	}
	klog.V(2).Infof("Metrics not yet available for pod %s/%s, age: %s", pod.Namespace, pod.Name, age.String())
	return nil
}
