/*
Copyright 2021 The Kubernetes Authors.

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

package top

import (
	corev1 "k8s.io/api/core/v1"
	metricsapi "k8s.io/metrics/pkg/apis/metrics"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
)

type PodMetricsSorter struct {
	metrics       []metricsapi.PodMetrics
	sortBy        string
	withNamespace bool
	podMetrics    []corev1.ResourceList
}

func (p *PodMetricsSorter) Len() int {
	return len(p.metrics)
}

func (p *PodMetricsSorter) Swap(i, j int) {
	p.metrics[i], p.metrics[j] = p.metrics[j], p.metrics[i]
	p.podMetrics[i], p.podMetrics[j] = p.podMetrics[j], p.podMetrics[i]
}

func (p *PodMetricsSorter) Less(i, j int) bool {
	switch p.sortBy {
	case "cpu":
		return p.podMetrics[i].Cpu().MilliValue() > p.podMetrics[j].Cpu().MilliValue()
	case "memory":
		return p.podMetrics[i].Memory().Value() > p.podMetrics[j].Memory().Value()
	default:
		if p.metrics[i].Annotations[autoscalingv1alpha1.QuerySourceAnnotationKey] != p.metrics[j].Annotations[autoscalingv1alpha1.QuerySourceAnnotationKey] {
			return p.metrics[i].Annotations[autoscalingv1alpha1.QuerySourceAnnotationKey] < p.metrics[j].Annotations[autoscalingv1alpha1.QuerySourceAnnotationKey]
		}
		if p.withNamespace && p.metrics[i].Namespace != p.metrics[j].Namespace {
			return p.metrics[i].Namespace < p.metrics[j].Namespace
		}
		return p.metrics[i].Name < p.metrics[j].Name
	}
}

func NewPodMetricsSorter(metrics []metricsapi.PodMetrics, withNamespace bool, sortBy string) *PodMetricsSorter {
	var podMetrics = make([]corev1.ResourceList, len(metrics))
	if len(sortBy) > 0 {
		for i := range metrics {
			podMetrics[i] = getPodMetrics(&metrics[i])
		}
	}

	return &PodMetricsSorter{
		metrics:       metrics,
		sortBy:        sortBy,
		withNamespace: withNamespace,
		podMetrics:    podMetrics,
	}
}
