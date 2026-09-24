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
	"sort"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/external_metrics"
	externalmetricsv1beta1 "k8s.io/metrics/pkg/apis/external_metrics/v1beta1"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/metricsadapter/multiclient"
)

var (
	externalMetricsScheme = runtime.NewScheme()
	externalMetricsCodecs = serializer.NewCodecFactory(externalMetricsScheme)
)

func init() {
	utilruntime.Must(externalmetricsv1beta1.AddToScheme(externalMetricsScheme))
}

// ExternalMetricsProvider queries external metrics from member clusters.
type ExternalMetricsProvider struct {
	// multiClusterDiscovery returns a discovery client for member cluster apiserver
	multiClusterDiscovery multiclient.MultiClusterDiscoveryInterface
	clusterLister         clusterlister.ClusterLister
}

// MakeExternalMetricsProvider creates a new external metrics provider
func MakeExternalMetricsProvider(clusterLister clusterlister.ClusterLister, multiClusterDiscovery multiclient.MultiClusterDiscoveryInterface) *ExternalMetricsProvider {
	return &ExternalMetricsProvider{
		clusterLister:         clusterLister,
		multiClusterDiscovery: multiClusterDiscovery,
	}
}

// GetExternalMetric will query metrics by selector from member clusters and return the result
func (c *ExternalMetricsProvider) GetExternalMetric(ctx context.Context, namespace string, metricSelector labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) {
	clusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return nil, err
	}
	metricsChannel := make(chan *external_metrics.ExternalMetricValueList)
	wg := sync.WaitGroup{}
	for _, cluster := range clusters {
		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()
			metrics, err := c.getExternalMetric(ctx, clusterName, namespace, metricSelector, info)
			if err != nil {
				klog.Warningf("query %s external metric from cluster %s failed, err: %+v", info.Metric, clusterName, err)
				return
			}
			metricsChannel <- metrics
		}(cluster.Name)
	}
	go func() {
		wg.Wait()
		close(metricsChannel)
	}()

	// A single metric may be reported by multiple clusters. Values that share the same
	// labels are summed, the same way the custom metrics provider folds duplicates.
	merged := make(map[string]external_metrics.ExternalMetricValue)
	for metrics := range metricsChannel {
		for _, item := range metrics.Items {
			key := externalMetricKey(item)
			if existing, ok := merged[key]; ok {
				existing.Value.Add(item.Value)
				merged[key] = existing
				continue
			}
			merged[key] = item
		}
	}
	if len(merged) == 0 {
		return nil, provider.NewMetricNotFoundError(externalmetricsv1beta1.Resource(info.Metric), info.Metric)
	}

	metricValueList := &external_metrics.ExternalMetricValueList{Items: make([]external_metrics.ExternalMetricValue, 0, len(merged))}
	for _, item := range merged {
		metricValueList.Items = append(metricValueList.Items, item)
	}
	return metricValueList, nil
}

func (c *ExternalMetricsProvider) getExternalMetric(ctx context.Context, clusterName, namespace string, metricSelector labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) {
	discoveryClient := c.multiClusterDiscovery.Get(clusterName)
	if discoveryClient == nil {
		err := fmt.Errorf("failed to get MultiClusterDiscovery for cluster(%s)", clusterName)
		klog.Error(err)
		return nil, err
	}
	gv := externalmetricsv1beta1.SchemeGroupVersion
	raw, err := discoveryClient.RESTClient().Get().
		Prefix("apis", gv.Group, gv.Version).
		Namespace(namespace).
		Resource(info.Metric).
		VersionedParams(&metav1.ListOptions{LabelSelector: metricSelector.String()}, metav1.ParameterCodec).
		DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	versioned := &externalmetricsv1beta1.ExternalMetricValueList{}
	if err := runtime.DecodeInto(externalMetricsCodecs.UniversalDeserializer(), raw, versioned); err != nil {
		return nil, err
	}
	result := &external_metrics.ExternalMetricValueList{}
	if err := externalmetricsv1beta1.Convert_v1beta1_ExternalMetricValueList_To_external_metrics_ExternalMetricValueList(versioned, result, nil); err != nil {
		return nil, err
	}
	return result, nil
}

// externalMetricKey builds a stable identity for a metric value from its name and labels.
func externalMetricKey(value external_metrics.ExternalMetricValue) string {
	if len(value.MetricLabels) == 0 {
		return value.MetricName
	}
	keys := make([]string, 0, len(value.MetricLabels))
	for k := range value.MetricLabels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(value.MetricName)
	for _, k := range keys {
		b.WriteString(",")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(value.MetricLabels[k])
	}
	return b.String()
}

// ListAllExternalMetrics returns all metrics in all member clusters
func (c *ExternalMetricsProvider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	clusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return []provider.ExternalMetricInfo{}
	}
	metricNameChan := make(chan string)
	wg := sync.WaitGroup{}
	gv := externalmetricsv1beta1.SchemeGroupVersion
	for _, cluster := range clusters {
		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()
			discoveryClient := c.multiClusterDiscovery.Get(clusterName)
			if discoveryClient == nil {
				klog.Errorf("failed to get MultiClusterDiscovery for cluster(%s)", clusterName)
				return
			}
			resources, err := discoveryClient.ServerResourcesForGroupVersion(gv.String())
			if err != nil {
				klog.Warningf("Failed to query %s resource in cluster(%s): %+v", gv.String(), clusterName, err)
				return
			}
			for _, resource := range resources.APIResources {
				metricNameChan <- resource.Name
			}
		}(cluster.Name)
	}
	go func() {
		wg.Wait()
		close(metricNameChan)
	}()

	metricNames := sets.New[string]()
	for name := range metricNameChan {
		metricNames.Insert(name)
	}
	externalMetricInfos := make([]provider.ExternalMetricInfo, 0, metricNames.Len())
	for _, name := range sets.List(metricNames) {
		externalMetricInfos = append(externalMetricInfos, provider.ExternalMetricInfo{Metric: name})
	}
	return externalMetricInfos
}
