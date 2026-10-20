package util

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestAddPodRequestNPUAccumulationRegression(t *testing.T) {
	npu := corev1.ResourceName("huawei.com/Ascend910")
	always := corev1.ContainerRestartPolicyAlways

	tests := []struct {
		name        string
		useInit     bool
		restartable bool
	}{
		{"ordinary containers", false, false},
		{"ordinary init containers", true, false},
		{"restartable init sidecars", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total := EmptyResource()

			for i := 0; i < 3; i++ {
				container := corev1.Container{
					Name: "runner",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							npu: resource.MustParse("1"),
						},
					},
				}

				spec := &corev1.PodSpec{
					Containers: []corev1.Container{container},
				}

				if tt.useInit {
					spec.Containers[0].Resources =
						corev1.ResourceRequirements{}

					init := container
					init.Name = "dind"

					if tt.restartable {
						init.RestartPolicy = &always
					}

					spec.InitContainers = []corev1.Container{init}
				}

				total.AddPodRequest(spec)
			}

			if got := total.ScalarResources[npu]; got != 3 {
				t.Errorf(
					"NPU accumulation: got %d, want 3",
					got,
				)
			}
		})
	}
}
