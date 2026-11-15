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
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/external_metrics"
	externalmetricsv1beta1 "k8s.io/metrics/pkg/apis/external_metrics/v1beta1"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/metricsadapter/multiclient"
)

// ExternalMetricsProvider is an external metrics provider that serves the
// external metrics of the member clusters.
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

// GetExternalMetric queries the metric from every member cluster that serves the
// external metrics API and returns the values of all of them. The HPA algorithm
// sums the returned values, so a metric reported by several clusters is added up.
//
// Clusters without a client or without the metric are skipped. Any other failure
// fails the whole request, since a sum that silently misses a cluster looks like
// lower load and could make the autoscaler scale down during an outage.
func (c *ExternalMetricsProvider) GetExternalMetric(ctx context.Context, namespace string, metricSelector labels.Selector, info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) {
	clusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return nil, err
	}

	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		result    = &external_metrics.ExternalMetricValueList{}
		errs      []error
		succeeded int
	)
	for _, cluster := range clusters {
		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()
			metrics, err := c.getExternalMetric(ctx, clusterName, namespace, metricSelector, info.Metric)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				succeeded++
				result.Items = append(result.Items, metrics.Items...)
			case errors.Is(err, errNoDiscoveryClient) || apierrors.IsNotFound(err):
				klog.V(4).Infof("cluster %s has no external metric %s: %v", clusterName, info.Metric, err)
			default:
				errs = append(errs, fmt.Errorf("cluster %s: %w", clusterName, err))
			}
		}(cluster.Name)
	}
	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to query external metric %s from member clusters: %w", info.Metric, utilerrors.NewAggregate(errs))
	}
	if succeeded == 0 {
		return nil, fmt.Errorf("external metric %s is not available from any member cluster", info.Metric)
	}
	return result, nil
}

var errNoDiscoveryClient = errors.New("no discovery client for the cluster")

func (c *ExternalMetricsProvider) getExternalMetric(ctx context.Context, clusterName, namespace string, metricSelector labels.Selector, metricName string) (*external_metrics.ExternalMetricValueList, error) {
	discoveryClient := c.multiClusterDiscovery.Get(clusterName)
	if discoveryClient == nil {
		return nil, errNoDiscoveryClient
	}

	if metricSelector == nil {
		metricSelector = labels.Everything()
	}
	data, err := discoveryClient.RESTClient().Get().
		AbsPath("/apis", externalmetricsv1beta1.SchemeGroupVersion.String(), "namespaces", namespace, metricName).
		Param("labelSelector", metricSelector.String()).
		DoRaw(ctx)
	if err != nil {
		return nil, err
	}

	versioned := &externalmetricsv1beta1.ExternalMetricValueList{}
	if err := json.Unmarshal(data, versioned); err != nil {
		return nil, fmt.Errorf("failed to decode external metric %s: %w", metricName, err)
	}
	metrics := &external_metrics.ExternalMetricValueList{}
	if err := externalmetricsv1beta1.Convert_v1beta1_ExternalMetricValueList_To_external_metrics_ExternalMetricValueList(versioned, metrics, nil); err != nil {
		return nil, err
	}
	return metrics, nil
}

// ListAllExternalMetrics returns the external metrics served by the member clusters
func (c *ExternalMetricsProvider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	clusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return []provider.ExternalMetricInfo{}
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		metrics = sets.New[string]()
	)
	for _, cluster := range clusters {
		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()
			discoveryClient := c.multiClusterDiscovery.Get(clusterName)
			if discoveryClient == nil {
				return
			}
			resources, err := discoveryClient.ServerResourcesForGroupVersion(externalmetricsv1beta1.SchemeGroupVersion.String())
			if err != nil {
				klog.V(4).Infof("cluster %s doesn't serve %s: %v", clusterName, externalmetricsv1beta1.SchemeGroupVersion.String(), err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, resource := range resources.APIResources {
				metrics.Insert(resource.Name)
			}
		}(cluster.Name)
	}
	wg.Wait()

	names := sets.List(metrics)
	infos := make([]provider.ExternalMetricInfo, 0, len(names))
	for _, name := range names {
		infos = append(infos, provider.ExternalMetricInfo{Metric: name})
	}
	return infos
}
