/*
Copyright 2023 The Karmada Authors.

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

package detector

import (
	"regexp"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func BenchmarkEventFilterNoSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = nil
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "demo-deployment",
					"namespace": "benchmark",
				},
				"spec": map[string]interface{}{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterNoMatchSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^benchmark-.*$"))
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "demo-deployment",
					"namespace": "benchmark",
				},
				"spec": map[string]interface{}{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterNoWildcards(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^benchmark$"))
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "demo-deployment",
					"namespace": "benchmark-1",
				},
				"spec": map[string]interface{}{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterPrefixMatchSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^benchmark-.*$"))
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "demo-deployment",
					"namespace": "benchmark-1",
				},
				"spec": map[string]interface{}{
					"replicas": 2,
				},
			},
		})
	}
}
func BenchmarkEventFilterSuffixMatchSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^.*-benchmark$"))
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "demo-deployment",
					"namespace": "example-benchmark",
				},
				"spec": map[string]interface{}{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterMultiSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^.*-benchmark$"), regexp.MustCompile("^benchmark-.*$"))
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "demo-deployment",
					"namespace": "benchmark-1",
				},
				"spec": map[string]interface{}{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterExtensionApiserverAuthentication(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^kube-.*$"))
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      "extension-apiserver-authentication",
					"namespace": "kube-system",
				},
			},
		})
	}
}
