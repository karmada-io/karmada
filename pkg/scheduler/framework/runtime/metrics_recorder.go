package runtime

import (
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

	// stopCh is used to stop the goroutine which periodically flushes metrics. It's currently only
	// used in tests.
	stopCh chan struct{}
	// isStoppedCh indicates whether the goroutine is stopped. It's used in tests only to make sure
	// the metric flushing goroutine is stopped so that tests can collect metrics for verification.
	isStoppedCh chan struct{}
}

func newMetricsRecorder(bufferSize int, interval time.Duration) *metricsRecorder {
	recorder := &metricsRecorder{
		bufferCh:    make(chan *frameworkMetric, bufferSize),
		bufferSize:  bufferSize,
		interval:    interval,
		stopCh:      make(chan struct{}),
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
		case <-r.stopCh:
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
