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

package core

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/features"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	schedulercache "github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/core/spreadconstraint"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/clusteraffinity"
	schedulerruntime "github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
	"github.com/karmada-io/karmada/test/helper"
)

type testcase struct {
	name     string
	clusters []spreadconstraint.ClusterDetailInfo
	object   workv1alpha2.ResourceBindingSpec
	result   []workv1alpha2.TargetCluster
}

func TestRetainScheduledClusters(t *testing.T) {
	cluster1 := helper.NewCluster(ClusterMember1)
	cluster2 := helper.NewCluster(ClusterMember2)
	got := retainScheduledClusters([]*clusterv1alpha1.Cluster{cluster1, cluster2}, []workv1alpha2.TargetCluster{{Name: ClusterMember2}})
	if !reflect.DeepEqual(got, []*clusterv1alpha1.Cluster{cluster2}) {
		t.Fatalf("retainScheduledClusters() = %v, want only %s", got, ClusterMember2)
	}
}

func TestComponentScaleDoesNotMigrateWhenCurrentTargetIsFilterIneligible(t *testing.T) {
	const eligibleLabel = "scale-eligible"
	currentTarget := helper.NewCluster(ClusterMember1)
	currentTarget.Labels = map[string]string{eligibleLabel: "false"}
	alternative := helper.NewCluster(ClusterMember2)
	alternative.Labels = map[string]string{eligibleLabel: "true"}

	indexer := toolscache.NewIndexer(toolscache.MetaNamespaceKeyFunc, toolscache.Indexers{})
	for _, cluster := range []*clusterv1alpha1.Cluster{currentTarget, alternative} {
		if err := indexer.Add(cluster); err != nil {
			t.Fatalf("failed to add cluster %q to indexer: %v", cluster.Name, err)
		}
	}

	registry := schedulerruntime.Registry{clusteraffinity.Name: clusteraffinity.New}
	algorithm, err := NewGenericScheduler(
		schedulercache.NewCache(clusterlister.NewClusterLister(indexer), nil, 0),
		registry,
	)
	if err != nil {
		t.Fatalf("NewGenericScheduler() error = %v", err)
	}

	spec := &workv1alpha2.ResourceBindingSpec{
		Placement: &policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{eligibleLabel: "true"}},
			},
			SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
				SpreadByField: policyv1alpha1.SpreadByFieldCluster,
				MinGroups:     1,
				MaxGroups:     1,
			}},
		},
		Components: []workv1alpha2.Component{
			{Name: "jobmanager", Replicas: 1},
			{Name: "taskmanager", Replicas: 6},
		},
		Clusters: []workv1alpha2.TargetCluster{{
			Name: ClusterMember1,
			Components: []workv1alpha2.TargetComponent{
				{Name: "jobmanager", Replicas: 1},
				{Name: "taskmanager", Replicas: 4},
			},
		}},
	}

	result, err := algorithm.Schedule(context.Background(), spec, &workv1alpha2.ResourceBindingStatus{}, &ScheduleAlgorithmOption{
		IsMultiComponentScale:        true,
		ReuseAcceptedComponentTarget: true,
	})
	var fitErr *framework.FitError
	if !errors.As(err, &fitErr) {
		t.Fatalf("Schedule() error = %v, want *framework.FitError", err)
	}
	if len(result.SuggestedClusters) != 0 {
		t.Fatalf("Schedule() suggested clusters = %v, want no migration target", result.SuggestedClusters)
	}
	if fitErr.NumAllClusters != 2 {
		t.Fatalf("FitError.NumAllClusters = %d, want 2", fitErr.NumAllClusters)
	}
	if _, exists := fitErr.Diagnosis.ClusterToResultMap[ClusterMember1]; !exists {
		t.Fatalf("FitError diagnosis does not contain rejected current target %q", ClusterMember1)
	}
	if _, exists := fitErr.Diagnosis.ClusterToResultMap[ClusterMember2]; exists {
		t.Fatalf("FitError diagnosis unexpectedly rejects feasible alternative %q", ClusterMember2)
	}
}

