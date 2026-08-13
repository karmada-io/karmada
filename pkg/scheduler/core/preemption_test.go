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

package core

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/features"
	clusterv1alpha1lister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	schedulercache "github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
	"github.com/karmada-io/karmada/test/helper"
)

func TestListPreemptionVictimCandidates(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, binding := range []*workv1alpha2.ResourceBinding{
		newPreemptionTestBinding("default", "lower-on-target", 50, []workv1alpha2.TargetCluster{{Name: "member1", Replicas: 3}}, time.Unix(1, 0)),
		newPreemptionTestBinding("default", "lower-on-other", 50, []workv1alpha2.TargetCluster{{Name: "member2", Replicas: 3}}, time.Unix(2, 0)),
		newPreemptionTestBinding("default", "equal-priority", 100, []workv1alpha2.TargetCluster{{Name: "member1", Replicas: 3}}, time.Unix(3, 0)),
		newPreemptionTestBinding("default", "higher-priority", 200, []workv1alpha2.TargetCluster{{Name: "member1", Replicas: 3}}, time.Unix(4, 0)),
		newPreemptionTestBinding("default", "zero-replicas", 50, []workv1alpha2.TargetCluster{{Name: "member1"}}, time.Unix(5, 0)),
		func() *workv1alpha2.ResourceBinding {
			binding := newPreemptionTestBinding("default", "already-evicting", 50, []workv1alpha2.TargetCluster{{Name: "member1", Replicas: 3}}, time.Unix(6, 0))
			binding.Spec.GracefulEvictionTasks = []workv1alpha2.GracefulEvictionTask{{FromCluster: "member1"}}
			return binding
		}(),
	} {
		if err := indexer.Add(binding); err != nil {
			t.Fatalf("failed to add binding %s/%s: %v", binding.Namespace, binding.Name, err)
		}
	}

	got, err := listPreemptionVictimCandidates(indexer, newPreemptionTestSpec(10, 100), "member1")
	if err != nil {
		t.Fatalf("listPreemptionVictimCandidates() error = %v", err)
	}
	want := []preemptionVictimCandidate{
		{
			namespace:         "default",
			name:              "lower-on-target",
			replicas:          3,
			priority:          50,
			creationTimestamp: time.Unix(1, 0).UnixNano(),
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listPreemptionVictimCandidates() = %+v, want %+v", got, want)
	}
}

func TestListPreemptionVictimCandidatesRejectsInvalidInputs(t *testing.T) {
	if _, err := listPreemptionVictimCandidates(nil, newPreemptionTestSpec(10, 100), "member1"); err == nil {
		t.Fatal("expected nil indexer to fail")
	}

	indexer := cache.NewIndexer(func(any) (string, error) { return "invalid", nil }, cache.Indexers{})
	if err := indexer.Add("not-a-binding"); err != nil {
		t.Fatalf("failed to add object: %v", err)
	}
	if _, err := listPreemptionVictimCandidates(indexer, newPreemptionTestSpec(10, 100), "member1"); err == nil {
		t.Fatal("expected invalid cached object to fail")
	}
}

func TestSelectVictimsByReplicaCount(t *testing.T) {
	tests := []struct {
		name             string
		preemptor        *workv1alpha2.ResourceBindingSpec
		candidates       []preemptionVictimCandidate
		clusterAvailable int32
		want             []VictimBinding
		wantErr          bool
	}{
		{
			name:             "no deficit returns no victims",
			preemptor:        newPreemptionTestSpec(5, 100),
			clusterAvailable: 5,
		},
		{
			name:      "reprieves higher-priority candidates first",
			preemptor: newPreemptionTestSpec(10, 100),
			candidates: []preemptionVictimCandidate{
				{namespace: "default", name: "highest-lower", replicas: 4, priority: 90, creationTimestamp: time.Unix(1, 0).UnixNano()},
				{namespace: "default", name: "middle", replicas: 5, priority: 70, creationTimestamp: time.Unix(2, 0).UnixNano()},
				{namespace: "default", name: "lowest", replicas: 5, priority: 60, creationTimestamp: time.Unix(3, 0).UnixNano()},
			},
			clusterAvailable: 2,
			want: []VictimBinding{
				{Namespace: "default", Name: "middle", Replicas: 5, Priority: 70},
				{Namespace: "default", Name: "lowest", Replicas: 5, Priority: 60},
			},
		},
		{
			name:      "uses creation timestamp as reprieve tiebreaker",
			preemptor: newPreemptionTestSpec(10, 100),
			candidates: []preemptionVictimCandidate{
				{namespace: "default", name: "younger", replicas: 5, priority: 50, creationTimestamp: time.Unix(2, 0).UnixNano()},
				{namespace: "default", name: "older", replicas: 5, priority: 50, creationTimestamp: time.Unix(1, 0).UnixNano()},
			},
			clusterAvailable: 5,
			want: []VictimBinding{
				{Namespace: "default", Name: "younger", Replicas: 5, Priority: 50},
			},
		},
		{
			name:      "candidates at or above preemptor priority are ignored",
			preemptor: newPreemptionTestSpec(10, 100),
			candidates: []preemptionVictimCandidate{
				{namespace: "default", name: "equal", replicas: 10, priority: 100, creationTimestamp: time.Unix(1, 0).UnixNano()},
				{namespace: "default", name: "higher", replicas: 10, priority: 200, creationTimestamp: time.Unix(2, 0).UnixNano()},
				{namespace: "default", name: "lower", replicas: 6, priority: 50, creationTimestamp: time.Unix(3, 0).UnixNano()},
			},
			clusterAvailable: 4,
			want: []VictimBinding{
				{Namespace: "default", Name: "lower", Replicas: 6, Priority: 50},
			},
		},
		{
			name:      "infeasible returns error",
			preemptor: newPreemptionTestSpec(10, 100),
			candidates: []preemptionVictimCandidate{
				{namespace: "default", name: "small", replicas: 3, priority: 50, creationTimestamp: time.Unix(1, 0).UnixNano()},
			},
			clusterAvailable: 2,
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectVictimsByReplicaCount(tt.preemptor, tt.candidates, tt.clusterAvailable)
			if (err != nil) != tt.wantErr {
				t.Fatalf("selectVictimsByReplicaCount() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("selectVictimsByReplicaCount() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestIsPreemptionApplicable(t *testing.T) {
	enablePreemptionFeatureGates(t)

	tests := []struct {
		name   string
		mutate func(*workv1alpha2.ResourceBindingSpec)
		want   bool
	}{
		{
			name: "applicable",
			want: true,
		},
		{
			name: "non workload",
			mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
				spec.Replicas = 0
				spec.ReplicaRequirements = nil
			},
		},
		{
			name: "multiple components",
			mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
				spec.Components = []workv1alpha2.Component{{Name: "main"}, {Name: "sidecar"}}
			},
		},
		{
			name: "cluster affinities path",
			mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
				spec.Placement.ClusterAffinity = nil
				spec.Placement.ClusterAffinities = []policyv1alpha1.ClusterAffinityTerm{{AffinityName: "primary"}}
			},
		},
		{
			name: "missing cluster affinity",
			mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
				spec.Placement.ClusterAffinity = nil
			},
		},
		{
			name: "multi cluster spread",
			mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
				spec.Placement.SpreadConstraints[0].MaxGroups = 2
			},
		},
		{
			name: "duplicated scheduling",
			mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
				spec.Placement.ReplicaScheduling.ReplicaSchedulingType = policyv1alpha1.ReplicaSchedulingTypeDuplicated
			},
		},
		{
			name: "static weight scheduling",
			mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
				spec.Placement.ReplicaScheduling.ReplicaDivisionPreference = policyv1alpha1.ReplicaDivisionPreferenceWeighted
				spec.Placement.ReplicaScheduling.WeightPreference = &policyv1alpha1.ClusterPreferences{
					StaticWeightList: []policyv1alpha1.StaticClusterWeight{
						{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}}, Weight: 1},
					},
				}
			},
		},
		{
			name: "missing schedule priority",
			mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
				spec.SchedulePriority = nil
			},
		},
		{
			name: "preemption policy unset",
			mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
				spec.SchedulePriority.PreemptionPolicy = ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := newApplicablePreemptionTestSpec("default", "preemptor", 10, 100)
			if tt.mutate != nil {
				tt.mutate(spec)
			}
			if got := isPreemptionApplicable(spec); got != tt.want {
				t.Fatalf("isPreemptionApplicable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPreemptionApplicableFeatureGateDisabled(t *testing.T) {
	disablePreemptionFeatureGates(t)

	spec := newApplicablePreemptionTestSpec("default", "preemptor", 10, 100)
	if got := isPreemptionApplicable(spec); got {
		t.Fatalf("isPreemptionApplicable() = %v, want false", got)
	}
}

func TestScheduleReturnsPreemptionResult(t *testing.T) {
	enablePreemptionFeatureGates(t)
	withPreemptionTestEstimator(t, []workv1alpha2.TargetCluster{{Name: "member1", Replicas: 2}})

	scheduler := newPreemptionTestScheduler(t,
		[]string{"member1"},
		newPreemptionTestBinding("default", "lower-priority", 50, []workv1alpha2.TargetCluster{{Name: "member1", Replicas: 5}}, time.Unix(1, 0)),
	)

	got, err := scheduler.Schedule(context.Background(), newApplicablePreemptionTestSpec("default", "preemptor", 5, 100), &workv1alpha2.ResourceBindingStatus{}, &ScheduleAlgorithmOption{})
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	want := &PreemptionResult{
		Cluster: "member1",
		Victims: []VictimBinding{
			{Namespace: "default", Name: "lower-priority", Replicas: 5, Priority: 50},
		},
	}
	if !reflect.DeepEqual(got.PreemptionResult, want) {
		t.Fatalf("Schedule().PreemptionResult = %+v, want %+v", got.PreemptionResult, want)
	}
	if got.SuggestedClusters != nil {
		t.Fatalf("Schedule().SuggestedClusters = %v, want nil", got.SuggestedClusters)
	}

	claim, ok := scheduler.preemptionClaims.Get("default/preemptor")
	if !ok {
		t.Fatal("expected preemption claim to be recorded")
	}
	if claim.cluster != "member1" || claim.priority != 100 || claim.replicas != 5 {
		t.Fatalf("preemption claim = %+v, want cluster member1 priority 100 replicas 5", claim)
	}
}

func TestScheduleSuppressesPreemptionWhenClusterHasActiveClaim(t *testing.T) {
	enablePreemptionFeatureGates(t)
	withPreemptionTestEstimator(t, []workv1alpha2.TargetCluster{{Name: "member1", Replicas: 2}})

	scheduler := newPreemptionTestScheduler(t,
		[]string{"member1"},
		newPreemptionTestBinding("default", "lower-priority", 50, []workv1alpha2.TargetCluster{{Name: "member1", Replicas: 5}}, time.Unix(1, 0)),
	)
	scheduler.preemptionClaims.Set(preemptionClaim{
		bindingKey: "default/other-preemptor",
		cluster:    "member1",
		priority:   100,
		replicas:   5,
	})

	got, err := scheduler.Schedule(context.Background(), newApplicablePreemptionTestSpec("default", "preemptor", 5, 100), &workv1alpha2.ResourceBindingStatus{}, &ScheduleAlgorithmOption{})
	if err == nil {
		t.Fatal("expected scheduling to fail when active claim suppresses preemption")
	}
	if got.PreemptionResult != nil {
		t.Fatalf("Schedule().PreemptionResult = %+v, want nil", got.PreemptionResult)
	}
}

func newPreemptionTestSpec(replicas, priority int32) *workv1alpha2.ResourceBindingSpec {
	return &workv1alpha2.ResourceBindingSpec{
		Replicas: replicas,
		SchedulePriority: &workv1alpha2.SchedulePriority{
			Priority: priority,
		},
	}
}

func newApplicablePreemptionTestSpec(namespace, name string, replicas, priority int32) *workv1alpha2.ResourceBindingSpec {
	return &workv1alpha2.ResourceBindingSpec{
		Resource: workv1alpha2.ObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  namespace,
			Name:       name,
		},
		Replicas: replicas,
		Placement: &policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{"member1"},
			},
			SpreadConstraints: []policyv1alpha1.SpreadConstraint{
				{
					SpreadByField: policyv1alpha1.SpreadByFieldCluster,
					MinGroups:     1,
					MaxGroups:     1,
				},
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
		},
		SchedulePriority: &workv1alpha2.SchedulePriority{
			Priority:         priority,
			PreemptionPolicy: workv1alpha2.PreemptLowerPriority,
		},
	}
}

