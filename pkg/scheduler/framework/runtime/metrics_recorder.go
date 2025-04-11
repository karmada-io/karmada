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

package runtime

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/metrics"
)

// frameworkMetric is the data structure passed in the buffer channel between the main framework thread
// and the metricsRecorder goroutine.
type frameworkMetric struct {
	metric      *prometheus.HistogramVec
	labelValues []string
	value       float64
}

// metricRecorder records framework metrics in a separate goroutine to avoid overhead in the critical path.
type metricsRecorder struct {
	// bufferCh is a channel that serves as a metrics buffer before the metricsRecorder goroutine reports it.
	bufferCh chan *frameworkMetric
	// if bufferSize is reached, incoming metrics will be discarded.
	bufferSize int
	// how often the recorder runs to flush the metrics.
	interval time.Duration

	// ctx and cancel are used to stop the goroutine which periodically flushes metrics. They're currently only
	// used in tests.
	ctx    context.Context
	cancel context.CancelFunc
	// isStoppedCh indicates whether the goroutine is stopped. It's used in tests only to make sure
	// the metric flushing goroutine is stopped so that tests can collect metrics for verification.
	isStoppedCh chan struct{}
}

func newMetricsRecorder(bufferSize int, interval time.Duration) *metricsRecorder {
	ctx, cancel := context.WithCancel(context.Background())
	recorder := &metricsRecorder{
		bufferCh:    make(chan *frameworkMetric, bufferSize),
		bufferSize:  bufferSize,
		interval:    interval,
		ctx:         ctx,
		cancel:      cancel,
		isStoppedCh: make(chan struct{}),
	}
	go recorder.run()
	return recorder
}

// observePluginDurationAsync observes the plugin_execution_duration_seconds metric.
// The metric will be flushed to Prometheus asynchronously.
func (r *metricsRecorder) observePluginDurationAsync(extensionPoint, pluginName string, result *framework.Result, value float64) {
	newMetric := &frameworkMetric{
		metric:      metrics.PluginExecutionDuration,
		labelValues: []string{pluginName, extensionPoint, result.Code().String()},
		value:       value,
	}
	select {
	case r.bufferCh <- newMetric:
	default:
	}
}

// run flushes buffered metrics into Prometheus every second.
func (r *metricsRecorder) run() {
	for {
		select {
		case <-r.ctx.Done():
			close(r.isStoppedCh)
			return
		default:
		}
		r.flushMetrics()
		time.Sleep(r.interval)
	}
}

// flushMetrics tries to clean up the bufferCh by reading at most bufferSize metrics.
func (r *metricsRecorder) flushMetrics() {
	for i := 0; i < r.bufferSize; i++ {
		select {
		case m := <-r.bufferCh:
			m.metric.WithLabelValues(m.labelValues...).Observe(m.value)
		default:
			return
		}
	}
}
