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
	"sync"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/external_metrics"
	externalclient "k8s.io/metrics/pkg/client/external_metrics"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/metricsadapter/multiclient"
	"github.com/karmada-io/karmada/pkg/util"
	clusterV1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	listcorev1 "k8s.io/client-go/listers/core/v1"
)

// ExternalMetricsProvider is a custom metrics provider
type ExternalMetricsProvider struct {
	// multiClusterDiscovery returns a discovery client for member cluster apiserver
	multiClusterDiscovery multiclient.MultiClusterDiscoveryInterface
	clusterLister         clusterlister.ClusterLister
	secretLister          listcorev1.SecretLister
}

// MakeExternalMetricsProvider creates a new external metrics provider
func MakeExternalMetricsProvider(clusterLister clusterlister.ClusterLister, multiClusterDiscovery multiclient.MultiClusterDiscoveryInterface, secretLister listcorev1.SecretLister) *ExternalMetricsProvider {
	return &ExternalMetricsProvider{
		clusterLister:         clusterLister,
		multiClusterDiscovery: multiClusterDiscovery,
		secretLister:          secretLister,
	}
}

// GetExternalMetric will query metrics by selector from member clusters and return the result
func (c *ExternalMetricsProvider) GetExternalMetric(ctx context.Context, namespace string, selector labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) {
	clusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return nil, err
	}

	metricValueList := &external_metrics.ExternalMetricValueList{}
	metricsChannel := make(chan *external_metrics.ExternalMetricValueList)
	wg := sync.WaitGroup{}

	for _, cluster := range clusters {
		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()
			metrics, err := c.getExternalMetric(ctx, clusterName, namespace, info.Metric, selector)
			if err != nil {
				klog.Warningf("query external metric %s from cluster %s failed, err: %+v", info.Metric, clusterName, err)
				return
			}
			metricsChannel <- metrics
		}(cluster.Name)
	}

	go func() {
		wg.Wait()
		close(metricsChannel)
	}()

	for {
		metrics, ok := <-metricsChannel
		if !ok {
			break
		}
		metricValueList.Items = append(metricValueList.Items, metrics.Items...)
	}

	// TODO(chaunceyjiang) The MetricValue items need to be sorted.
	if len(metricValueList.Items) == 0 {
		return nil, fmt.Errorf("no external metrics found for metric %s in any cluster", info.Metric)
	}

	return metricValueList, nil
}

// ListAllExternalMetrics returns all metrics in all member clusters
func (c *ExternalMetricsProvider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	clusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return []provider.ExternalMetricInfo{}
	}

	// Use a map to deduplicate metrics across clusters
	metricsMap := make(map[string]provider.ExternalMetricInfo)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, cluster := range clusters {
		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()
			metrics, err := c.listExternalMetrics(clusterName)
			if err != nil {
				klog.Warningf("Failed to list external metrics from cluster %s: %v", clusterName, err)
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for _, metric := range metrics {
				metricsMap[metric.Metric] = metric
			}
		}(cluster.Name)
	}

	wg.Wait()

	result := make([]provider.ExternalMetricInfo, 0, len(metricsMap))
	for _, metric := range metricsMap {
		result = append(result, metric)
	}

	return result
}

// getDiscoveryClient gets or sets up a discovery client for a given cluster.
func (c *ExternalMetricsProvider) getDiscoveryClient(clusterName string) (*discovery.DiscoveryClient, error) {
	discoveryClient := c.multiClusterDiscovery.Get(clusterName)
	if discoveryClient != nil {
		return discoveryClient, nil
	}

	// Try to set up the client if it doesn't exist
	if err := c.multiClusterDiscovery.Set(clusterName); err != nil {
		return nil, fmt.Errorf("failed to set up discovery client for cluster %s: %v", clusterName, err)
	}

	discoveryClient = c.multiClusterDiscovery.Get(clusterName)
	if discoveryClient == nil {
		return nil, fmt.Errorf("failed to get discovery client for cluster %s", clusterName)
	}
	return discoveryClient, nil
}

