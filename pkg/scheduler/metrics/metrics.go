package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// SchedulerSubsystem - subsystem name used by scheduler
const SchedulerSubsystem = "karmada_scheduler"

var (
	scheduleAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "schedule_attempts_total",
			Help:      "Number of attempts to schedule resourceBinding",
		}, []string{"result", "scheduleType"})

	e2eSchedulingLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "e2e_scheduling_duration_seconds",
			Help:      "E2e scheduling latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 15),
		}, []string{"result", "scheduleType"})

	schedulingAlgorithmLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "scheduling_algorithm_duration_seconds",
			Help:      "Scheduling algorithm latency in seconds(exclude scale scheduler)",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 15),
		}, []string{"scheduleStep"})

	schedulerQueueIncomingBindings = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "queue_incoming_bindings_total",
			Help:      "Number of bindings added to scheduling queues by event type.",
		}, []string{"event"})

	metricsList = []prometheus.Collector{
		scheduleAttempts,
		e2eSchedulingLatency,
		schedulingAlgorithmLatency,
		schedulerQueueIncomingBindings,
	}
)

var registerMetrics sync.Once

// Register all metrics.
func Register() {
	// Register the metrics.
	registerMetrics.Do(func() {
		RegisterMetrics(metricsList...)
	})
}

// RegisterMetrics registers a list of metrics.
// This function is exported because it is intended to be used by out-of-tree plugins to register their custom metrics.
func RegisterMetrics(extraMetrics ...prometheus.Collector) {
	for _, metric := range extraMetrics {
		prometheus.MustRegister(metric)
	}
}

// SinceInSeconds gets the time since the specified start in seconds.
func SinceInSeconds(start time.Time) float64 {
	return time.Since(start).Seconds()
}
