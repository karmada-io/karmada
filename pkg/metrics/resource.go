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
	"strconv"
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
	createResourceToCluster                    = "create_resource_to_cluster"
	updateResourceToCluster                    = "update_resource_to_cluster"
	deleteResourceFromCluster                  = "delete_resource_from_cluster"
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

	createResourceWhenSyncWork = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: createResourceToCluster,
		Help: "Number of creation operations against a target member cluster. The 'result' label indicates outcome ('success' or 'error'), 'recreate' indicates whether the operation is recreated (true/false). Labels 'apiversion', 'kind', and 'cluster' specify the resource type, API version, and target cluster respectively.",
	}, []string{"result", "apiversion", "kind", "cluster", "recreate"})

	updateResourceWhenSyncWork = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: updateResourceToCluster,
		Help: "Number of updating operation of the resource to a target member cluster. By the result, 'error' means a resource updated failed. Otherwise 'success'. Cluster means the target member cluster. operationResult means the result of the update operation.",
	}, []string{"result", "apiversion", "kind", "cluster", "operationResult"})

	deleteResourceWhenSyncWork = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: deleteResourceFromCluster,
		Help: "Number of deletion operations against a target member cluster. The 'result' label indicates outcome ('success' or 'error'). Labels 'apiversion', 'kind', and 'cluster' specify the resource's API version, type, and source cluster respectively.",
	}, []string{"result", "apiversion", "kind", "cluster"})

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

// CountCreateResourceToCluster records the number of creation operations of the resource for a target member cluster.
func CountCreateResourceToCluster(err error, apiVersion, kind, cluster string, recreate bool) {
	createResourceWhenSyncWork.WithLabelValues(utilmetrics.GetResultByError(err), apiVersion, kind, cluster, strconv.FormatBool(recreate)).Inc()
}

// CountUpdateResourceToCluster records the number of updating operation of the resource to a target member cluster.
func CountUpdateResourceToCluster(err error, apiVersion, kind, cluster string, operationResult string) {
	updateResourceWhenSyncWork.WithLabelValues(utilmetrics.GetResultByError(err), apiVersion, kind, cluster, operationResult).Inc()
}

// CountDeleteResourceFromCluster records the number of deletion operations of the resource from a target member cluster.
func CountDeleteResourceFromCluster(err error, apiVersion, kind, cluster string) {
	deleteResourceWhenSyncWork.WithLabelValues(utilmetrics.GetResultByError(err), apiVersion, kind, cluster).Inc()
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
		createResourceWhenSyncWork,
		updateResourceWhenSyncWork,
		deleteResourceWhenSyncWork,
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
