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
