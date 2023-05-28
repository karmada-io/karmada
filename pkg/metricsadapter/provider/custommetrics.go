package provider

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/metrics/pkg/apis/custom_metrics"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
)

// CustomMetricsProvider is a custom metrics provider
type CustomMetricsProvider struct {
}

// MakeCustomMetricsProvider creates a new custom metrics provider
func MakeCustomMetricsProvider() *CustomMetricsProvider {
	return &CustomMetricsProvider{}
}

// GetMetricByName will query metrics by name from member clusters and return the result
func (c *CustomMetricsProvider) GetMetricByName(ctx context.Context, name types.NamespacedName, info provider.CustomMetricInfo, metricSelector labels.Selector) (*custom_metrics.MetricValue, error) {
	return nil, fmt.Errorf("karmada-metrics-adapter still not implement it")
}

// GetMetricBySelector will query metrics by selector from member clusters and return the result
func (c *CustomMetricsProvider) GetMetricBySelector(ctx context.Context, namespace string, selector labels.Selector, info provider.CustomMetricInfo, metricSelector labels.Selector) (*custom_metrics.MetricValueList, error) {
	return nil, fmt.Errorf("karmada-metrics-adapter still not implement it")
}

// ListAllMetrics returns all metrics in all member clusters
func (c *CustomMetricsProvider) ListAllMetrics() []provider.CustomMetricInfo {
	return []provider.CustomMetricInfo{}
}
