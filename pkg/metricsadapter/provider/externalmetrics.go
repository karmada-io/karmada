package provider

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/metrics/pkg/apis/external_metrics"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
)

// ExternalMetricsProvider is a custom metrics provider
type ExternalMetricsProvider struct {
}

// MakeExternalMetricsProvider creates a new custom metrics provider
func MakeExternalMetricsProvider() *ExternalMetricsProvider {
	return &ExternalMetricsProvider{}
}

// GetExternalMetric will query metrics by selector from member clusters and return the result
func (c *ExternalMetricsProvider) GetExternalMetric(ctx context.Context, namespace string, metricSelector labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) {
	return nil, fmt.Errorf("karmada-metrics-adapter still not implement it")
}

// ListAllExternalMetrics returns all metrics in all member clusters
func (c *ExternalMetricsProvider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	return []provider.ExternalMetricInfo{}
}
