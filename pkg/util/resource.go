package util

import corev1 "k8s.io/api/core/v1"

// Resource is a collection of compute resource.
type Resource struct {
	MilliCPU int64
	Memory   int64
}

// EmptyResource creates a empty resource object and returns.
func EmptyResource() *Resource {
	return &Resource{}
}

// Add is used to add two resources.
func (r *Resource) Add(rl corev1.ResourceList) {
	if r == nil {
		return
	}

	for rName, rQuant := range rl {
		switch rName {
		case corev1.ResourceCPU:
			r.MilliCPU += rQuant.MilliValue()
		case corev1.ResourceMemory:
			r.Memory += rQuant.Value()
		default:
			continue
		}
	}
}

// SetMaxResource compares with ResourceList and takes max value for each Resource.
func (r *Resource) SetMaxResource(rl corev1.ResourceList) {
	if r == nil {
		return
	}

	for rName, rQuant := range rl {
		switch rName {
		case corev1.ResourceCPU:
			if cpu := rQuant.MilliValue(); cpu > r.MilliCPU {
				r.MilliCPU = cpu
			}
		case corev1.ResourceMemory:
			if mem := rQuant.Value(); mem > r.Memory {
				r.Memory = mem
			}
		default:
			continue
		}
	}
}

// CalculateRequestResourceWithoutInitContainers returns Pod's resource request, it does not contain
// init containers' resource request.
func CalculateRequestResourceWithoutInitContainers(pod *corev1.Pod) *Resource {
	result := EmptyResource()
	for _, container := range pod.Spec.Containers {
		result.Add(container.Resources.Requests)
	}

	return result
}

// CalculateRequestResource calculates the resource required for pod, that is the higher of:
// - the sum of all app containers(spec.Containers) request for a resource.
// - the effective init containers(spec.InitContainers) request for a resource.
// The effective init containers request is the highest request on all init containers.
func CalculateRequestResource(pod *corev1.Pod) *Resource {
	result := CalculateRequestResourceWithoutInitContainers(pod)

	for _, container := range pod.Spec.InitContainers {
		result.SetMaxResource(container.Resources.Requests)
	}

	return result
}
