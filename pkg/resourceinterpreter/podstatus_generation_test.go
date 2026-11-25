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

package resourceinterpreter

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/native"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func TestCopySingleMemberPodObservedGeneration(t *testing.T) {
	cases := []struct {
		name                  string
		memberGeneration      int64
		templateGeneration    int64
		templateAnnotation    int64
		memberObserved        int64
		conditionObserved     int64
		wantPodObserved       int64
		wantConditionObserved int64
	}{
		{name: "both observations current", memberGeneration: 1, templateGeneration: 3, templateAnnotation: 3, memberObserved: 1, conditionObserved: 1, wantPodObserved: 3, wantConditionObserved: 3},
		{name: "member status stale", memberGeneration: 2, templateGeneration: 3, templateAnnotation: 3, memberObserved: 1, conditionObserved: 1},
		{name: "condition stale independently", memberGeneration: 2, templateGeneration: 3, templateAnnotation: 3, memberObserved: 2, conditionObserved: 1, wantPodObserved: 3},
		{name: "new template has not reached member", memberGeneration: 1, templateGeneration: 4, templateAnnotation: 3, memberObserved: 1, conditionObserved: 1},
		{name: "generation tracking disabled", memberGeneration: 1, templateGeneration: 3, templateAnnotation: 3, memberObserved: 0, conditionObserved: 0},
		{name: "missing reflected generation metadata", templateGeneration: 3, memberObserved: 1, conditionObserved: 1},
		{name: "condition current with pod observation missing", memberGeneration: 1, templateGeneration: 3, templateAnnotation: 3, memberObserved: 0, conditionObserved: 1, wantConditionObserved: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
				ObjectMeta: metav1.ObjectMeta{Name: "runner", Namespace: "default", Generation: tc.templateGeneration},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "runner"}}},
			}
			object, err := helper.ToUnstructured(pod)
			require.NoError(t, err)

			memberStatus := corev1.PodStatus{
				Phase:              corev1.PodRunning,
				PodIP:              "10.1.2.3",
				ObservedGeneration: tc.memberObserved,
				Conditions:         []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse, ObservedGeneration: tc.conditionObserved}},
				ContainerStatuses:  []corev1.ContainerStatus{{Name: "runner", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}},
			}
			raw, err := helper.BuildStatusRawExtension(&native.WrappedPodStatus{
				FederatedGeneration: native.FederatedGeneration{
					Generation:                 tc.memberGeneration,
					ResourceTemplateGeneration: tc.templateAnnotation,
				},
				PodStatus: memberStatus,
			})
			require.NoError(t, err)

			gotObject, err := copySingleMemberPodStatus(object, raw)
			require.NoError(t, err)
			var got corev1.Pod
			require.NoError(t, helper.ConvertToTypedObject(gotObject, &got))
			require.Equal(t, pod.Spec, got.Spec)
			require.Equal(t, corev1.PodRunning, got.Status.Phase)
			require.Equal(t, "10.1.2.3", got.Status.PodIP)
			require.Equal(t, "runner", got.Status.ContainerStatuses[0].Name)
			require.Equal(t, int32(0), got.Status.ContainerStatuses[0].State.Terminated.ExitCode)
			require.Equal(t, tc.wantPodObserved, got.Status.ObservedGeneration)
			require.Equal(t, tc.wantConditionObserved, got.Status.Conditions[0].ObservedGeneration)
		})
	}
}

func TestCopySingleMemberPodObservedGenerationLegacyStatus(t *testing.T) {
	pod := &corev1.Pod{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}, ObjectMeta: metav1.ObjectMeta{Generation: 3}}
	object, err := helper.ToUnstructured(pod)
	require.NoError(t, err)
	raw, err := helper.BuildStatusRawExtension(&corev1.PodStatus{
		ObservedGeneration: 1,
		Conditions:         []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, ObservedGeneration: 1}},
	})
	require.NoError(t, err)
	result, err := copySingleMemberPodStatus(object, raw)
	require.NoError(t, err)
	var copied corev1.Pod
	require.NoError(t, helper.ConvertToTypedObject(result, &copied))
	require.Zero(t, copied.Status.ObservedGeneration)
	require.Zero(t, copied.Status.Conditions[0].ObservedGeneration)
	require.Equal(t, corev1.ConditionTrue, copied.Status.Conditions[0].Status)
}

