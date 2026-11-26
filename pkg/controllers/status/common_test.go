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

package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestCanPreserveSingleMemberPodStatus(t *testing.T) {
	raw := &runtime.RawExtension{Raw: []byte(`{"phase":"Running"}`)}
	single := workv1alpha2.ResourceBindingSpec{Clusters: []workv1alpha2.TargetCluster{{Name: "member-a"}}}
	dual := workv1alpha2.ResourceBindingSpec{Clusters: []workv1alpha2.TargetCluster{{Name: "member-a"}, {Name: "member-b"}}}
	cases := []struct {
		name     string
		spec     workv1alpha2.ResourceBindingSpec
		statuses []workv1alpha2.AggregatedStatusItem
		want     bool
	}{
		{name: "single applied target with status", spec: single, statuses: []workv1alpha2.AggregatedStatusItem{{ClusterName: "member-a", Applied: true, Status: raw}}, want: true},
		{name: "no scheduled targets", statuses: []workv1alpha2.AggregatedStatusItem{{ClusterName: "member-a", Applied: true, Status: raw}}},
		{name: "two targets but only one Work exists", spec: dual, statuses: []workv1alpha2.AggregatedStatusItem{{ClusterName: "member-a", Applied: true, Status: raw}}},
		{name: "two targets with both Works", spec: dual, statuses: []workv1alpha2.AggregatedStatusItem{{ClusterName: "member-a", Applied: true, Status: raw}, {ClusterName: "member-b", Applied: true, Status: raw}}},
		{name: "Work not applied", spec: single, statuses: []workv1alpha2.AggregatedStatusItem{{ClusterName: "member-a", Status: raw}}},
		{name: "no reflected status", spec: single, statuses: []workv1alpha2.AggregatedStatusItem{{ClusterName: "member-a", Applied: true}}},
		{name: "wrong source cluster", spec: single, statuses: []workv1alpha2.AggregatedStatusItem{{ClusterName: "member-b", Applied: true, Status: raw}}},
		{name: "duplicate Work statuses", spec: single, statuses: []workv1alpha2.AggregatedStatusItem{{ClusterName: "member-a", Applied: true, Status: raw}, {ClusterName: "member-a", Applied: true, Status: raw}}},
		{name: "dependent binding targets another cluster", spec: workv1alpha2.ResourceBindingSpec{
			Clusters:   []workv1alpha2.TargetCluster{{Name: "member-a"}},
			RequiredBy: []workv1alpha2.BindingSnapshot{{Clusters: []workv1alpha2.TargetCluster{{Name: "member-b"}}}},
		}, statuses: []workv1alpha2.AggregatedStatusItem{{ClusterName: "member-a", Applied: true, Status: raw}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := workv1alpha2.ResourceBindingStatus{AggregatedStatus: tc.statuses}
			assert.Equal(t, tc.want, canPreserveSingleMemberPodStatus(tc.spec, status))
		})
	}
}

func TestShouldEnqueueBindingUpdateOnPlacementChange(t *testing.T) {
	now := metav1.Now()
	testCases := []struct {
		name       string
		mutate     func(*workv1alpha2.ResourceBindingSpec)
		unschedule bool
		want       bool
	}{
		{name: "no effective change"},
		{name: "second cluster planned", mutate: func(s *workv1alpha2.ResourceBindingSpec) {
			s.Clusters = append(s.Clusters, workv1alpha2.TargetCluster{Name: "member-b"})
		}, want: true},
		{name: "cluster changed", mutate: func(s *workv1alpha2.ResourceBindingSpec) {
			s.Clusters = []workv1alpha2.TargetCluster{{Name: "member-b"}}
		}, want: true},
		{name: "dependent cluster planned", mutate: func(s *workv1alpha2.ResourceBindingSpec) {
			s.RequiredBy = []workv1alpha2.BindingSnapshot{{Clusters: []workv1alpha2.TargetCluster{{Name: "member-b"}}}}
		}, want: true},
		{name: "graceful eviction target", mutate: func(s *workv1alpha2.ResourceBindingSpec) {
			s.GracefulEvictionTasks = []workv1alpha2.GracefulEvictionTask{{FromCluster: "member-b"}}
		}, want: true},
		{name: "duplicate target does not change effective placement", mutate: func(s *workv1alpha2.ResourceBindingSpec) {
			s.RequiredBy = []workv1alpha2.BindingSnapshot{{Clusters: []workv1alpha2.TargetCluster{{Name: "member-a"}}}}
		}},
		{name: "resource version changed", mutate: func(s *workv1alpha2.ResourceBindingSpec) {
			s.Resource.ResourceVersion = "new-version"
		}, want: true},
		{name: "unscheduled change is ignored", mutate: func(s *workv1alpha2.ResourceBindingSpec) {
			s.Clusters = append(s.Clusters, workv1alpha2.TargetCluster{Name: "member-b"})
		}, unschedule: true},
	}
	for _, kind := range []string{"ResourceBinding", "ClusterResourceBinding"} {
		for _, tc := range testCases {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				spec := workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{APIVersion: "v1", Kind: "Pod", ResourceVersion: "original-version"},
					Clusters: []workv1alpha2.TargetCluster{{Name: "member-a"}},
				}
				status := workv1alpha2.ResourceBindingStatus{
					LastScheduledTime: &now,
					AggregatedStatus:  []workv1alpha2.AggregatedStatusItem{{ClusterName: "member-a", Applied: true}},
				}
				var oldObject event.UpdateEvent
				switch kind {
				case "ResourceBinding":
					old := &workv1alpha2.ResourceBinding{Spec: spec, Status: status}
					newBinding := old.DeepCopy()
					if tc.mutate != nil {
						tc.mutate(&newBinding.Spec)
					}
					if tc.unschedule {
						newBinding.Status.LastScheduledTime = nil
					}
					oldObject = event.UpdateEvent{ObjectOld: old, ObjectNew: newBinding}
				case "ClusterResourceBinding":
					old := &workv1alpha2.ClusterResourceBinding{Spec: spec, Status: status}
					newBinding := old.DeepCopy()
					if tc.mutate != nil {
						tc.mutate(&newBinding.Spec)
					}
					if tc.unschedule {
						newBinding.Status.LastScheduledTime = nil
					}
					oldObject = event.UpdateEvent{ObjectOld: old, ObjectNew: newBinding}
				}
				assert.Equal(t, tc.want, shouldEnqueueBindingUpdate(oldObject))
			})
		}
	}
}
