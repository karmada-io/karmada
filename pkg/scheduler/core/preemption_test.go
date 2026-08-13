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
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
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

func newPreemptionTestSpec(replicas, priority int32) *workv1alpha2.ResourceBindingSpec {
	return &workv1alpha2.ResourceBindingSpec{
		Replicas: replicas,
		SchedulePriority: &workv1alpha2.SchedulePriority{
			Priority: priority,
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