// Verify that reflection and aggregation agree on generation metadata.
func TestPodObservedGenerationReflectionRoundTrip(t *testing.T) {
	memberPod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "runner", Namespace: "default", Generation: 1,
			Annotations: map[string]string{workv1alpha2.ResourceTemplateGenerationAnnotationKey: "3"},
		},
		Status: corev1.PodStatus{
			Phase:              corev1.PodRunning,
			ObservedGeneration: 1,
			Conditions:         []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse, ObservedGeneration: 1}},
		},
	}
	memberObject, err := helper.ToUnstructured(memberPod)
	require.NoError(t, err)
	reflected, err := native.NewDefaultInterpreter().ReflectStatus(memberObject)
	require.NoError(t, err)
	require.NotNil(t, reflected)

	templatePod := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{Name: "runner", Namespace: "default", Generation: 3},
	}
	templateObject, err := helper.ToUnstructured(templatePod)
	require.NoError(t, err)
	result, err := copySingleMemberPodStatus(templateObject, reflected)
	require.NoError(t, err)
	var got corev1.Pod
	require.NoError(t, helper.ConvertToTypedObject(result, &got))
	require.Equal(t, int64(3), got.Status.ObservedGeneration)
	require.Equal(t, int64(3), got.Status.Conditions[0].ObservedGeneration)
	require.Equal(t, corev1.PodRunning, got.Status.Phase)
}

// Preserve nonzero generations as Kubernetes does to avoid repeat writes.
func TestCopySingleMemberPodObservedGenerationPreservesExisting(t *testing.T) {
	pod := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{Name: "runner", Namespace: "default", Generation: 3},
		Status: corev1.PodStatus{
			Phase:              corev1.PodPending,
			ObservedGeneration: 2,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse, ObservedGeneration: 2},
				{Type: corev1.PodInitialized, Status: corev1.ConditionTrue, ObservedGeneration: 1},
			},
		},
	}
	object, err := helper.ToUnstructured(pod)
	require.NoError(t, err)

	// Only Initialized is current; condition order differs from the template.
	status := corev1.PodStatus{
		Phase:              corev1.PodRunning,
		ObservedGeneration: 1,
		Conditions: []corev1.PodCondition{
			{Type: corev1.PodInitialized, Status: corev1.ConditionTrue, ObservedGeneration: 2},
			{Type: corev1.PodReady, Status: corev1.ConditionFalse, ObservedGeneration: 1},
		},
	}
	raw, err := helper.BuildStatusRawExtension(&native.WrappedPodStatus{
		FederatedGeneration: native.FederatedGeneration{Generation: 2, ResourceTemplateGeneration: 3},
		PodStatus:           status,
	})
	require.NoError(t, err)

	first, err := copySingleMemberPodStatus(object, raw)
	require.NoError(t, err)
	var got corev1.Pod
	require.NoError(t, helper.ConvertToTypedObject(first, &got))
	require.Equal(t, corev1.PodRunning, got.Status.Phase)
	require.Equal(t, int64(2), got.Status.ObservedGeneration)
	require.Equal(t, int64(3), got.Status.Conditions[0].ObservedGeneration)
	require.Equal(t, int64(2), got.Status.Conditions[1].ObservedGeneration)

	second, err := copySingleMemberPodStatus(first, raw)
	require.NoError(t, err)
	require.Same(t, first, second, "unchanged reflected status must not trigger another UpdateStatus")
}
