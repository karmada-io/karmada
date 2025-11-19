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
	evictionQueueDepthMetricsName        = "eviction_queue_depth"
	evictionKindTotalMetricsName         = "eviction_kind_total"
	evictionProcessingLatencyMetricsName = "eviction_processing_latency_seconds"
	evictionProcessingTotalMetricsName   = "eviction_processing_total"

	// Canonical label for Karmada member clusters.
	memberClusterLabel = "member_cluster"

	// DEPRECATED: cluster_name (target removal: 1.18)
	// Rationale: avoid collision with Prometheus external_labels like cluster and standardize on the metric label name used to denote a Karmada member cluster across all metrics.
	// Migration: use member_cluster instead across all queries and dashboards.
	clusterNameLabel = "cluster_name"
)

var (
	// clusterReadyGauge reports if the cluster is ready.
	clusterReadyGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterReadyMetricsName,
		Help: "State of the cluster (1 if ready, 0 otherwise). [Label deprecation: cluster_name deprecated in 1.16; use member_cluster. Removal planned 1.18.]",
	}, []string{memberClusterLabel, clusterNameLabel})

	// clusterTotalNodeNumberGauge reports the number of nodes in the given cluster.
	clusterTotalNodeNumberGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterTotalNodeNumberMetricsName,
		Help: "Number of nodes in the cluster. [Label deprecation: cluster_name deprecated in 1.16; use member_cluster. Removal planned 1.18.]",
	}, []string{memberClusterLabel, clusterNameLabel})

	// clusterReadyNodeNumberGauge reports the number of ready nodes in the given cluster.
	clusterReadyNodeNumberGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterReadyNodeNumberMetricsName,
		Help: "Number of ready nodes in the cluster. [Label deprecation: cluster_name deprecated in 1.16; use member_cluster. Removal planned 1.18.]",
	}, []string{memberClusterLabel, clusterNameLabel})

	// clusterMemoryAllocatableGauge reports the allocatable memory in the given cluster.
	clusterMemoryAllocatableGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterMemoryAllocatableMetricsName,
		Help: "Allocatable cluster memory resource in bytes. [Label deprecation: cluster_name deprecated in 1.16; use member_cluster. Removal planned 1.18.]",
	}, []string{memberClusterLabel, clusterNameLabel})

	// clusterCPUAllocatableGauge reports the allocatable CPU in the given cluster.
	clusterCPUAllocatableGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterCPUAllocatableMetricsName,
		Help: "Number of allocatable CPU in the cluster. [Label deprecation: cluster_name deprecated in 1.16; use member_cluster. Removal planned 1.18.]",
	}, []string{memberClusterLabel, clusterNameLabel})

	// clusterPodAllocatableGauge reports the allocatable Pod number in the given cluster.
	clusterPodAllocatableGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterPodAllocatableMetricsName,
		Help: "Number of allocatable pods in the cluster. [Label deprecation: cluster_name deprecated in 1.16; use member_cluster. Removal planned 1.18.]",
	}, []string{memberClusterLabel, clusterNameLabel})

	// clusterMemoryAllocatedGauge reports the allocated memory in the given cluster.
	clusterMemoryAllocatedGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterMemoryAllocatedMetricsName,
		Help: "Allocated cluster memory resource in bytes. [Label deprecation: cluster_name deprecated in 1.16; use member_cluster. Removal planned 1.18.]",
	}, []string{memberClusterLabel, clusterNameLabel})

	// clusterCPUAllocatedGauge reports the allocated CPU in the given cluster.
	clusterCPUAllocatedGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterCPUAllocatedMetricsName,
		Help: "Number of allocated CPU in the cluster. [Label deprecation: cluster_name deprecated in 1.16; use member_cluster. Removal planned 1.18.]",
	}, []string{memberClusterLabel, clusterNameLabel})

	// clusterPodAllocatedGauge reports the allocated Pod number in the given cluster.
	clusterPodAllocatedGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: clusterPodAllocatedMetricsName,
		Help: "Number of allocated pods in the cluster. [Label deprecation: cluster_name deprecated in 1.16; use member_cluster. Removal planned 1.18.]",
	}, []string{memberClusterLabel, clusterNameLabel})

	// clusterSyncStatusDuration reports the duration of the given cluster syncing status.
	clusterSyncStatusDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: clusterSyncStatusDurationMetricsName,
		Help: "Duration in seconds for syncing the status of the cluster once. [Label deprecation: cluster_name deprecated in 1.16; use member_cluster. Removal planned 1.18.]",
	}, []string{memberClusterLabel, clusterNameLabel})

	evictionQueueMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: evictionQueueDepthMetricsName,
		Help: "Current depth of the eviction queue",
	}, []string{"name"})

	evictionKindTotalMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: evictionKindTotalMetricsName,
		Help: "Number of resources in the eviction queue by resource kind [Label deprecation: cluster_name deprecated in 1.16; use member_cluster. Removal planned 1.18.]",
	}, []string{memberClusterLabel, clusterNameLabel, "resource_kind"})

	evictionProcessingLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    evictionProcessingLatencyMetricsName,
		Help:    "Latency of processing an eviction task in seconds",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
	}, []string{"name"})

	evictionProcessingTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: evictionProcessingTotalMetricsName,
		Help: "Total number of evictions processed",
	}, []string{"name", "result"})
)

