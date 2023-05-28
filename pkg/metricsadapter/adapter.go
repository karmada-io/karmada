package metricsadapter

import (
	basecmd "sigs.k8s.io/custom-metrics-apiserver/pkg/cmd"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/cmd/options"

	"github.com/karmada-io/karmada/pkg/metricsadapter/provider"
)

// MetricsAdapter is a metrics adapter to provider native metrics, custom metrics and external metrics
type MetricsAdapter struct {
	basecmd.AdapterBase

	*provider.ResourceMetricsProvider
}

// NewMetricsAdapter creates a new metrics adapter
func NewMetricsAdapter(controller *MetricsController, customMetricsAdapterServerOptions *options.CustomMetricsAdapterServerOptions) *MetricsAdapter {
	adapter := &MetricsAdapter{}
	adapter.CustomMetricsAdapterServerOptions = customMetricsAdapterServerOptions

	adapter.ResourceMetricsProvider = provider.NewResourceMetricsProvider(controller.ClusterLister, controller.InformerManager)
	customProvider := provider.MakeCustomMetricsProvider()
	externalProvider := provider.MakeExternalMetricsProvider()
	adapter.WithCustomMetrics(customProvider)
	adapter.WithExternalMetrics(externalProvider)

	return adapter
}