func TestComponentScaleDownDoesNotLeakAvailabilitySentinelIntoScheduleResult(t *testing.T) {
	originalFeatureGates := features.FeatureGate.DeepCopy()
	if err := features.FeatureGate.Set(fmt.Sprintf("%s=true", features.MultiplePodTemplatesScheduling)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { features.FeatureGate = originalFeatureGates })

	originalEstimators := make(map[string]estimatorclient.ReplicaEstimator, len(estimatorclient.GetReplicaEstimators()))
	maps.Copy(originalEstimators, estimatorclient.GetReplicaEstimators())
	for name := range estimatorclient.GetReplicaEstimators() {
		delete(estimatorclient.GetReplicaEstimators(), name)
	}
	t.Cleanup(func() {
		for name := range estimatorclient.GetReplicaEstimators() {
			delete(estimatorclient.GetReplicaEstimators(), name)
		}
		maps.Copy(estimatorclient.GetReplicaEstimators(), originalEstimators)
	})

	indexer := toolscache.NewIndexer(toolscache.MetaNamespaceKeyFunc, toolscache.Indexers{})
	for _, cluster := range []*clusterv1alpha1.Cluster{helper.NewCluster(ClusterMember1), helper.NewCluster(ClusterMember2)} {
		if err := indexer.Add(cluster); err != nil {
			t.Fatalf("failed to add cluster %q to indexer: %v", cluster.Name, err)
		}
	}
	algorithm, err := NewGenericScheduler(
		schedulercache.NewCache(clusterlister.NewClusterLister(indexer), nil, 0),
		schedulerruntime.Registry{},
	)
	if err != nil {
		t.Fatalf("NewGenericScheduler() error = %v", err)
	}

	desired := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 2}}
	spec := &workv1alpha2.ResourceBindingSpec{
		Resource: workv1alpha2.ObjectReference{Namespace: "default", Name: "flink"},
		Placement: &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
			SpreadByField: policyv1alpha1.SpreadByFieldCluster,
			MinGroups:     1,
			MaxGroups:     1,
		}}},
		Components: desired,
		Clusters: []workv1alpha2.TargetCluster{{Name: ClusterMember1, Components: []workv1alpha2.TargetComponent{
			{Name: "jobmanager", Replicas: 1},
			{Name: "taskmanager", Replicas: 4},
		}}},
	}

	result, err := algorithm.Schedule(context.Background(), spec, &workv1alpha2.ResourceBindingStatus{}, &ScheduleAlgorithmOption{IsMultiComponentScale: true})
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	want := []workv1alpha2.TargetCluster{{Name: ClusterMember1, Components: []workv1alpha2.TargetComponent{
		{Name: "jobmanager", Replicas: 1},
		{Name: "taskmanager", Replicas: 2},
	}}}
	if !reflect.DeepEqual(result.SuggestedClusters, want) {
		t.Fatalf("Schedule() suggested clusters = %#v, want %#v", result.SuggestedClusters, want)
	}
	if result.SuggestedClusters[0].Replicas != 0 {
		t.Fatalf("Schedule() persisted scalar replicas = %d, want 0", result.SuggestedClusters[0].Replicas)
	}
	if len(estimatorclient.GetReplicaEstimators()) != 0 {
		t.Fatalf("replica estimator registry size = %d, want 0", len(estimatorclient.GetReplicaEstimators()))
	}
}

func TestAcceptedComponentTargetIsReusedOnlyWhileFilterEligible(t *testing.T) {
	estimator := setupAcceptedComponentTargetTest(t)

	t.Run("keeps accepted target without estimating its full desired footprint", func(t *testing.T) {
		testAcceptedComponentTargetReuse(t, estimator)
	})
	t.Run("estimates new candidates when accepted target no longer passes filters", func(t *testing.T) {
		testAcceptedComponentTargetFilterFallback(t, estimator)
	})
	t.Run("falls back when accepted target no longer satisfies spread constraints", func(t *testing.T) {
		testAcceptedComponentTargetSpreadFallback(t, estimator)
	})
}