// RecordClusterStatus records the status of the given cluster.
func RecordClusterStatus(cluster *v1alpha1.Cluster) {
	labels := []string{cluster.Name, cluster.Name} // member_cluster, cluster_name

	clusterReadyGauge.WithLabelValues(labels...).Set(func() float64 {
		if util.IsClusterReady(&cluster.Status) {
			return 1
		}
		return 0
	}())

	if cluster.Status.NodeSummary != nil {
		clusterTotalNodeNumberGauge.WithLabelValues(labels...).Set(float64(cluster.Status.NodeSummary.TotalNum))
		clusterReadyNodeNumberGauge.WithLabelValues(labels...).Set(float64(cluster.Status.NodeSummary.ReadyNum))
	}

	if cluster.Status.ResourceSummary != nil {
		if cluster.Status.ResourceSummary.Allocatable != nil {
			clusterMemoryAllocatableGauge.WithLabelValues(labels...).Set(cluster.Status.ResourceSummary.Allocatable.Memory().AsApproximateFloat64())
			clusterCPUAllocatableGauge.WithLabelValues(labels...).Set(cluster.Status.ResourceSummary.Allocatable.Cpu().AsApproximateFloat64())
			clusterPodAllocatableGauge.WithLabelValues(labels...).Set(cluster.Status.ResourceSummary.Allocatable.Pods().AsApproximateFloat64())
		}

		if cluster.Status.ResourceSummary.Allocated != nil {
			clusterMemoryAllocatedGauge.WithLabelValues(labels...).Set(cluster.Status.ResourceSummary.Allocated.Memory().AsApproximateFloat64())
			clusterCPUAllocatedGauge.WithLabelValues(labels...).Set(cluster.Status.ResourceSummary.Allocated.Cpu().AsApproximateFloat64())
			clusterPodAllocatedGauge.WithLabelValues(labels...).Set(cluster.Status.ResourceSummary.Allocated.Pods().AsApproximateFloat64())
		}
	}
}

// RecordClusterSyncStatusDuration records the duration of the given cluster syncing status
func RecordClusterSyncStatusDuration(cluster *v1alpha1.Cluster, startTime time.Time) {
	labels := []string{cluster.Name, cluster.Name}
	clusterSyncStatusDuration.WithLabelValues(labels...).Observe(utilmetrics.DurationInSeconds(startTime))
}

// CleanupMetricsForCluster removes the cluster status metrics after the cluster is deleted.
func CleanupMetricsForCluster(clusterName string) {
	labels := []string{clusterName, clusterName}

	clusterReadyGauge.DeleteLabelValues(labels...)
	clusterTotalNodeNumberGauge.DeleteLabelValues(labels...)
	clusterReadyNodeNumberGauge.DeleteLabelValues(labels...)
	clusterMemoryAllocatableGauge.DeleteLabelValues(labels...)
	clusterCPUAllocatableGauge.DeleteLabelValues(labels...)
	clusterPodAllocatableGauge.DeleteLabelValues(labels...)
	clusterMemoryAllocatedGauge.DeleteLabelValues(labels...)
	clusterCPUAllocatedGauge.DeleteLabelValues(labels...)
	clusterPodAllocatedGauge.DeleteLabelValues(labels...)
	clusterSyncStatusDuration.DeleteLabelValues(labels...)
}

// RecordEvictionQueueMetrics record the depth Of the EvictionQueue
func RecordEvictionQueueMetrics(name string, depth float64) {
	evictionQueueMetrics.WithLabelValues(name).Set(depth)
}

// RecordEvictionKindMetrics records eviction queue items by resource type
// Increase count when true and decrease count when false
func RecordEvictionKindMetrics(clusterName, resourceKind string, increase bool) {
	if clusterName == "" || resourceKind == "" {
		return
	}

	labels := []string{clusterName, clusterName, resourceKind}
	if increase {
		evictionKindTotalMetrics.WithLabelValues(labels...).Inc()
	} else {
		evictionKindTotalMetrics.WithLabelValues(labels...).Dec()
	}
}

// RecordEvictionProcessingMetrics records the processing delay and results of the eviction task
func RecordEvictionProcessingMetrics(name string, err error, startTime time.Time) {
	latency := utilmetrics.DurationInSeconds(startTime)
	evictionProcessingLatency.WithLabelValues(name).Observe(latency)

	result := utilmetrics.GetResultByError(err)
	evictionProcessingTotal.WithLabelValues(name, result).Inc()
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
		evictionQueueMetrics,
		evictionKindTotalMetrics,
		evictionProcessingLatency,
		evictionProcessingTotal,
	}
}