func newPreemptionTestBinding(namespace, name string, priority int32, clusters []workv1alpha2.TargetCluster, createdAt time.Time) *workv1alpha2.ResourceBinding {
	return &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         namespace,
			Name:              name,
			CreationTimestamp: metav1.NewTime(createdAt),
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: clusters,
			SchedulePriority: &workv1alpha2.SchedulePriority{
				Priority: priority,
			},
		},
	}
}

func enablePreemptionFeatureGates(t *testing.T) {
	t.Helper()

	setPreemptionFeatureGates(t, true, true)
}

func disablePreemptionFeatureGates(t *testing.T) {
	t.Helper()

	setPreemptionFeatureGates(t, false, false)
}

func setPreemptionFeatureGates(t *testing.T, priorityBasedScheduling, priorityBasedPreemptiveScheduling bool) {
	t.Helper()

	originalFeatureGate := features.FeatureGate.DeepCopy()
	t.Cleanup(func() {
		features.FeatureGate = originalFeatureGate
	})

	if err := features.FeatureGate.Set(fmt.Sprintf("%s=%t,%s=%t",
		features.PriorityBasedScheduling,
		priorityBasedScheduling,
		features.PriorityBasedPreemptiveScheduling,
		priorityBasedPreemptiveScheduling,
	)); err != nil {
		t.Fatalf("failed to set feature gates: %v", err)
	}
}

