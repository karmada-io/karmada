package top

import (
	"fmt"
	"io"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/kubectl/pkg/metricsutil"
	metricsapi "k8s.io/metrics/pkg/apis/metrics"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
)

var (
	MeasuredResources = []corev1.ResourceName{
		corev1.ResourceCPU,
		corev1.ResourceMemory,
	}
	PodColumns      = []string{"NAME", "CLUSTER", "CPU(cores)", "MEMORY(bytes)"}
	NamespaceColumn = "NAMESPACE"
	PodColumn       = "POD"
)

type ResourceMetricsInfo struct {
	Cluster   string
	Name      string
	Metrics   corev1.ResourceList
	Available corev1.ResourceList
}

type TopCmdPrinter struct {
	out io.Writer
}

func NewTopCmdPrinter(out io.Writer) *TopCmdPrinter {
	return &TopCmdPrinter{out: out}
}

func (printer *TopCmdPrinter) PrintPodMetrics(metrics []metricsapi.PodMetrics, printContainers, withNamespace, noHeaders bool, sortBy string, sum bool) error {
	if len(metrics) == 0 {
		return nil
	}
	w := printers.GetNewTabWriter(printer.out)
	defer w.Flush()

	columnWidth := len(PodColumns)
	if !noHeaders {
		if withNamespace {
			printValue(w, NamespaceColumn)
			columnWidth++
		}
		if printContainers {
			printValue(w, PodColumn)
			columnWidth++
		}
		printColumnNames(w, PodColumns)
	}

	sort.Sort(NewPodMetricsSorter(metrics, withNamespace, sortBy))

	for i := range metrics {
		if printContainers {
			sort.Sort(metricsutil.NewContainerMetricsSorter(metrics[i].Containers, sortBy))
			printSinglePodContainerMetrics(w, &metrics[i], withNamespace)
		} else {
			printSinglePodMetrics(w, &metrics[i], withNamespace)
		}
	}

	if sum {
		adder := NewResourceAdder(MeasuredResources)
		for i := range metrics {
			adder.AddPodMetrics(&metrics[i])
		}
		printPodResourcesSum(w, adder.total, columnWidth)
	}

	return nil
}

func printColumnNames(out io.Writer, names []string) {
	for _, name := range names {
		printValue(out, name)
	}
	fmt.Fprint(out, "\n")
}

func printSinglePodMetrics(out io.Writer, m *metricsapi.PodMetrics, withNamespace bool) {
	podMetrics := getPodMetrics(m)
	if withNamespace {
		printValue(out, m.Namespace)
	}
	printMetricsLine(out, &ResourceMetricsInfo{
		Name:      m.Name,
		Cluster:   m.Annotations[autoscalingv1alpha1.QuerySourceAnnotationKey],
		Metrics:   podMetrics,
		Available: corev1.ResourceList{},
	})
}

func printSinglePodContainerMetrics(out io.Writer, m *metricsapi.PodMetrics, withNamespace bool) {
	for _, c := range m.Containers {
		if withNamespace {
			printValue(out, m.Namespace)
		}
		printValue(out, m.Name)
		printMetricsLine(out, &ResourceMetricsInfo{
			Name:      c.Name,
			Cluster:   m.Annotations[autoscalingv1alpha1.QuerySourceAnnotationKey],
			Metrics:   c.Usage,
			Available: corev1.ResourceList{},
		})
	}
}

func getPodMetrics(m *metricsapi.PodMetrics) corev1.ResourceList {
	podMetrics := make(corev1.ResourceList)
	for _, res := range MeasuredResources {
		podMetrics[res], _ = resource.ParseQuantity("0")
	}

	for _, c := range m.Containers {
		for _, res := range MeasuredResources {
			quantity := podMetrics[res]
			quantity.Add(c.Usage[res])
			podMetrics[res] = quantity
		}
	}
	return podMetrics
}

func printMetricsLine(out io.Writer, metrics *ResourceMetricsInfo) {
	printValue(out, metrics.Name)
	printValue(out, metrics.Cluster)
	printAllResourceUsages(out, metrics)
	fmt.Fprint(out, "\n")
}

func printValue(out io.Writer, value interface{}) {
	fmt.Fprintf(out, "%v\t", value)
}

func printAllResourceUsages(out io.Writer, metrics *ResourceMetricsInfo) {
	for _, res := range MeasuredResources {
		quantity := metrics.Metrics[res]
		printSingleResourceUsage(out, res, quantity)
		fmt.Fprint(out, "\t")
		if available, found := metrics.Available[res]; found {
			fraction := float64(quantity.MilliValue()) / float64(available.MilliValue()) * 100
			fmt.Fprintf(out, "%d%%\t", int64(fraction))
		}
	}
}

func printSingleResourceUsage(out io.Writer, resourceType corev1.ResourceName, quantity resource.Quantity) {
	switch resourceType {
	case corev1.ResourceCPU:
		fmt.Fprintf(out, "%vm", quantity.MilliValue())
	case corev1.ResourceMemory:
		fmt.Fprintf(out, "%vMi", quantity.Value()/(1024*1024))
	default:
		fmt.Fprintf(out, "%v", quantity.Value())
	}
}

func printPodResourcesSum(out io.Writer, total corev1.ResourceList, columnWidth int) {
	for i := 0; i < columnWidth-2; i++ {
		printValue(out, "")
	}
	printValue(out, "________")
	printValue(out, "________")
	fmt.Fprintf(out, "\n")
	for i := 0; i < columnWidth-4; i++ {
		printValue(out, "")
	}
	printMetricsLine(out, &ResourceMetricsInfo{
		Name:      "",
		Metrics:   total,
		Available: corev1.ResourceList{},
	})
}

type ResourceAdder struct {
	resources []corev1.ResourceName
	total     corev1.ResourceList
}

func NewResourceAdder(resources []corev1.ResourceName) *ResourceAdder {
	return &ResourceAdder{
		resources: resources,
		total:     make(corev1.ResourceList),
	}
}

// AddPodMetrics adds each pod metric to the total
func (adder *ResourceAdder) AddPodMetrics(m *metricsapi.PodMetrics) {
	for _, c := range m.Containers {
		for _, res := range adder.resources {
			total := adder.total[res]
			total.Add(c.Usage[res])
			adder.total[res] = total
		}
	}
}
