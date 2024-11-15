package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	utilmetrics "github.com/karmada-io/karmada/pkg/util/metrics"
)

var (
	// asyncWorkerReconcileTotal is a prometheus counter metrics which holds the total
	// number of reconciliations per controller. It has two labels. controller label refers
	// to the controller name and result label refers to the reconcile result i.e
	// success, error, requeue, requeue_after.
	asyncWorkerReconcileTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "async_worker_reconcile_total",
		Help: "Total number of reconciliations per controller",
	}, []string{"controller", "result"})

	// asyncWorkerReconcileErrors is a prometheus counter metrics which holds the total
	// number of errors from the Reconciler.
	asyncWorkerReconcileErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "async_worker_reconcile_errors_total",
		Help: "Total number of reconciliation errors per controller",
	}, []string{"controller"})

	// asyncWorkerTerminalReconcileErrors is a prometheus counter metrics which holds the total
	// number of terminal errors from the Reconciler.
	asyncWorkerTerminalReconcileErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "async_worker_terminal_reconcile_errors_total",
		Help: "Total number of terminal reconciliation errors per controller",
	}, []string{"controller"})

	// asyncWorkerReconcileTime is a prometheus metric which keeps track of the duration
	// of reconciliations.
	asyncWorkerReconcileTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "async_worker_reconcile_time_seconds",
		Help: "Length of time per reconciliation per controller",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.15, 0.2, 0.25, 0.3, 0.35, 0.4, 0.45, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0,
			1.25, 1.5, 1.75, 2.0, 2.5, 3.0, 3.5, 4.0, 4.5, 5, 6, 7, 8, 9, 10, 15, 20, 25, 30, 40, 50, 60},
	}, []string{"controller"})

	// asyncWorkerWorkerCount is a prometheus metric which holds the number of
	// concurrent reconciles per controller.
	asyncWorkerWorkerCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "async_worker_max_concurrent_reconciles",
		Help: "Maximum number of concurrent reconciles per controller",
	}, []string{"controller"})

	// asyncWorkerActiveWorkers is a prometheus metric which holds the number
	// of active workers per controller.
	asyncWorkerActiveWorkers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "async_worker_active_workers",
		Help: "Number of currently used workers per controller",
	}, []string{"controller"})
)

// InitAsyncWorkerMetrics init async worker metrics value.
func InitAsyncWorkerMetrics(name string, maxConcurrentReconciles int) {
	asyncWorkerReconcileTotal.WithLabelValues(name, utilmetrics.ResultError).Add(0)
	asyncWorkerReconcileTotal.WithLabelValues(name, utilmetrics.ResultSuccess).Add(0)
	asyncWorkerReconcileErrors.WithLabelValues(name).Add(0)
	asyncWorkerTerminalReconcileErrors.WithLabelValues(name).Add(0)
	asyncWorkerWorkerCount.WithLabelValues(name).Set(float64(maxConcurrentReconciles))
	asyncWorkerActiveWorkers.WithLabelValues(name).Set(0)
}

// IncReconcileTotal records reconcile total counter.
func IncReconcileTotal(name, label string) {
	asyncWorkerReconcileTotal.WithLabelValues(name, label).Inc()
}

// IncReconcileErrors records reconcile error counter.
func IncReconcileErrors(name string) {
	asyncWorkerReconcileErrors.WithLabelValues(name).Inc()
}

// IncTerminalReconcileErrors records reconcile terminal errors counter.
func IncTerminalReconcileErrors(name string) {
	asyncWorkerTerminalReconcileErrors.WithLabelValues(name).Inc()
}

// ObserveReconcileLatency records the duration for to reconcile.
func ObserveReconcileLatency(name string, start time.Time) {
	asyncWorkerReconcileTime.WithLabelValues(name).Observe(utilmetrics.DurationInSeconds(start))
}

// AddActiveWorkers records active workers per controller
func AddActiveWorkers(name string, count float64) {
	asyncWorkerActiveWorkers.WithLabelValues(name).Add(count)
}

// AsyncWorkerCollectors returns the collectors about asyncworker.
func AsyncWorkerCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		asyncWorkerReconcileTotal,
		asyncWorkerReconcileErrors,
		asyncWorkerTerminalReconcileErrors,
		asyncWorkerReconcileTime,
		asyncWorkerWorkerCount,
		asyncWorkerActiveWorkers,
	}
}
