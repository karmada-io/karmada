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

package native

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func TestReflectPodStatusWithGeneration(t *testing.T) {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "runner", Namespace: "default", Generation: 2,
			Annotations: map[string]string{workv1alpha2.ResourceTemplateGenerationAnnotationKey: "4"},
		},
		Status: corev1.PodStatus{
			Phase:              corev1.PodRunning,
			ObservedGeneration: 2,
			Conditions:         []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse, ObservedGeneration: 1}},
			ContainerStatuses:  []corev1.ContainerStatus{{Name: "runner", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}},
		},
	}
	obj, err := helper.ToUnstructured(pod)
	require.NoError(t, err)
	raw, err := reflectPodStatus(obj)
	require.NoError(t, err)
	require.NotNil(t, raw)

	var reflected WrappedPodStatus
	require.NoError(t, json.Unmarshal(raw.Raw, &reflected))
	require.Equal(t, int64(2), reflected.Generation)
	require.Equal(t, int64(4), reflected.ResourceTemplateGeneration)
	require.Equal(t, pod.Status, reflected.PodStatus)
}

func TestReflectPodStatusMissingOrInvalidGeneration(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{"name": "runner", "namespace": "default"},
		"status":   map[string]any{"phase": "Running"},
	}}
	raw, err := reflectPodStatus(obj)
	require.NoError(t, err)
	require.NotNil(t, raw)
	var reflected WrappedPodStatus
	require.NoError(t, json.Unmarshal(raw.Raw, &reflected))
	require.Zero(t, reflected.Generation)
	require.Zero(t, reflected.ResourceTemplateGeneration)
	require.Equal(t, corev1.PodRunning, reflected.Phase)

	obj.SetAnnotations(map[string]string{workv1alpha2.ResourceTemplateGenerationAnnotationKey: "not-a-number"})
	_, err = reflectPodStatus(obj)
	require.Error(t, err)

	delete(obj.Object, "status")
	raw, err = reflectPodStatus(obj)
	require.NoError(t, err)
	require.Nil(t, raw)
}
