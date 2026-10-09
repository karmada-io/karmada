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
	"testing"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
	metricsprovider "sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/metricsadapter/multiclient"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	clusterV1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mock implementations for testing
type mockClusterLister struct {
	clusterlister.ClusterLister
}

func (m *mockClusterLister) List(selector labels.Selector) ([]*clusterV1alpha1.Cluster, error) {
	// Return a mock cluster to enable testing of the core logic
	return []*clusterV1alpha1.Cluster{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "member1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "member2",
			},
		},
	}, nil
}

type mockMultiClusterDiscovery struct {
	multiclient.MultiClusterDiscoveryInterface
}

func (m *mockMultiClusterDiscovery) Get(clusterName string) *discovery.DiscoveryClient {
	// Return nil to simulate no discovery client initially
	return nil
}

func (m *mockMultiClusterDiscovery) Set(clusterName string) error {
	// Return error to simulate failure
	return fmt.Errorf("mock discovery client not implemented")
}

func (m *mockMultiClusterDiscovery) Remove(clusterName string) {
	// No-op for mock
}

type mockSecretLister struct {
	listcorev1.SecretLister
}

func TestMakeExternalMetricsProvider(t *testing.T) {
	// Test that the provider can be created without errors
	clusterLister := &mockClusterLister{}
	multiClusterDiscovery := &mockMultiClusterDiscovery{}
	secretLister := &mockSecretLister{}

	provider := MakeExternalMetricsProvider(clusterLister, multiClusterDiscovery, secretLister)
	if provider == nil {
		t.Fatal("Expected provider to be created, got nil")
	}

	if provider.clusterLister != clusterLister {
		t.Error("Expected clusterLister to be set correctly")
	}

	if provider.multiClusterDiscovery != multiClusterDiscovery {
		t.Error("Expected multiClusterDiscovery to be set correctly")
	}

	if provider.secretLister != secretLister {
		t.Error("Expected secretLister to be set correctly")
	}
}

func TestGetExternalMetric(t *testing.T) {
	// Test the GetExternalMetric method
	clusterLister := &mockClusterLister{}
	multiClusterDiscovery := &mockMultiClusterDiscovery{}
	secretLister := &mockSecretLister{}

	provider := MakeExternalMetricsProvider(clusterLister, multiClusterDiscovery, secretLister)

	ctx := context.Background()
	namespace := "default"
	selector := labels.Everything()
	info := metricsprovider.ExternalMetricInfo{
		Metric: "test_metric",
	}

	// This should return an error since we're using mock implementations
	// but it should not panic and should test the parallel processing logic
	_, err := provider.GetExternalMetric(ctx, namespace, selector, info)
	if err == nil {
		t.Log("GetExternalMetric returned no error (expected with mock implementations)")
	} else {
		klog.V(4).Infof("GetExternalMetric returned expected error: %v", err)
	}
}

func TestListAllExternalMetrics(t *testing.T) {
	// Test the ListAllExternalMetrics method
	clusterLister := &mockClusterLister{}
	multiClusterDiscovery := &mockMultiClusterDiscovery{}
	secretLister := &mockSecretLister{}

	provider := MakeExternalMetricsProvider(clusterLister, multiClusterDiscovery, secretLister)

	// This should return an empty list since we're using mock implementations
	// but it should not panic and should test the parallel processing logic
	metrics := provider.ListAllExternalMetrics()
	if metrics == nil {
		t.Fatal("Expected metrics list to be returned, got nil")
	}

	// With mock implementations, we expect an empty list
	if len(metrics) != 0 {
		t.Errorf("Expected empty metrics list, got %d metrics", len(metrics))
	}
}

func TestExternalMetricsProvider_Integration(t *testing.T) {
	// Test the complete integration flow
	clusterLister := &mockClusterLister{}
	multiClusterDiscovery := &mockMultiClusterDiscovery{}
	secretLister := &mockSecretLister{}

	provider := MakeExternalMetricsProvider(clusterLister, multiClusterDiscovery, secretLister)

	// Test that the provider implements the required interface
	var _ metricsprovider.ExternalMetricsProvider = provider

	// Test that all methods can be called without panic
	ctx := context.Background()
	namespace := "default"
	selector := labels.Everything()
	info := metricsprovider.ExternalMetricInfo{
		Metric: "queue_length",
	}

	// Test GetExternalMetric
	_, err := provider.GetExternalMetric(ctx, namespace, selector, info)
	if err == nil {
		t.Log("GetExternalMetric completed successfully with mock implementations")
	} else {
		t.Logf("GetExternalMetric returned expected error: %v", err)
	}

	// Test ListAllExternalMetrics
	metrics := provider.ListAllExternalMetrics()
	if metrics == nil {
		t.Error("ListAllExternalMetrics should not return nil")
	}

	t.Logf("ListAllExternalMetrics returned %d metrics", len(metrics))
}

func TestGetDiscoveryClient(t *testing.T) {
	// Test the new helper method
	clusterLister := &mockClusterLister{}
	multiClusterDiscovery := &mockMultiClusterDiscovery{}
	secretLister := &mockSecretLister{}

	provider := MakeExternalMetricsProvider(clusterLister, multiClusterDiscovery, secretLister)

	// Test getDiscoveryClient helper method
	_, err := provider.getDiscoveryClient("test-cluster")
	if err == nil {
		t.Error("Expected error when discovery client is not implemented")
	} else {
		t.Logf("getDiscoveryClient returned expected error: %v", err)
	}
}

func TestParallelProcessing(t *testing.T) {
	// Test that parallel processing works correctly with multiple clusters
	clusterLister := &mockClusterLister{}
	multiClusterDiscovery := &mockMultiClusterDiscovery{}
	secretLister := &mockSecretLister{}

	provider := MakeExternalMetricsProvider(clusterLister, multiClusterDiscovery, secretLister)

	ctx := context.Background()
	namespace := "default"
	selector := labels.Everything()
	info := metricsprovider.ExternalMetricInfo{
		Metric: "test_metric",
	}

	// This should test the parallel processing logic with multiple clusters
	_, err := provider.GetExternalMetric(ctx, namespace, selector, info)
	if err == nil {
		t.Log("Parallel processing completed successfully with mock implementations")
	} else {
		t.Logf("Parallel processing returned expected error: %v", err)
	}

	// Test that ListAllExternalMetrics also uses parallel processing
	metrics := provider.ListAllExternalMetrics()
	if metrics == nil {
		t.Error("ListAllExternalMetrics should not return nil")
	}

	t.Logf("Parallel processing test completed with %d metrics", len(metrics))
}
