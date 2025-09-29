package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	DependencyPolicyConflictsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "karmada",
			Subsystem: "dependencies",
			Name:      "dependency_policy_conflicts_total",
			Help:      "Total number of detected dependency policy conflicts across parents.",
		},
		[]string{"namespace"},
	)

	DependencyParentsCount = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "karmada",
			Subsystem: "dependencies",
			Name:      "dependency_parents_count",
			Help:      "Observed number of parents for dependency-generated ResourceBindings during aggregation.",
			Buckets:   []float64{1, 2, 3, 5, 8, 13, 21},
		},
		[]string{"namespace"},
	)
)

func init() {
	crmetrics.Registry.MustRegister(DependencyPolicyConflictsTotal)
	crmetrics.Registry.MustRegister(DependencyParentsCount)
}
