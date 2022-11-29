package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	poolGetCounterMetricsName = "pool_get_operation_total"
	poolPutCounterMetricsName = "pool_put_operation_total"
)

var (
	poolGetCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: poolGetCounterMetricsName,
		Help: "Total times of getting from pool",
	}, []string{"name", "from"})

	poolPutCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: poolPutCounterMetricsName,
		Help: "Total times of putting from pool",
	}, []string{"name", "to"})
)

// RecordPoolGet records the times of getting from pool
func RecordPoolGet(name string, created bool) {
	from := "pool"
	if created {
		from = "new"
	}
	poolGetCounter.WithLabelValues(name, from).Inc()
}

// RecordPoolPut records the times of putting from pool
func RecordPoolPut(name string, destroyed bool) {
	to := "pool"
	if destroyed {
		to = "destroyed"
	}
	poolPutCounter.WithLabelValues(name, to).Inc()
}

// PoolCollectors returns the collectors about pool.
func PoolCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		poolGetCounter,
		poolPutCounter,
	}
}
