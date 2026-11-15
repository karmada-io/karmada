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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	externalmetricsv1beta1 "k8s.io/metrics/pkg/apis/external_metrics/v1beta1"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
)

type fakeMultiClusterDiscovery map[string]*discovery.DiscoveryClient

func (f fakeMultiClusterDiscovery) Get(clusterName string) *discovery.DiscoveryClient {
	return f[clusterName]
}
func (f fakeMultiClusterDiscovery) Set(string) error { return nil }
func (f fakeMultiClusterDiscovery) Remove(string)    {}

// newMemberAPIServer serves the external metrics API of a member cluster with
// one metric, queue_length, that has the given value in namespace default.
func newMemberAPIServer(t *testing.T, value int64) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/apis/external.metrics.k8s.io/v1beta1":
			_ = json.NewEncoder(w).Encode(metav1.APIResourceList{
				GroupVersion: "external.metrics.k8s.io/v1beta1",
				APIResources: []metav1.APIResource{{Name: "queue_length", Namespaced: true, Kind: "ExternalMetricValueList"}},
			})
		case "/apis/external.metrics.k8s.io/v1beta1/namespaces/default/queue_length":
			assert.Equal(t, "queue=orders", r.URL.Query().Get("labelSelector"))
			_ = json.NewEncoder(w).Encode(externalmetricsv1beta1.ExternalMetricValueList{
				Items: []externalmetricsv1beta1.ExternalMetricValue{{
					MetricName:   "queue_length",
					MetricLabels: map[string]string{"queue": "orders"},
					Value:        *resource.NewQuantity(value, resource.DecimalSI),
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newExternalMetricsProvider(t *testing.T, servers map[string]*httptest.Server) *ExternalMetricsProvider {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	clients := fakeMultiClusterDiscovery{}
	for name, server := range servers {
		require.NoError(t, indexer.Add(&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name}}))
		if server != nil {
			clients[name] = discovery.NewDiscoveryClientForConfigOrDie(&rest.Config{Host: server.URL})
		}
	}
	return MakeExternalMetricsProvider(clusterlister.NewClusterLister(indexer), clients)
}

func TestGetExternalMetricFromMemberClusters(t *testing.T) {
	p := newExternalMetricsProvider(t, map[string]*httptest.Server{
		"member1": newMemberAPIServer(t, 3),
		"member2": newMemberAPIServer(t, 4),
		// no discovery client, e.g. a cluster that isn't ready yet
		"member3": nil,
	})
	selector := labels.SelectorFromSet(labels.Set{"queue": "orders"})

	metrics, err := p.GetExternalMetric(context.TODO(), "default", selector, provider.ExternalMetricInfo{Metric: "queue_length"})
	require.NoError(t, err)

	require.Len(t, metrics.Items, 2)
	var total int64
	for _, item := range metrics.Items {
		assert.Equal(t, "queue_length", item.MetricName)
		total += item.Value.Value()
	}
	assert.Equal(t, int64(7), total)
}

func TestGetExternalMetricNotServedByAnyCluster(t *testing.T) {
	p := newExternalMetricsProvider(t, map[string]*httptest.Server{
		"member1": newMemberAPIServer(t, 3),
	})

	_, err := p.GetExternalMetric(context.TODO(), "default", labels.Everything(), provider.ExternalMetricInfo{Metric: "missing"})
	assert.Error(t, err)
}

func TestListAllExternalMetrics(t *testing.T) {
	p := newExternalMetricsProvider(t, map[string]*httptest.Server{
		"member1": newMemberAPIServer(t, 3),
		"member2": newMemberAPIServer(t, 4),
	})

	assert.Equal(t, []provider.ExternalMetricInfo{{Metric: "queue_length"}}, p.ListAllExternalMetrics())
}
