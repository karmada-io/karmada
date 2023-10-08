package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	utilmetrics "github.com/karmada-io/karmada/pkg/util/metrics"
)

const (
	resourceMatchPolicyDurationMetricsName     = "resource_match_policy_duration_seconds"
	resourceApplyPolicyDurationMetricsName     = "resource_apply_policy_duration_seconds"
	policyApplyAttemptsMetricsName             = "policy_apply_attempts_total"
	syncWorkDurationMetricsName                = "binding_sync_work_duration_seconds"
	syncWorkloadDurationMetricsName            = "work_sync_workload_duration_seconds"
	policyPreemptionMetricsName                = "policy_preemption_total"
	cronFederatedHPADurationMetricsName        = "cronfederatedhpa_process_duration_seconds"
	cronFederatedHPARuleDurationMetricsName    = "cronfederatedhpa_rule_process_duration_seconds"
	federatedHPADurationMetricsName            = "federatedhpa_process_duration_seconds"
	federatedHPAPullMetricsDurationMetricsName = "federatedhpa_pull_metrics_duration_seconds"
)

var (
	findMatchedPolicyDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    resourceMatchPolicyDurationMetricsName,
		Help:    "Duration in seconds to find a matched PropagationPolicy or ClusterPropagationPolicy for the resource templates.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
	}, []string{})

	applyPolicyDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    resourceApplyPolicyDurationMetricsName,
		Help:    "Duration in seconds to apply a PropagationPolicy or ClusterPropagationPolicy for the resource templates. By the result, 'error' means a resource template failed to apply the policy. Otherwise 'success'.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
	}, []string{"result"})

	policyApplyAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: policyApplyAttemptsMetricsName,
		Help: "Number of attempts to be applied for a PropagationPolicy or ClusterPropagationPolicy. By the result, 'error' means a resource template failed to apply the policy. Otherwise 'success'.",
	}, []string{"result"})

	syncWorkDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    syncWorkDurationMetricsName,
		Help:    "Duration in seconds to sync works for ResourceBinding and ClusterResourceBinding objects. By the result, 'error' means a binding failed to sync works. Otherwise 'success'.",
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

	cronFederatedHPADurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    cronFederatedHPADurationMetricsName,
		Help:    "Duration in seconds to process a CronFederatedHPA. By the result, 'error' means a CronFederatedHPA failed to be processed. Otherwise 'success'.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
	}, []string{"result"})

	cronFederatedHPARuleDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    cronFederatedHPARuleDurationMetricsName,
		Help:    "Duration in seconds to process a CronFederatedHPA rule. By the result, 'error' means a CronFederatedHPA rule failed to be processed. Otherwise 'success'.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
	}, []string{"result"})

	federatedHPADurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    federatedHPADurationMetricsName,
		Help:    "Duration in seconds to process a FederatedHPA. By the result, 'error' means a FederatedHPA failed to be processed. Otherwise 'success'.",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 12),
	}, []string{"result"})

	federatedHPAPullMetricsDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    federatedHPAPullMetricsDurationMetricsName,
		Help:    "Duration in seconds taken by the FederatedHPA to pull metrics. By the result, 'error' means the FederatedHPA failed to pull the metrics. Otherwise 'success'.",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 12),
	}, []string{"result", "metricType"})
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

// ObserveProcessCronFederatedHPALatency records the duration to process a cron federated HPA.
func ObserveProcessCronFederatedHPALatency(err error, start time.Time) {
	cronFederatedHPADurationHistogram.WithLabelValues(utilmetrics.GetResultByError(err)).Observe(utilmetrics.DurationInSeconds(start))
}

// ObserveProcessCronFederatedHPARuleLatency records the duration to process a cron federated HPA rule.
func ObserveProcessCronFederatedHPARuleLatency(err error, start time.Time) {
	cronFederatedHPARuleDurationHistogram.WithLabelValues(utilmetrics.GetResultByError(err)).Observe(utilmetrics.DurationInSeconds(start))
}

// ObserveProcessFederatedHPALatency records the duration to process a FederatedHPA.
func ObserveProcessFederatedHPALatency(err error, start time.Time) {
	federatedHPADurationHistogram.WithLabelValues(utilmetrics.GetResultByError(err)).Observe(utilmetrics.DurationInSeconds(start))
}

// ObserveFederatedHPAPullMetricsLatency records the duration it takes for the FederatedHPA to pull metrics.
func ObserveFederatedHPAPullMetricsLatency(err error, metricType string, start time.Time) {
	federatedHPAPullMetricsDurationHistogram.WithLabelValues(utilmetrics.GetResultByError(err), metricType).Observe(utilmetrics.DurationInSeconds(start))
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
		cronFederatedHPADurationHistogram,
		cronFederatedHPARuleDurationHistogram,
		federatedHPADurationHistogram,
		federatedHPAPullMetricsDurationHistogram,
	}
}

// ResourceCollectorsForAgent returns the collectors about resources for karmada-agent.
func ResourceCollectorsForAgent() []prometheus.Collector {
	return []prometheus.Collector{
		syncWorkloadDurationHistogram,
	}
}
