package metrics

import "time"

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
	// ClusterNotReady means binding needs to be rescheduled for cluster is not ready
	ClusterNotReady = "ClusterNotReady"
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
	schedulingAlgorithmLatency.WithLabelValues(action).Observe(SinceInSeconds(startTime))
}

// CountSchedulerBindings records the number of binding added to scheduling queues by event type.
func CountSchedulerBindings(event string) {
	schedulerQueueIncomingBindings.WithLabelValues(event).Inc()
}
