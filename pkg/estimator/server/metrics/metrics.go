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

	metrics = []prometheus.Collector{
		requestCount,
		estimatingAlgorithmLatency,
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
