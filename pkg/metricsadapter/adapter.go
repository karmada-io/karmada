/*
Copyright 2023 The Karmada Authors.

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
	adapter.ResourceMetricsProvider = provider.NewResourceMetricsProvider(controller.ClusterLister, controller.TypedInformerManager, controller.InformerManager)
	customProvider := provider.MakeCustomMetricsProvider(controller.ClusterLister, controller.MultiClusterDiscovery)
	externalProvider := provider.MakeExternalMetricsProvider()
	adapter.WithCustomMetrics(customProvider)
	adapter.WithExternalMetrics(externalProvider)

	return adapter
}
