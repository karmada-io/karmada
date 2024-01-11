/*
Copyright 2021 The Karmada Authors.

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
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	utilmetrics "github.com/karmada-io/karmada/pkg/util/metrics"
)

// SchedulerEstimatorSubsystem - subsystem name used by scheduler estimator
const SchedulerEstimatorSubsystem = "karmada_scheduler_estimator"

const (
	// EstimatingTypeMaxAvailableReplicas - label of estimating type
	EstimatingTypeMaxAvailableReplicas = "MaxAvailableReplicas"
	// EstimatingTypeGetUnschedulableReplicas - label of estimating type
	EstimatingTypeGetUnschedulableReplicas = "GetUnschedulableReplicas"
)

const (
	// EstimatingStepListNodesByNodeClaim - label of estimating step
	EstimatingStepListNodesByNodeClaim = "ListNodesByNodeClaim"
	// EstimatingStepMaxAvailableReplicas - label of estimating step
	EstimatingStepMaxAvailableReplicas = "MaxAvailableReplicas"
	// EstimatingStepGetObjectFromCache - label of estimating step
	EstimatingStepGetObjectFromCache = "GetObjectFromCache"
	// EstimatingStepGetUnschedulablePodsOfWorkload - label of estimating step
	EstimatingStepGetUnschedulablePodsOfWorkload = "GetWorkloadUnschedulablePods"
	// EstimatingStepTotal - label of estimating step, total step
	EstimatingStepTotal = "Total"
)

var (
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SchedulerEstimatorSubsystem,
			Name:      "estimating_request_total",
			Help:      "Number of scheduler estimator requests",
		}, []string{"result", "type"})

	estimatingAlgorithmLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SchedulerEstimatorSubsystem,
			Name:      "estimating_algorithm_duration_seconds",
			Help:      "Estimating algorithm latency in seconds for each step",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		}, []string{"result", "type", "step"})

	// FrameworkExtensionPointDuration is the metrics which indicates the latency for running all plugins of a specific extension point.
	FrameworkExtensionPointDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SchedulerEstimatorSubsystem,
			Name:      "estimating_plugin_extension_point_duration_seconds",
			Help:      "Latency for running all plugins of a specific extension point.",
			// Start with 0.1ms with the last bucket being [~200ms, Inf)
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 12),
		},
		[]string{"estimating_plugin_extension_point"})

	// PluginExecutionDuration is the metrics which indicates the duration for running a plugin at a specific extension point.
	PluginExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SchedulerEstimatorSubsystem,
			Name:      "estimating_plugin_execution_duration_seconds",
			Help:      "Duration for running a plugin at a specific extension point.",
			// Start with 0.01ms with the last bucket being [~22ms, Inf). We use a small factor (1.5)
			// so that we have better granularity since plugin latency is very sensitive.
			Buckets: prometheus.ExponentialBuckets(0.00001, 1.5, 20),
		},
		[]string{"estimating_plugin", "estimating_plugin_extension_point"})

	metrics = []prometheus.Collector{
		requestCount,
		estimatingAlgorithmLatency,
		FrameworkExtensionPointDuration,
		PluginExecutionDuration,
	}
)

func init() {
	for _, m := range metrics {
		ctrlmetrics.Registry.MustRegister(m)
	}
}

// CountRequests total number of scheduler estimator requests
func CountRequests(err error, estimatingType string) {
	requestCount.WithLabelValues(utilmetrics.GetResultByError(err), estimatingType).Inc()
}

// UpdateEstimatingAlgorithmLatency updates latency for every step
func UpdateEstimatingAlgorithmLatency(err error, estimatingType, estimatingStep string, startTime time.Time) {
	estimatingAlgorithmLatency.WithLabelValues(utilmetrics.GetResultByError(err), estimatingType, estimatingStep).
		Observe(utilmetrics.DurationInSeconds(startTime))
}
