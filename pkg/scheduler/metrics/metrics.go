package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

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
	scheduleAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "schedule_attempts_total",
			Help:      "Number of attempts to schedule resourceBinding",
		}, []string{"result", "schedule_type"})

	e2eSchedulingLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "e2e_scheduling_duration_seconds",
			Help:      "E2e scheduling latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 15),
		}, []string{"result", "schedule_type"})

	schedulingAlgorithmLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "scheduling_algorithm_duration_seconds",
			Help:      "Scheduling algorithm latency in seconds(exclude scale scheduler)",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 15),
		}, []string{"schedule_step"})

	schedulerQueueIncomingBindings = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "queue_incoming_bindings_total",
			Help:      "Number of bindings added to scheduling queues by event type.",
		}, []string{"event"})
)

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
	schedulerQueueIncomingBindings.WithLabelValues(event).Inc()
}
