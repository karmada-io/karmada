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
func (c *ExternalMetricsProvider) GetExternalMetric(_ context.Context, _ string, _ labels.Selector, _ provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) {
	return nil, fmt.Errorf("karmada-metrics-adapter still not implement it")
}

// ListAllExternalMetrics returns all metrics in all member clusters
func (c *ExternalMetricsProvider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	return []provider.ExternalMetricInfo{}
}
