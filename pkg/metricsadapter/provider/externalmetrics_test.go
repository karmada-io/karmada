/*
Copyright 2026 The Karmada Authors.

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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/metrics/pkg/apis/external_metrics"
	externalmetricsv1beta1 "k8s.io/metrics/pkg/apis/external_metrics/v1beta1"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/metricsadapter/multiclient"
)

// fakeMultiClusterDiscovery returns a preconfigured discovery client per cluster.
type fakeMultiClusterDiscovery struct {
	clients map[string]*discovery.DiscoveryClient
}

func (f *fakeMultiClusterDiscovery) Get(clusterName string) *discovery.DiscoveryClient {
	return f.clients[clusterName]
}

func (f *fakeMultiClusterDiscovery) Set(string) error { return nil }

func (f *fakeMultiClusterDiscovery) Remove(string) {}

var _ multiclient.MultiClusterDiscoveryInterface = (*fakeMultiClusterDiscovery)(nil)

func newClusterLister(t *testing.T, names ...string) clusterlister.ClusterLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, name := range names {
		require.NoError(t, indexer.Add(&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name}}))
	}
	return clusterlister.NewClusterLister(indexer)
}

func mustQuantity(t *testing.T, s string) resource.Quantity {
	t.Helper()
	q, err := resource.ParseQuantity(s)
	require.NoError(t, err)
	return q
}

// fakeMemberCluster serves the external metrics API of a single member cluster.
type fakeMemberCluster struct {
	// metrics maps a metric name to the values returned by GetExternalMetric.
	metrics map[string][]externalmetricsv1beta1.ExternalMetricValue
	// resources lists the metric names advertised through API discovery.
	resources []string
}

func startMemberCluster(t *testing.T, cluster fakeMemberCluster) *discovery.DiscoveryClient {
	t.Helper()
	const apiPrefix = "/apis/external.metrics.k8s.io/v1beta1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == apiPrefix:
			list := &metav1.APIResourceList{GroupVersion: "external.metrics.k8s.io/v1beta1"}
			for _, name := range cluster.resources {
				list.APIResources = append(list.APIResources, metav1.APIResource{Name: name, Namespaced: true})
			}
			require.NoError(t, json.NewEncoder(w).Encode(list))
		case strings.HasPrefix(r.URL.Path, apiPrefix+"/namespaces/"):
			metricName := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			list := &externalmetricsv1beta1.ExternalMetricValueList{
				TypeMeta: metav1.TypeMeta{APIVersion: "external.metrics.k8s.io/v1beta1", Kind: "ExternalMetricValueList"},
				Items:    cluster.metrics[metricName],
			}
			require.NoError(t, json.NewEncoder(w).Encode(list))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return discovery.NewDiscoveryClientForConfigOrDie(&rest.Config{Host: server.URL})
}

func TestGetExternalMetric(t *testing.T) {
	const namespace = "default"
	const metricName = "queue_messages"

	tests := []struct {
		name      string
		clusters  map[string]fakeMemberCluster
		wantErr   bool
		wantItems map[string]string // metric label "queue" value -> expected quantity
	}{
		{
			name: "values with the same labels are summed across clusters",
			clusters: map[string]fakeMemberCluster{
				"member1": {metrics: map[string][]externalmetricsv1beta1.ExternalMetricValue{
					metricName: {{MetricName: metricName, MetricLabels: map[string]string{"queue": "worker"}, Value: mustQuantity(t, "10")}},
				}},
				"member2": {metrics: map[string][]externalmetricsv1beta1.ExternalMetricValue{
					metricName: {{MetricName: metricName, MetricLabels: map[string]string{"queue": "worker"}, Value: mustQuantity(t, "5")}},
				}},
			},
			wantItems: map[string]string{"worker": "15"},
		},
		{
			name: "values with different labels are kept separate",
			clusters: map[string]fakeMemberCluster{
				"member1": {metrics: map[string][]externalmetricsv1beta1.ExternalMetricValue{
					metricName: {{MetricName: metricName, MetricLabels: map[string]string{"queue": "worker"}, Value: mustQuantity(t, "10")}},
				}},
				"member2": {metrics: map[string][]externalmetricsv1beta1.ExternalMetricValue{
					metricName: {{MetricName: metricName, MetricLabels: map[string]string{"queue": "batch"}, Value: mustQuantity(t, "7")}},
				}},
			},
			wantItems: map[string]string{"worker": "10", "batch": "7"},
		},
		{
			name: "no values from any cluster returns a not-found error",
			clusters: map[string]fakeMemberCluster{
				"member1": {metrics: map[string][]externalmetricsv1beta1.ExternalMetricValue{}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clients := map[string]*discovery.DiscoveryClient{}
			names := make([]string, 0, len(tt.clusters))
			for name, cluster := range tt.clusters {
				clients[name] = startMemberCluster(t, cluster)
				names = append(names, name)
			}
			p := MakeExternalMetricsProvider(newClusterLister(t, names...), &fakeMultiClusterDiscovery{clients: clients})

			got, err := p.GetExternalMetric(context.Background(), namespace, labels.Everything(), provider.ExternalMetricInfo{Metric: metricName})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			gotItems := map[string]string{}
			for _, item := range got.Items {
				gotItems[item.MetricLabels["queue"]] = item.Value.String()
			}
			assert.Equal(t, tt.wantItems, gotItems)
		})
	}
}

func TestListAllExternalMetrics(t *testing.T) {
	clients := map[string]*discovery.DiscoveryClient{
		"member1": startMemberCluster(t, fakeMemberCluster{resources: []string{"queue_messages", "http_requests"}}),
		"member2": startMemberCluster(t, fakeMemberCluster{resources: []string{"queue_messages", "rabbitmq_queue"}}),
	}
	p := MakeExternalMetricsProvider(newClusterLister(t, "member1", "member2"), &fakeMultiClusterDiscovery{clients: clients})

	got := p.ListAllExternalMetrics()

	names := make([]string, 0, len(got))
	for _, info := range got {
		names = append(names, info.Metric)
	}
	// Metrics are deduplicated across clusters and returned in sorted order.
	assert.Equal(t, []string{"http_requests", "queue_messages", "rabbitmq_queue"}, names)
}

func TestExternalMetricKey(t *testing.T) {
	withLabels := external_metrics.ExternalMetricValue{
		MetricName:   "queue_messages",
		MetricLabels: map[string]string{"queue": "worker", "region": "eu"},
	}
	// Label order in the map must not change the key.
	reordered := external_metrics.ExternalMetricValue{
		MetricName:   "queue_messages",
		MetricLabels: map[string]string{"region": "eu", "queue": "worker"},
	}
	assert.Equal(t, externalMetricKey(withLabels), externalMetricKey(reordered))
	assert.NotEqual(t, externalMetricKey(withLabels), externalMetricKey(external_metrics.ExternalMetricValue{MetricName: "queue_messages"}))
}
