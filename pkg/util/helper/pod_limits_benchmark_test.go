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

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

var benchmarkReplicaRequirements *workv1alpha2.ReplicaRequirements

func BenchmarkGenerateReplicaRequirements(b *testing.B) {
	template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{
		{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}, Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")}}},
		{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}, Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("700m")}}},
	}}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkReplicaRequirements = GenerateReplicaRequirements(template)
	}
}
