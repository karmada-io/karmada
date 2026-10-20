/*
Copyright 2025 The Karmada Authors.

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
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	// SchedulerSubsystem - subsystem name used by scheduler.
	SchedulerSubsystem = "karmada_scheduler"
)

var (
	pendingBindings = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: SchedulerSubsystem,
			Name:      "pending_bindings",
			Help:      "Number of pending bindings, by the queue type. 'active' means number of bindings in activeQ; 'backoff' means number of bindings in backoffQ; 'unschedulable' means number of bindings in unschedulableBindings that the scheduler attempted to schedule and failed.",
		}, []string{"queue"})

	metricsList = []prometheus.Collector{
		pendingBindings,
	}
)

func init() {
	// Register the metrics.
	RegisterMetrics(metricsList...)
}

// RegisterMetrics registers a list of metrics with the registry served on the scheduler's /metrics endpoint.
// This function is exported because it is intended to be used by out-of-tree plugins to register their custom metrics.
func RegisterMetrics(extraMetrics ...prometheus.Collector) {
	for _, metric := range extraMetrics {
		ctrlmetrics.Registry.MustRegister(metric)
	}
}

// ActiveBindings returns the pending bindings metrics with the label active
func ActiveBindings() prometheus.Gauge {
	return pendingBindings.With(prometheus.Labels{"queue": "active"})
}

// BackoffBindings returns the pending bindings metrics with the label backoff
func BackoffBindings() prometheus.Gauge {
	return pendingBindings.With(prometheus.Labels{"queue": "backoff"})
}

// UnschedulableBindings returns the pending bindings metrics with the label unschedulable
func UnschedulableBindings() prometheus.Gauge {
	return pendingBindings.With(prometheus.Labels{"queue": "unschedulable"})
}

// MetricRecorder represents a metric recorder which takes action when the
// metric Inc(), Dec() and Clear()
type MetricRecorder interface {
	Inc()
	Dec()
	Clear()
}

var _ MetricRecorder = &PendingBindingsRecorder{}

// PendingBindingsRecorder is an implementation of MetricRecorder
type PendingBindingsRecorder struct {
	recorder prometheus.Gauge
}

// NewActiveBindingsRecorder returns ActiveBindings in a Prometheus metric fashion
func NewActiveBindingsRecorder() *PendingBindingsRecorder {
	return &PendingBindingsRecorder{
		recorder: ActiveBindings(),
	}
}

// NewUnschedulableBindingsRecorder returns UnschedulableBindings in a Prometheus metric fashion
func NewUnschedulableBindingsRecorder() *PendingBindingsRecorder {
	return &PendingBindingsRecorder{
		recorder: UnschedulableBindings(),
	}
}

// NewBackoffBindingsRecorder returns BackoffBindings in a Prometheus metric fashion
func NewBackoffBindingsRecorder() *PendingBindingsRecorder {
	return &PendingBindingsRecorder{
		recorder: BackoffBindings(),
	}
}

// Inc increases a metric counter by 1, in an atomic way
func (r *PendingBindingsRecorder) Inc() {
	r.recorder.Inc()
}

// Dec decreases a metric counter by 1, in an atomic way
func (r *PendingBindingsRecorder) Dec() {
	r.recorder.Dec()
}

// Clear set a metric counter to 0, in an atomic way
func (r *PendingBindingsRecorder) Clear() {
	r.recorder.Set(float64(0))
}
