package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	utilmetrics "github.com/karmada-io/karmada/pkg/util/metrics"
)

const (
	resourceMatchPolicyDurationMetricsName = "resource_match_policy_duration_seconds"
	resourceApplyPolicyDurationMetricsName = "resource_apply_policy_duration_seconds"
	policyApplyAttemptsMetricsName         = "policy_apply_attempts_total"
	syncWorkDurationMetricsName            = "binding_sync_work_duration_seconds"
	syncWorkloadDurationMetricsName        = "work_sync_workload_duration_seconds"
	policyPreemptionMetricsName            = "policy_preemption_total"
)

var (
	findMatchedPolicyDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    resourceMatchPolicyDurationMetricsName,
		Help:    "Duration in seconds to find a matched propagation policy for the resource template.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
	}, []string{})

	applyPolicyDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    resourceApplyPolicyDurationMetricsName,
		Help:    "Duration in seconds to apply a propagation policy for the resource template. By the result, 'error' means a resource template failed to apply the policy. Otherwise 'success'.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
	}, []string{"result"})

	policyApplyAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: policyApplyAttemptsMetricsName,
		Help: "Number of attempts to be applied for a propagation policy. By the result, 'error' means a resource template failed to apply the policy. Otherwise 'success'.",
	}, []string{"result"})

	syncWorkDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    syncWorkDurationMetricsName,
		Help:    "Duration in seconds to sync works for a binding object. By the result, 'error' means a binding failed to sync works. Otherwise 'success'.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
	}, []string{"result"})

	syncWorkloadDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    syncWorkloadDurationMetricsName,
		Help:    "Duration in seconds to sync the workload to a target cluster. By the result, 'error' means a work failed to sync workloads. Otherwise 'success'.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
	}, []string{"result"})

	policyPreemptionCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: policyPreemptionMetricsName,
		Help: "Number of preemption for the resource template. By the result, 'error' means a resource template failed to be preempted by other propagation policies. Otherwise 'success'.",
	}, []string{"result"})
)

// ObserveFindMatchedPolicyLatency records the duration for the resource finding a matched policy.
func ObserveFindMatchedPolicyLatency(start time.Time) {
	findMatchedPolicyDurationHistogram.WithLabelValues().Observe(utilmetrics.DurationInSeconds(start))
}

// ObserveApplyPolicyAttemptAndLatency records the duration for the resource applying a policy and a applying attempt for the policy.
func ObserveApplyPolicyAttemptAndLatency(err error, start time.Time) {
	applyPolicyDurationHistogram.WithLabelValues(utilmetrics.GetResultByError(err)).Observe(utilmetrics.DurationInSeconds(start))
	policyApplyAttempts.WithLabelValues(utilmetrics.GetResultByError(err)).Inc()
}

// ObserveSyncWorkLatency records the duration to sync works for a binding object.
func ObserveSyncWorkLatency(err error, start time.Time) {
	syncWorkDurationHistogram.WithLabelValues(utilmetrics.GetResultByError(err)).Observe(utilmetrics.DurationInSeconds(start))
}

// ObserveSyncWorkloadLatency records the duration to sync the workload to a target cluster.
func ObserveSyncWorkloadLatency(err error, start time.Time) {
	syncWorkloadDurationHistogram.WithLabelValues(utilmetrics.GetResultByError(err)).Observe(utilmetrics.DurationInSeconds(start))
}

// CountPolicyPreemption records the numbers of policy preemption.
func CountPolicyPreemption(err error) {
	policyPreemptionCounter.WithLabelValues(utilmetrics.GetResultByError(err)).Inc()
}

// ResourceCollectors returns the collectors about resources.
func ResourceCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		applyPolicyDurationHistogram,
		findMatchedPolicyDurationHistogram,
		policyApplyAttempts,
		syncWorkDurationHistogram,
		syncWorkloadDurationHistogram,
		policyPreemptionCounter,
	}
}

// ResourceCollectorsForAgent returns the collectors about resources for karmada-agent.
func ResourceCollectorsForAgent() []prometheus.Collector {
	return []prometheus.Collector{
		syncWorkloadDurationHistogram,
	}
}