func setupAcceptedComponentTargetTest(t *testing.T) *mockReplicaEstimator {
	t.Helper()
	originalFeatureGates := features.FeatureGate.DeepCopy()
	if err := features.FeatureGate.Set(fmt.Sprintf("%s=true", features.MultiplePodTemplatesScheduling)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { features.FeatureGate = originalFeatureGates })

	originalEstimators := make(map[string]estimatorclient.ReplicaEstimator, len(estimatorclient.GetReplicaEstimators()))
	maps.Copy(originalEstimators, estimatorclient.GetReplicaEstimators())
	for name := range estimatorclient.GetReplicaEstimators() {
		delete(estimatorclient.GetReplicaEstimators(), name)
	}
	t.Cleanup(func() {
		for name := range estimatorclient.GetReplicaEstimators() {
			delete(estimatorclient.GetReplicaEstimators(), name)
		}
		maps.Copy(estimatorclient.GetReplicaEstimators(), originalEstimators)
	})

	estimator := &mockReplicaEstimator{}
	estimator.maxAvailableComponentSetsFunc = func(req estimatorclient.ComponentSetEstimationRequest) ([]estimatorclient.ComponentSetEstimationResponse, error) {
		result := make([]estimatorclient.ComponentSetEstimationResponse, len(req.Clusters))
		for i := range req.Clusters {
			sets := int32(0)
			if req.Clusters[i].Name == ClusterMember2 {
				sets = 1
			}
			result[i] = estimatorclient.ComponentSetEstimationResponse{Name: req.Clusters[i].Name, Sets: sets}
		}
		return result, nil
	}
	estimatorclient.GetReplicaEstimators()["test"] = estimator
	return estimator
}

func acceptedComponentTargetSpec() *workv1alpha2.ResourceBindingSpec {
	components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	return &workv1alpha2.ResourceBindingSpec{
		Resource: workv1alpha2.ObjectReference{Namespace: "default", Name: "flink"},
		Placement: &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
			SpreadByField: policyv1alpha1.SpreadByFieldCluster,
			MinGroups:     1,
			MaxGroups:     1,
		}}},
		Components: components,
		Clusters: []workv1alpha2.TargetCluster{{Name: ClusterMember1, Components: []workv1alpha2.TargetComponent{
			{Name: "jobmanager", Replicas: 1},
			{Name: "taskmanager", Replicas: 4},
		}}},
	}
}

func testAcceptedComponentTargetReuse(t *testing.T, estimator *mockReplicaEstimator) {
	t.Helper()
	estimator.componentSetRequests = nil
	algorithm := newGenericSchedulerForTest(t, []*clusterv1alpha1.Cluster{
		helper.NewCluster(ClusterMember1), helper.NewCluster(ClusterMember2),
	}, schedulerruntime.Registry{})
	spec := acceptedComponentTargetSpec()

	result, err := algorithm.Schedule(context.Background(), spec, &workv1alpha2.ResourceBindingStatus{}, &ScheduleAlgorithmOption{ReuseAcceptedComponentTarget: true})
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	if got := result.SuggestedClusters; len(got) != 1 || got[0].Name != ClusterMember1 {
		t.Fatalf("Schedule() suggested clusters = %v, want accepted target %q", got, ClusterMember1)
	}
	if !reflect.DeepEqual(result.SuggestedClusters[0].Components, spec.Clusters[0].Components) {
		t.Fatalf("Schedule() components = %v, want accepted components %v", result.SuggestedClusters[0].Components, spec.Clusters[0].Components)
	}
	if len(estimator.componentSetRequests) != 0 {
		t.Fatalf("MaxAvailableComponentSets() calls = %d, want 0", len(estimator.componentSetRequests))
	}
}

func testAcceptedComponentTargetFilterFallback(t *testing.T, estimator *mockReplicaEstimator) {
	t.Helper()
	estimator.componentSetRequests = nil
	const eligibleLabel = "steady-eligible"
	current := helper.NewCluster(ClusterMember1)
	current.Labels = map[string]string{eligibleLabel: "false"}
	alternative := helper.NewCluster(ClusterMember2)
	alternative.Labels = map[string]string{eligibleLabel: "true"}
	algorithm := newGenericSchedulerForTest(t, []*clusterv1alpha1.Cluster{current, alternative}, schedulerruntime.Registry{clusteraffinity.Name: clusteraffinity.New})
	spec := acceptedComponentTargetSpec()
	spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{eligibleLabel: "true"}}}
	assertAcceptedTargetFallback(t, algorithm, estimator, spec, "failover target")
}

func testAcceptedComponentTargetSpreadFallback(t *testing.T, estimator *mockReplicaEstimator) {
	t.Helper()
	estimator.componentSetRequests = nil
	current := helper.NewCluster(ClusterMember1)
	alternative := helper.NewCluster(ClusterMember2)
	alternative.Spec.Region = "region1"
	algorithm := newGenericSchedulerForTest(t, []*clusterv1alpha1.Cluster{current, alternative}, schedulerruntime.Registry{})
	spec := acceptedComponentTargetSpec()
	spec.Placement.SpreadConstraints = append([]policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldRegion,
		MinGroups:     1,
		MaxGroups:     1,
	}}, spec.Placement.SpreadConstraints...)
	assertAcceptedTargetFallback(t, algorithm, estimator, spec, "spread-compatible target")
}