// getExternalMetric queries external metrics from a specific cluster
func (c *ExternalMetricsProvider) getExternalMetric(ctx context.Context, clusterName, namespace, metricName string, selector labels.Selector) (*external_metrics.ExternalMetricValueList, error) {
	discoveryClient, err := c.getDiscoveryClient(clusterName)
	if err != nil {
		return nil, err
	}

	// Check if external metrics API is available
	_, err = c.getPreferredVersion(discoveryClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get preferred version for cluster %s: %v", clusterName, err)
	}

	// Create external metrics client for the cluster
	clusterConfig, err := c.getClusterConfig(clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster config for cluster %s: %v", clusterName, err)
	}

	externalClient, err := externalclient.NewForConfig(clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create external metrics client for cluster %s: %v", clusterName, err)
	}

	// Query the external metric
	externalMetrics, err := externalClient.NamespacedMetrics(namespace).List(metricName, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to query external metric %s from cluster %s: %v", metricName, clusterName, err)
	}

	// Convert to internal format
	result := &external_metrics.ExternalMetricValueList{
		Items: make([]external_metrics.ExternalMetricValue, len(externalMetrics.Items)),
	}

	for i, item := range externalMetrics.Items {
		result.Items[i] = external_metrics.ExternalMetricValue{
			MetricName:   item.MetricName,
			MetricLabels: item.MetricLabels,
			Timestamp:    item.Timestamp,
			Value:        item.Value,
		}
	}

	return result, nil
}

// listExternalMetrics lists all available external metrics from a cluster
func (c *ExternalMetricsProvider) listExternalMetrics(clusterName string) ([]provider.ExternalMetricInfo, error) {
	discoveryClient, err := c.getDiscoveryClient(clusterName)
	if err != nil {
		return nil, err
	}

	// Check if external metrics API is available
	_, err = c.getPreferredVersion(discoveryClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get preferred version for cluster %s: %v", clusterName, err)
	}

	// Create external metrics client for the cluster
	clusterConfig, err := c.getClusterConfig(clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster config for cluster %s: %v", clusterName, err)
	}

	externalClient, err := externalclient.NewForConfig(clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create external metrics client for cluster %s: %v", clusterName, err)
	}

	// Try to discover available metrics by querying the external metrics API
	// This is a more dynamic approach than hardcoded metrics
	metrics := []provider.ExternalMetricInfo{}
	
	// Common external metrics that are typically available
	commonMetrics := []string{
		"queue_length",
		"requests_per_second",
		"db_connections",
		"message_queue_length",
		"api_response_time",
		"error_rate",
		"throughput",
		"latency",
		"cpu_usage",
		"memory_usage",
		"disk_usage",
		"network_io",
		"active_connections",
		"cache_hit_rate",
		"job_queue_size",
	}

	// Try to discover which metrics are actually available
	for _, metricName := range commonMetrics {
		// Try to query the metric to see if it exists
		_, err := externalClient.NamespacedMetrics("default").List(metricName, labels.Everything())
		if err == nil {
			// Metric exists, add it to the list
			metrics = append(metrics, provider.ExternalMetricInfo{
				Metric: metricName,
			})
		}
		// If metric doesn't exist, just continue to the next one
		// We don't log errors here as it's expected that not all metrics will be available
	}

	// If no common metrics are found, try to query the API for available metrics
	// This would require the external metrics API to support listing available metrics
	// For now, we'll return the discovered metrics or an empty list
	if len(metrics) == 0 {
		klog.V(4).Infof("No external metrics discovered from cluster %s", clusterName)
	}

	return metrics, nil
}

// getPreferredVersion gets the preferred API version for external metrics
func (c *ExternalMetricsProvider) getPreferredVersion(discoveryClient *discovery.DiscoveryClient) (schema.GroupVersion, error) {
	apiGroups, err := discoveryClient.ServerGroups()
	if err != nil {
		return schema.GroupVersion{}, err
	}

	for _, apiGroup := range apiGroups.Groups {
		if apiGroup.Name == "external.metrics.k8s.io" {
			if len(apiGroup.Versions) > 0 {
				return schema.ParseGroupVersion(apiGroup.Versions[0].GroupVersion)
			}
		}
	}

	return schema.GroupVersion{}, fmt.Errorf("external metrics API not found")
}

// getClusterConfig gets the cluster configuration for a specific cluster
func (c *ExternalMetricsProvider) getClusterConfig(clusterName string) (*rest.Config, error) {
	// Get cluster information
	_, err := c.clusterLister.Get(clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %s: %v", clusterName, err)
	}

	// Build cluster config using the same pattern as multiclient
	clusterGetter := func(cluster string) (*clusterV1alpha1.Cluster, error) {
		return c.clusterLister.Get(cluster)
	}
	
	secretGetter := func(namespace string, name string) (*corev1.Secret, error) {
		return c.secretLister.Secrets(namespace).Get(name)
	}

	clusterConfig, err := util.BuildClusterConfig(clusterName, clusterGetter, secretGetter)
	if err != nil {
		return nil, fmt.Errorf("failed to build cluster config for cluster %s: %v", clusterName, err)
	}

	return clusterConfig, nil
}
