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

package helper

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

func limits(cpu, memory string) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(cpu),
		corev1.ResourceMemory: resource.MustParse(memory),
	}
}

func assertLimit(t *testing.T, got corev1.ResourceList, name corev1.ResourceName, want string) {
	t.Helper()
	quantity, found := got[name]
	if !found || quantity.Cmp(resource.MustParse(want)) != 0 {
		t.Fatalf("%s: got %v, want %s", name, quantity.String(), want)
	}
}

func TestPodTemplateLimitsInitPeaks(t *testing.T) {
	template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{
		Containers: []corev1.Container{
			{Resources: corev1.ResourceRequirements{Limits: limits("250m", "128Mi"), Requests: limits("100m", "64Mi")}},
			{Resources: corev1.ResourceRequirements{Limits: limits("500m", "128Mi"), Requests: limits("200m", "64Mi")}},
		},
		InitContainers: []corev1.Container{
			{Resources: corev1.ResourceRequirements{Limits: limits("1500m", "192Mi")}},
			{Resources: corev1.ResourceRequirements{Limits: limits("750m", "512Mi")}},
		},
	}}
	got := PodTemplateLimits(template)
	assertLimit(t, got, corev1.ResourceCPU, "1500m")
	assertLimit(t, got, corev1.ResourceMemory, "512Mi")
	assertLimit(t, template.Spec.Containers[0].Resources.Requests, corev1.ResourceCPU, "100m")
}

func TestPodTemplateLimitsRestartableInit(t *testing.T) {
	template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")}}}},
		InitContainers: []corev1.Container{
			{RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways), Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}}},
			{Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("900m")}}},
			{RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways), Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("300m")}}},
		},
	}}
	assertLimit(t, PodTemplateLimits(template), corev1.ResourceCPU, "1100m")
}

func TestPodTemplateLimitsOverheadAndPodLevel(t *testing.T) {
	template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Resources: corev1.ResourceRequirements{Limits: limits("500m", "128Mi")}}},
		Overhead:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m"), corev1.ResourceEphemeralStorage: resource.MustParse("1Gi")},
		Resources:  &corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}},
	}}
	got := PodTemplateLimits(template)
	assertLimit(t, got, corev1.ResourceCPU, "1050m")
	if _, found := got[corev1.ResourceEphemeralStorage]; found {
		t.Fatal("overhead must not create a missing limit")
	}
	assertLimit(t, template.Spec.Resources.Limits, corev1.ResourceCPU, "1")
	assertLimit(t, PodTemplateLimits(template), corev1.ResourceCPU, "1050m")
}

func TestPodTemplateLimitsPodLevelIndependent(t *testing.T) {
	template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{
		Resources: &corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("9223372036854775808")}},
	}}
	got := PodTemplateLimits(template)
	quantity := got[corev1.ResourceCPU]
	quantity.Add(resource.MustParse("1"))
	got[corev1.ResourceCPU] = quantity
	assertLimit(t, template.Spec.Resources.Limits, corev1.ResourceCPU, "9223372036854775808")
}

func TestPodTemplateLimitsExactAndIndependent(t *testing.T) {
	template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{
		{Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("9007199254740992")}}},
		{Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1")}}},
	}}}
	got := PodTemplateLimits(template)
	assertLimit(t, got, corev1.ResourceMemory, "9007199254740993")
	changed := got[corev1.ResourceMemory]
	changed.Add(resource.MustParse("1"))
	got[corev1.ResourceMemory] = changed
	assertLimit(t, template.Spec.Containers[0].Resources.Limits, corev1.ResourceMemory, "9007199254740992")
	assertLimit(t, PodTemplateLimits(template), corev1.ResourceMemory, "9007199254740993")
	if PodTemplateLimits(nil) != nil {
		t.Fatal("nil template must return nil")
	}
}

func TestPodTemplateLimitsExplicitZero(t *testing.T) {
	template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0")}}}}}}
	got := PodTemplateLimits(template)
	assertLimit(t, got, corev1.ResourceCPU, "0")
	if _, found := got[corev1.ResourceMemory]; found {
		t.Fatal("missing memory limit must stay missing")
	}
}