func assertAcceptedTargetFallback(t *testing.T, algorithm ScheduleAlgorithm, estimator *mockReplicaEstimator, spec *workv1alpha2.ResourceBindingSpec, targetDescription string) {
	t.Helper()
	result, err := algorithm.Schedule(context.Background(), spec, &workv1alpha2.ResourceBindingStatus{}, &ScheduleAlgorithmOption{ReuseAcceptedComponentTarget: true})
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	if got := result.SuggestedClusters; len(got) != 1 || got[0].Name != ClusterMember2 {
		t.Fatalf("Schedule() suggested clusters = %v, want %s %q", got, targetDescription, ClusterMember2)
	}
	if len(estimator.componentSetRequests) != 1 {
		t.Fatalf("MaxAvailableComponentSets() calls = %d, want 1", len(estimator.componentSetRequests))
	}
	request := estimator.componentSetRequests[0]
	if len(request.Clusters) != 1 || request.Clusters[0].Name != ClusterMember2 {
		t.Fatalf("estimated clusters = %v, want only %q", request.Clusters, ClusterMember2)
	}
	if !reflect.DeepEqual(request.Components, spec.Components) {
		t.Fatalf("estimated components = %v, want full desired %v", request.Components, spec.Components)
	}
}

func newGenericSchedulerForTest(t *testing.T, clusters []*clusterv1alpha1.Cluster, registry schedulerruntime.Registry) ScheduleAlgorithm {
	t.Helper()
	indexer := toolscache.NewIndexer(toolscache.MetaNamespaceKeyFunc, toolscache.Indexers{})
	for _, cluster := range clusters {
		if err := indexer.Add(cluster); err != nil {
			t.Fatalf("failed to add cluster %q to indexer: %v", cluster.Name, err)
		}
	}
	algorithm, err := NewGenericScheduler(schedulercache.NewCache(clusterlister.NewClusterLister(indexer), nil, 0), registry)
	if err != nil {
		t.Fatalf("NewGenericScheduler() error = %v", err)
	}
	return algorithm
}

func Test_DistributionOfReplicas(t *testing.T) {
	tests := []testcase{
		{
			name: "replica 3, static weighted 1:1",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 3,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 3, static weighted 1:1:1, change replicas from 3 to 5, before change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 3,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 1,
				},
				{
					Name:     ClusterMember2,
					Replicas: 1,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 3, static weighted 1:1:1, change replicas from 3 to 5, after change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 5, // change replicas from 3 to 5
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 7, static weighted 2:1:1:1, change replicas from 7 to 8, before change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 7,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 7, static weighted 2:1:1:1, change replicas from 7 to 8, after change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 8, // change replicas from 7 to 8
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 2,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 9, static weighted 2:1:1:1, change replicas from 9 to 8, before change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 9,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 2,
				},
				{
					Name:     ClusterMember4,
					Replicas: 2,
				},
			},
		},
		{
			name: "replica 9, static weighted 2:1:1:1, change replicas from 9 to 8, after change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 8,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 2,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 6, static weighted 1:1:1:1, change static weighted from 1:1:1:1 to 2:1:1:1, before change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 6,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 6, static weighted 1:1:1:1, change static weighted from 1:1:1:1 to 2:1:1:1, after change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 6,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
				{
					Name:     ClusterMember2,
					Replicas: 1,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 5, static weighted 1:1:1, add a new cluster and change static weight to 1:1:1:1, before change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 5,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 5, static weighted 1:1:1, add a new cluster and change static weight to 1:1:1:1, after change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 5,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 1,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 6, static weighted 1:1:1:1, remove a cluster and change static weight to 1:1:1, before change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 6,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 6, static weighted 1:1:1:1, remove a cluster and change static weight to 1:1:1, after change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 6,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 2,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var g = &genericScheduler{}
			obj := tt.object

			// 2. schedule basing on previous schedule result
			got, err := g.assignReplicas(tt.clusters, &obj, &workv1alpha2.ResourceBindingStatus{})
			if err != nil {
				t.Errorf("AssignReplicas() error = %v", err)
				return
			}

			// 3. check if schedule result got match to expected
			if !reflect.DeepEqual(got, tt.result) {
				t.Errorf("AssignReplicas() got = %v, wants %v", got, tt.result)
			}
		})
	}
}
