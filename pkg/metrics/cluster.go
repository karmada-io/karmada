package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	utilmetrics "github.com/karmada-io/karmada/pkg/util/metrics"
)

const (
	clusterReadyMetricsName              = "cluster_ready_state"
	clusterTotalNodeNumberMetricsName    = "cluster_node_number"
	clusterReadyNodeNumberMetricsName    = "cluster_ready_node_number"
	clusterMemoryAllocatableMetricsName  = "cluster_memory_allocatable_bytes"
	clusterCPUAllocatableMetricsName     = "cluster_cpu_allocatable_number"
	clusterPodAllocatableMetricsName     = "cluster_pod_allocatable_number"
	clusterMemoryAllocatedMetricsName    = "cluster_memory_allocated_bytes"
	clusterCPUAllocatedMetricsName       = "cluster_cpu_allocated_number"
	clusterPodAllocatedMetricsName       = "cluster_pod_allocated_number"
	clusterSyncStatusDurationMetricsName = "cluster_sync_status_duration_seconds"
)

var (
	// clusterReadyGauge reports if the cluster is ready.
	clusterReadyGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterReadyMetricsName,
		Help: "State of the cluster(1 if ready, 0 otherwise).",
	}, []string{"cluster_name"})

	// clusterTotalNodeNumberGauge reports the number of nodes in the given cluster.
	clusterTotalNodeNumberGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterTotalNodeNumberMetricsName,
		Help: "Number of nodes in the cluster.",
	}, []string{"cluster_name"})

	// clusterReadyNodeNumberGauge reports the number of ready nodes in the given cluster.
	clusterReadyNodeNumberGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterReadyNodeNumberMetricsName,
		Help: "Number of ready nodes in the cluster.",
	}, []string{"cluster_name"})

	// clusterMemoryAllocatableGauge reports the allocatable memory in the given cluster.
	clusterMemoryAllocatableGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterMemoryAllocatableMetricsName,
		Help: "Allocatable cluster memory resource in bytes.",
	}, []string{"cluster_name"})

	// clusterCPUAllocatableGauge reports the allocatable CPU in the given cluster.
	clusterCPUAllocatableGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterCPUAllocatableMetricsName,
		Help: "Number of allocatable CPU in the cluster.",
	}, []string{"cluster_name"})

	// clusterPodAllocatableGauge reports the allocatable Pod number in the given cluster.
	clusterPodAllocatableGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterPodAllocatableMetricsName,
		Help: "Number of allocatable pods in the cluster.",
	}, []string{"cluster_name"})

	// clusterMemoryAllocatedGauge reports the allocated memory in the given cluster.
	clusterMemoryAllocatedGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterMemoryAllocatedMetricsName,
		Help: "Allocated cluster memory resource in bytes.",
	}, []string{"cluster_name"})

	// clusterCPUAllocatedGauge reports the allocated CPU in the given cluster.
	clusterCPUAllocatedGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterCPUAllocatedMetricsName,
		Help: "Number of allocated CPU in the cluster.",
	}, []string{"cluster_name"})

	// clusterPodAllocatedGauge reports the allocated Pod number in the given cluster.
	clusterPodAllocatedGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterPodAllocatedMetricsName,
		Help: "Number of allocated pods in the cluster.",
	}, []string{"cluster_name"})

	// clusterSyncStatusDuration reports the duration of the given cluster syncing status.
	clusterSyncStatusDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: clusterSyncStatusDurationMetricsName,
		Help: "Duration in seconds for syncing the status of the cluster once.",
	}, []string{"cluster_name"})
)

// RecordClusterStatus records the status of the given cluster.
func RecordClusterStatus(cluster *v1alpha1.Cluster) {
	clusterReadyGauge.WithLabelValues(cluster.Name).Set(func() float64 {
		if util.IsClusterReady(&cluster.Status) {
			return 1
		}
		return 0
	}())

	if cluster.Status.NodeSummary != nil {
		clusterTotalNodeNumberGauge.WithLabelValues(cluster.Name).Set(float64(cluster.Status.NodeSummary.TotalNum))
		clusterReadyNodeNumberGauge.WithLabelValues(cluster.Name).Set(float64(cluster.Status.NodeSummary.ReadyNum))
	}

	if cluster.Status.ResourceSummary != nil {
		if cluster.Status.ResourceSummary.Allocatable != nil {
			clusterMemoryAllocatableGauge.WithLabelValues(cluster.Name).Set(cluster.Status.ResourceSummary.Allocatable.Memory().AsApproximateFloat64())
			clusterCPUAllocatableGauge.WithLabelValues(cluster.Name).Set(cluster.Status.ResourceSummary.Allocatable.Cpu().AsApproximateFloat64())
			clusterPodAllocatableGauge.WithLabelValues(cluster.Name).Set(cluster.Status.ResourceSummary.Allocatable.Pods().AsApproximateFloat64())
		}

		if cluster.Status.ResourceSummary.Allocated != nil {
			clusterMemoryAllocatedGauge.WithLabelValues(cluster.Name).Set(cluster.Status.ResourceSummary.Allocated.Memory().AsApproximateFloat64())
			clusterCPUAllocatedGauge.WithLabelValues(cluster.Name).Set(cluster.Status.ResourceSummary.Allocated.Cpu().AsApproximateFloat64())
			clusterPodAllocatedGauge.WithLabelValues(cluster.Name).Set(cluster.Status.ResourceSummary.Allocated.Pods().AsApproximateFloat64())
		}
	}
}

// RecordClusterSyncStatusDuration records the duration of the given cluster syncing status
func RecordClusterSyncStatusDuration(cluster *v1alpha1.Cluster, startTime time.Time) {
	clusterSyncStatusDuration.WithLabelValues(cluster.Name).Observe(utilmetrics.DurationInSeconds(startTime))
}

// ClusterCollectors returns the collectors about clusters.
func ClusterCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		clusterReadyGauge,
		clusterTotalNodeNumberGauge,
		clusterReadyNodeNumberGauge,
		clusterMemoryAllocatableGauge,
		clusterCPUAllocatableGauge,
		clusterPodAllocatableGauge,
		clusterMemoryAllocatedGauge,
		clusterCPUAllocatedGauge,
		clusterPodAllocatedGauge,
		clusterSyncStatusDuration,
	}
}