func newPreemptionTestScheduler(t *testing.T, clusterNames []string, bindings ...*workv1alpha2.ResourceBinding) *genericScheduler {
	t.Helper()

	clusterIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, clusterName := range clusterNames {
		if err := clusterIndexer.Add(helper.NewCluster(clusterName)); err != nil {
			t.Fatalf("failed to add cluster %s: %v", clusterName, err)
		}
	}

	bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, binding := range bindings {
		if err := bindingIndexer.Add(binding); err != nil {
			t.Fatalf("failed to add binding %s/%s: %v", binding.Namespace, binding.Name, err)
		}
	}

	algorithm, err := NewGenericScheduler(schedulercache.NewCache(clusterv1alpha1lister.NewClusterLister(clusterIndexer), bindingIndexer, 0), runtime.Registry{})
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}
	scheduler, ok := algorithm.(*genericScheduler)
	if !ok {
		t.Fatalf("scheduler type = %T, want *genericScheduler", algorithm)
	}
	return scheduler
}

func withPreemptionTestEstimator(t *testing.T, available []workv1alpha2.TargetCluster) {
	t.Helper()

	estimators := estimatorclient.GetReplicaEstimators()
	previous := make(map[string]estimatorclient.ReplicaEstimator, len(estimators))
	for name, estimator := range estimators {
		previous[name] = estimator
		delete(estimators, name)
	}
	estimators["preemption-test"] = &preemptionTestReplicaEstimator{available: available}

	t.Cleanup(func() {
		for name := range estimators {
			delete(estimators, name)
		}
		for name, estimator := range previous {
			estimators[name] = estimator
		}
	})
}

type preemptionTestReplicaEstimator struct {
	available []workv1alpha2.TargetCluster
}

func (e *preemptionTestReplicaEstimator) MaxAvailableReplicas(context.Context, estimatorclient.ReplicaEstimationRequest) ([]workv1alpha2.TargetCluster, error) {
	return e.available, nil
}

func (e *preemptionTestReplicaEstimator) MaxAvailableComponentSets(context.Context, estimatorclient.ComponentSetEstimationRequest) ([]estimatorclient.ComponentSetEstimationResponse, error) {
	return nil, nil
}
