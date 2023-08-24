package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	utilmetrics "github.com/karmada-io/karmada/pkg/util/metrics"
)

// SchedulerSubsystem - subsystem name used by scheduler
const SchedulerSubsystem = "karmada_scheduler"

const (
	scheduledResult = "scheduled"
	errorResult     = "error"
)

const (
	// BindingAdd is the event when a new binding is added to API server.
	BindingAdd = "BindingAdd"
	// BindingUpdate is the event when a new binding is updated to API server.
	BindingUpdate = "BindingUpdate"
	// ScheduleAttemptFailure is the event when a schedule attempt fails.
	ScheduleAttemptFailure = "ScheduleAttemptFailure"
	// PolicyChanged means binding needs to be rescheduled for the policy changed
	PolicyChanged = "PolicyChanged"
	// ClusterChanged means binding needs to be rescheduled for the cluster changed
	ClusterChanged = "ClusterChanged"
)

const (
	// ScheduleStepFilter means the step in generic scheduler to filter clusters
	ScheduleStepFilter = "Filter"
	// ScheduleStepScore means the step in generic scheduler to score clusters
	ScheduleStepScore = "Score"
	// ScheduleStepSelect means the step in generic scheduler to select clusters
	ScheduleStepSelect = "Select"
	// ScheduleStepAssignReplicas means the step in generic scheduler to assign replicas
	ScheduleStepAssignReplicas = "AssignReplicas"
)

var (
	scheduleAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "schedule_attempts_total",
			Help:      "Number of attempts to schedule a ResourceBinding or ClusterResourceBinding",
		}, []string{"result", "schedule_type"})

	e2eSchedulingLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "e2e_scheduling_duration_seconds",
			Help:      "E2e scheduling latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 15),
		}, []string{"result", "schedule_type"})

	schedulingAlgorithmLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "scheduling_algorithm_duration_seconds",
			Help:      "Scheduling algorithm latency in seconds(exclude scale scheduler)",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 15),
		}, []string{"schedule_step"})

	// SchedulerQueueIncomingBindings is the Number of ResourceBinding and ClusterResourceBinding objects added to scheduling queues by event type.
	SchedulerQueueIncomingBindings = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "queue_incoming_bindings_total",
			Help:      "Number of ResourceBinding and ClusterResourceBinding objects added to scheduling queues by event type.",
		}, []string{"event"})

	// FrameworkExtensionPointDuration is the metrics which indicates the latency for running all plugins of a specific extension point.
	FrameworkExtensionPointDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "framework_extension_point_duration_seconds",
			Help:      "Latency for running all plugins of a specific extension point.",
			// Start with 0.1ms with the last bucket being [~200ms, Inf)
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 12),
		},
		[]string{"extension_point", "result"})

	// PluginExecutionDuration is the metrics which indicates the duration for running a plugin at a specific extension point.
	PluginExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "plugin_execution_duration_seconds",
			Help:      "Duration for running a plugin at a specific extension point.",
			// Start with 0.01ms with the last bucket being [~22ms, Inf). We use a small factor (1.5)
			// so that we have better granularity since plugin latency is very sensitive.
			Buckets: prometheus.ExponentialBuckets(0.00001, 1.5, 20),
		},
		[]string{"plugin", "extension_point", "result"})

	metrics = []prometheus.Collector{
		scheduleAttempts,
		e2eSchedulingLatency,
		schedulingAlgorithmLatency,
		SchedulerQueueIncomingBindings,
		FrameworkExtensionPointDuration,
		PluginExecutionDuration,
	}
)

func init() {
	for _, m := range metrics {
		ctrlmetrics.Registry.MustRegister(m)
	}
}

// BindingSchedule can record a scheduling attempt and the duration
// since `start`.
func BindingSchedule(scheduleType string, duration float64, err error) {
	if err != nil {
		observeScheduleAttemptAndLatency(errorResult, scheduleType, duration)
	} else {
		observeScheduleAttemptAndLatency(scheduledResult, scheduleType, duration)
	}
}

func observeScheduleAttemptAndLatency(result, scheduleType string, duration float64) {
	e2eSchedulingLatency.WithLabelValues(result, scheduleType).Observe(duration)
	scheduleAttempts.WithLabelValues(result, scheduleType).Inc()
}

// ScheduleStep can record each scheduling step duration.
func ScheduleStep(action string, startTime time.Time) {
	schedulingAlgorithmLatency.WithLabelValues(action).Observe(utilmetrics.DurationInSeconds(startTime))
}

// CountSchedulerBindings records the number of binding added to scheduling queues by event type.
func CountSchedulerBindings(event string) {
	SchedulerQueueIncomingBindings.WithLabelValues(event).Inc()
}
