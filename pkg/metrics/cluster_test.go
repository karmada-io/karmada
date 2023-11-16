/*
Copyright 2022 The Karmada Authors.

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

package metrics

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/metrics/testutil"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

func TestClusterReadyMetrics(t *testing.T) {
	tests := []struct {
		name    string
		cluster *clusterv1alpha1.Cluster
		want    string
	}{
		{
			name: "cluster ready",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					SyncMode: clusterv1alpha1.Push,
				},
				Status: clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   clusterv1alpha1.ClusterConditionReady,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			want: `
# HELP cluster_ready_state State of the cluster(1 if ready, 0 otherwise).
# TYPE cluster_ready_state gauge
cluster_ready_state{cluster_name="foo"} 1
`,
		},
		{
			name: "cluster not ready",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					SyncMode: clusterv1alpha1.Pull,
				},
				Status: clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{
						{},
					},
				},
			},
			want: `
# HELP cluster_ready_state State of the cluster(1 if ready, 0 otherwise).
# TYPE cluster_ready_state gauge
cluster_ready_state{cluster_name="foo"} 0
`,
		},
	}
	for _, test := range tests {
		clusterReadyGauge.Reset()
		RecordClusterStatus(test.cluster)
		if err := testutil.CollectAndCompare(clusterReadyGauge, strings.NewReader(test.want), clusterReadyMetricsName); err != nil {
			t.Errorf("unexpected collecting result:\n%s", err)
		}
	}
}

func TestClusterTotalNodeNumberMetrics(t *testing.T) {
	testCluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: clusterv1alpha1.ClusterSpec{
			SyncMode: clusterv1alpha1.Push,
		},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1alpha1.ClusterConditionReady,
					Status: metav1.ConditionTrue,
				},
			},
			NodeSummary: &clusterv1alpha1.NodeSummary{
				TotalNum: 100,
			},
		},
	}
	want := `
# HELP cluster_node_number Number of nodes in the cluster.
# TYPE cluster_node_number gauge
cluster_node_number{cluster_name="foo"} 100
`
	clusterTotalNodeNumberGauge.Reset()
	RecordClusterStatus(testCluster)
	if err := testutil.CollectAndCompare(clusterTotalNodeNumberGauge, strings.NewReader(want), clusterTotalNodeNumberMetricsName); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestClusterReadyNodeNumberMetrics(t *testing.T) {
	testCluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: clusterv1alpha1.ClusterSpec{
			SyncMode: clusterv1alpha1.Push,
		},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1alpha1.ClusterConditionReady,
					Status: metav1.ConditionTrue,
				},
			},
			NodeSummary: &clusterv1alpha1.NodeSummary{
				TotalNum: 100,
				ReadyNum: 10,
			},
		},
	}
	want := `
# HELP cluster_ready_node_number Number of ready nodes in the cluster.
# TYPE cluster_ready_node_number gauge
cluster_ready_node_number{cluster_name="foo"} 10
`
	clusterReadyNodeNumberGauge.Reset()
	RecordClusterStatus(testCluster)
	if err := testutil.CollectAndCompare(clusterReadyNodeNumberGauge, strings.NewReader(want), clusterReadyNodeNumberMetricsName); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestClusterMemoryAllocatableMetrics(t *testing.T) {
	testCluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: clusterv1alpha1.ClusterSpec{
			SyncMode: clusterv1alpha1.Push,
		},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1alpha1.ClusterConditionReady,
					Status: metav1.ConditionTrue,
				},
			},
			ResourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocatable: corev1.ResourceList{
					corev1.ResourceMemory: *resource.NewQuantity(200, resource.DecimalSI),
				},
			},
		},
	}
	want := `
# HELP cluster_memory_allocatable_bytes Allocatable cluster memory resource in bytes.
# TYPE cluster_memory_allocatable_bytes gauge
cluster_memory_allocatable_bytes{cluster_name="foo"} 200
`
	clusterMemoryAllocatableGauge.Reset()
	RecordClusterStatus(testCluster)
	if err := testutil.CollectAndCompare(clusterMemoryAllocatableGauge, strings.NewReader(want), clusterMemoryAllocatableMetricsName); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestClusterCPUAllocatableMetrics(t *testing.T) {
	testCluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: clusterv1alpha1.ClusterSpec{
			SyncMode: clusterv1alpha1.Push,
		},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1alpha1.ClusterConditionReady,
					Status: metav1.ConditionTrue,
				},
			},
			ResourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU: *resource.NewMilliQuantity(200, resource.DecimalSI),
				},
			},
		},
	}
	want := `
# HELP cluster_cpu_allocatable_number Number of allocatable CPU in the cluster.
# TYPE cluster_cpu_allocatable_number gauge
cluster_cpu_allocatable_number{cluster_name="foo"} 0.2
`
	clusterCPUAllocatableGauge.Reset()
	RecordClusterStatus(testCluster)
	if err := testutil.CollectAndCompare(clusterCPUAllocatableGauge, strings.NewReader(want), clusterCPUAllocatableMetricsName); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestClusterPodAllocatableMetrics(t *testing.T) {
	testCluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: clusterv1alpha1.ClusterSpec{
			SyncMode: clusterv1alpha1.Push,
		},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1alpha1.ClusterConditionReady,
					Status: metav1.ConditionTrue,
				},
			},
			ResourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocatable: corev1.ResourceList{
					corev1.ResourcePods: *resource.NewQuantity(110, resource.DecimalSI),
				},
			},
		},
	}
	want := `
# HELP cluster_pod_allocatable_number Number of allocatable pods in the cluster.
# TYPE cluster_pod_allocatable_number gauge
cluster_pod_allocatable_number{cluster_name="foo"} 110
`
	clusterPodAllocatableGauge.Reset()
	RecordClusterStatus(testCluster)
	if err := testutil.CollectAndCompare(clusterPodAllocatableGauge, strings.NewReader(want), clusterPodAllocatableMetricsName); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestClusterMemoryAllocatedMetrics(t *testing.T) {
	testCluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: clusterv1alpha1.ClusterSpec{
			SyncMode: clusterv1alpha1.Push,
		},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1alpha1.ClusterConditionReady,
					Status: metav1.ConditionTrue,
				},
			},
			ResourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocated: corev1.ResourceList{
					corev1.ResourceMemory: *resource.NewQuantity(200, resource.DecimalSI),
				},
			},
		},
	}
	want := `
# HELP cluster_memory_allocated_bytes Allocated cluster memory resource in bytes.
# TYPE cluster_memory_allocated_bytes gauge
cluster_memory_allocated_bytes{cluster_name="foo"} 200
`
	clusterMemoryAllocatedGauge.Reset()
	RecordClusterStatus(testCluster)
	if err := testutil.CollectAndCompare(clusterMemoryAllocatedGauge, strings.NewReader(want), clusterMemoryAllocatedMetricsName); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestClusterCPUAllocatedMetrics(t *testing.T) {
	testCluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: clusterv1alpha1.ClusterSpec{
			SyncMode: clusterv1alpha1.Push,
		},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1alpha1.ClusterConditionReady,
					Status: metav1.ConditionTrue,
				},
			},
			ResourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocated: corev1.ResourceList{
					corev1.ResourceCPU: *resource.NewMilliQuantity(200, resource.DecimalSI),
				},
			},
		},
	}
	want := `
# HELP cluster_cpu_allocated_number Number of allocated CPU in the cluster.
# TYPE cluster_cpu_allocated_number gauge
cluster_cpu_allocated_number{cluster_name="foo"} 0.2
`
	clusterCPUAllocatedGauge.Reset()
	RecordClusterStatus(testCluster)
	if err := testutil.CollectAndCompare(clusterCPUAllocatedGauge, strings.NewReader(want), clusterCPUAllocatedMetricsName); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestClusterPodAllocatedMetrics(t *testing.T) {
	testCluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: clusterv1alpha1.ClusterSpec{
			SyncMode: clusterv1alpha1.Push,
		},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1alpha1.ClusterConditionReady,
					Status: metav1.ConditionTrue,
				},
			},
			ResourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocated: corev1.ResourceList{
					corev1.ResourcePods: *resource.NewQuantity(110, resource.DecimalSI),
				},
			},
		},
	}
	want := `
# HELP cluster_pod_allocated_number Number of allocated pods in the cluster.
# TYPE cluster_pod_allocated_number gauge
cluster_pod_allocated_number{cluster_name="foo"} 110
`
	clusterPodAllocatedGauge.Reset()
	RecordClusterStatus(testCluster)
	if err := testutil.CollectAndCompare(clusterPodAllocatedGauge, strings.NewReader(want), clusterPodAllocatedMetricsName); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}
