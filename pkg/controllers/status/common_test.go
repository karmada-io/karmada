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
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/event"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/native"
	"github.com/karmada-io/karmada/pkg/util/helper"
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

type placementAwareProbe struct {
	FakeResourceInterpreter
	called        bool
	allowPreserve bool
}

func (p *placementAwareProbe) AggregateStatusWithPlacement(object *unstructured.Unstructured, _ []workv1alpha2.AggregatedStatusItem, allow bool) (*unstructured.Unstructured, error) {
	p.called = true
	p.allowPreserve = allow
	return object, nil
}

func TestUpdateResourceStatusPassesPlacement(t *testing.T) {
	for _, tc := range []struct {
		name              string
		additionalCluster bool
		want              bool
	}{
		{name: "single member", want: true},
		{name: "incomplete multi-member", additionalCluster: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "runner", Namespace: "default"}}
			dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme, pod)
			mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
			mapper.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
			probe := &placementAwareProbe{FakeResourceInterpreter: FakeResourceInterpreter{DefaultInterpreter: native.NewDefaultInterpreter()}}
			spec := workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "runner"},
				Clusters: []workv1alpha2.TargetCluster{{Name: "member-a"}},
			}
			if tc.additionalCluster {
				spec.Clusters = append(spec.Clusters, workv1alpha2.TargetCluster{Name: "member-b"})
			}
			bindingStatus := workv1alpha2.ResourceBindingStatus{AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "member-a", Applied: true, Status: &runtime.RawExtension{Raw: []byte(`{"phase":"Running"}`)}},
			}}
			err := updateResourceStatus(context.Background(), dynamicClient, mapper, probe, record.NewFakeRecorder(2), spec, bindingStatus)
			assert.NoError(t, err)
			assert.True(t, probe.called)
			assert.Equal(t, tc.want, probe.allowPreserve)
		})
	}
}

// persistingPodStatusProbe exercises the status-controller write path. It is
// intentionally a test double: the real interpreter's status-copying behavior
// is verified separately in pkg/resourceinterpreter.
type persistingPodStatusProbe struct {
	FakeResourceInterpreter
	called   bool
	preserve bool
}

func (p *persistingPodStatusProbe) AggregateStatusWithPlacement(
	object *unstructured.Unstructured,
	items []workv1alpha2.AggregatedStatusItem,
	preserve bool,
) (*unstructured.Unstructured, error) {
	p.called = true
	p.preserve = preserve
	if !preserve {
		return p.FakeResourceInterpreter.AggregateStatus(object, items)
	}

	pod := &corev1.Pod{}
	if err := helper.ConvertToTypedObject(object, pod); err != nil {
		return nil, err
	}
	var status corev1.PodStatus
	if err := json.Unmarshal(items[0].Status.Raw, &status); err != nil {
		return nil, err
	}
	pod.Status = status
	return helper.ToUnstructured(pod)
}

func TestUpdateResourceStatusPersistsPodStatus(t *testing.T) {
	for _, tc := range []struct {
		name        string
		multiTarget bool
	}{
		{name: "one planned member"},
		{name: "two planned members, only one status", multiTarget: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{
					Name: "runner-pod", Namespace: "default",
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{
					{Name: "runner"}, {Name: "sidecar"},
				}},
			}
			client := dynamicfake.NewSimpleDynamicClient(scheme.Scheme, pod)
			mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
			mapper.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)

			originalStatus := corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "10.244.1.7",
				Conditions: []corev1.PodCondition{{
					Type: corev1.PodReady, Status: corev1.ConditionFalse,
				}},
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "runner", Ready: false, Image: "runner:v1", RestartCount: 2,
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
						},
					},
					{
						Name: "sidecar", Ready: true,
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{},
						},
					},
				},
			}
			raw, err := helper.BuildStatusRawExtension(&originalStatus)
			if !assert.NoError(t, err) {
				return
			}
			spec := workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{
					APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "runner-pod",
				},
				Clusters: []workv1alpha2.TargetCluster{{Name: "member-a"}},
			}
			if tc.multiTarget {
				spec.Clusters = append(spec.Clusters, workv1alpha2.TargetCluster{Name: "member-b"})
			}
			bindingStatus := workv1alpha2.ResourceBindingStatus{
				AggregatedStatus: []workv1alpha2.AggregatedStatusItem{{
					ClusterName: "member-a", Applied: true, Status: raw,
				}},
			}
			interpreter := &persistingPodStatusProbe{
				FakeResourceInterpreter: FakeResourceInterpreter{
					DefaultInterpreter: native.NewDefaultInterpreter(),
				},
			}
			err = updateResourceStatus(
				context.Background(), client, mapper, interpreter,
				record.NewFakeRecorder(2), spec, bindingStatus,
			)
			if !assert.NoError(t, err) {
				return
			}
			assert.True(t, interpreter.called)
			assert.Equal(t, !tc.multiTarget, interpreter.preserve)
			updatedStatus := false
			for _, action := range client.Actions() {
				if action.GetVerb() == "update" && action.GetResource().Resource == "pods" && action.GetSubresource() == "status" {
					updatedStatus = true
				}
			}
			assert.True(t, updatedStatus, "status-controller must issue UpdateStatus")

			gvr := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
			actual, err := client.Resource(gvr).Namespace("default").Get(
				context.Background(), "runner-pod", metav1.GetOptions{},
			)
			if !assert.NoError(t, err) {
				return
			}
			var actualPod corev1.Pod
			if !assert.NoError(t, helper.ConvertToTypedObject(actual, &actualPod)) {
				return
			}
			assert.Equal(t, pod.Spec, actualPod.Spec)
			assert.Equal(t, corev1.PodRunning, actualPod.Status.Phase)
			if !tc.multiTarget {
				assert.Equal(t, originalStatus, actualPod.Status)
			} else {
				// Existing multi-member semantics remain unchanged.
				assert.Empty(t, actualPod.Status.Conditions)
				assert.Empty(t, actualPod.Status.PodIP)
				if assert.Len(t, actualPod.Status.ContainerStatuses, 2) {
					assert.Empty(t, actualPod.Status.ContainerStatuses[0].Name)
					assert.Empty(t, actualPod.Status.ContainerStatuses[1].Name)
				}
			}
		})
	}
}
