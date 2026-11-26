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

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func TestCopySingleMemberPodStatus(t *testing.T) {
	started := true
	pod := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "runner-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "runner"}, {Name: "sidecar"}}},
	}
	original, err := helper.ToUnstructured(pod)
	if !assert.NoError(t, err) {
		return
	}
	status := corev1.PodStatus{
		Phase:      corev1.PodRunning,
		PodIP:      "10.1.2.3",
		Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}},
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name:                 "runner",
				Ready:                false,
				Image:                "example/runner:latest",
				RestartCount:         2,
				Started:              &started,
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 137, Reason: "OOMKilled"}},
				State:                corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}},
			},
			{Name: "sidecar", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
		},
	}
	raw, err := helper.BuildStatusRawExtension(status)
	if !assert.NoError(t, err) {
		return
	}
	copied, err := copySingleMemberPodStatus(original, raw)
	if !assert.NoError(t, err) {
		return
	}
	var result corev1.Pod
	if !assert.NoError(t, helper.ConvertToTypedObject(copied, &result)) {
		return
	}
	assert.Equal(t, pod.Name, result.Name)
	assert.Equal(t, pod.Spec, result.Spec)
	assert.Equal(t, status, result.Status)
}

func TestPlacementAwareStatusRespectsRIC(t *testing.T) {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "Pod", APIVersion: "v1",
		},
		Status: corev1.PodStatus{Phase: corev1.PodPending},
	}

	object, err := helper.ToUnstructured(pod)
	if !assert.NoError(t, err) {
		return
	}

	raw, err := helper.BuildStatusRawExtension(&corev1.PodStatus{
		Phase: corev1.PodRunning,
	})
	if !assert.NoError(t, err) {
		return
	}

	configurable := declarative.NewConfigurableInterpreter(nil)
	configurable.LoadConfig([]*configv1alpha1.ResourceInterpreterCustomization{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-status-override"},
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Target: configv1alpha1.CustomizationTarget{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				Customizations: configv1alpha1.CustomizationRules{
					StatusAggregation: &configv1alpha1.StatusAggregation{
						LuaScript: `
function AggregateStatus(desiredObj, statusItems)
    desiredObj.status = {phase = "Succeeded"}
    return desiredObj
end
`,
					},
				},
			},
		},
	})

	interpreter := &customResourceInterpreterImpl{
		configurableInterpreter: configurable,
	}

	result, err := interpreter.AggregateStatusWithPlacement(
		object,
		[]workv1alpha2.AggregatedStatusItem{
			{ClusterName: "member-a", Applied: true, Status: raw},
		},
		true,
	)
	if !assert.NoError(t, err) {
		return
	}

	var resultPod corev1.Pod
	if assert.NoError(t, helper.ConvertToTypedObject(result, &resultPod)) {
		assert.Equal(t, corev1.PodSucceeded, resultPod.Status.Phase)
	}
}
