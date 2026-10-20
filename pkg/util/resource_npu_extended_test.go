package util

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestAddPodRequestMixedNPUResources(t *testing.T) {
	npu := corev1.ResourceName("huawei.com/Ascend910")
	always := corev1.ContainerRestartPolicyAlways

	res := func(cpu, memory, devices string) corev1.ResourceList {
		return corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpu),
			corev1.ResourceMemory: resource.MustParse(memory),
			npu:                   resource.MustParse(devices),
		}
	}

	first := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Resources: corev1.ResourceRequirements{
				Requests: res("300m", "256Mi", "1"),
			}},
		},
		InitContainers: []corev1.Container{
			{
				RestartPolicy: &always,
				Resources: corev1.ResourceRequirements{
					Requests: res("100m", "64Mi", "1"),
				},
			},
			{
				Resources: corev1.ResourceRequirements{
					Requests: res("700m", "1024Mi", "2"),
				},
			},
		},
		Overhead: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("32Mi"),
		},
	}

	second := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Resources: corev1.ResourceRequirements{
				Requests: res("250m", "128Mi", "1"),
			}},
		},
		InitContainers: []corev1.Container{
			{Resources: corev1.ResourceRequirements{
				Requests: res("100m", "512Mi", "1"),
			}},
		},
		Overhead: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("25m"),
			corev1.ResourceMemory: resource.MustParse("32Mi"),
		},
	}

	original := first.DeepCopy()

	total := EmptyResource()
	total.AddPodRequest(first)
	total.AddPodRequest(second)

	want := NewResource(res("1125m", "1664Mi", "4"))

	if !reflect.DeepEqual(total, want) {
		t.Errorf("got %+v, want %+v", total, want)
	}
	if !reflect.DeepEqual(first, original) {
		t.Error("AddPodRequest modified original PodSpec")
	}
}
