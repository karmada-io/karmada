package util

import corev1 "k8s.io/api/core/v1"

// Resource is a collection of compute resource.
type Resource struct {
	MilliCPU int64
	Memory   int64
}

// EmptyResource creates a empty resource object and returns
func EmptyResource() *Resource {
	return &Resource{}
}

// NewResource creates a new resource object from resource list
func NewResource(rl corev1.ResourceList) *Resource {
	r := EmptyResource()
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
	return r
}

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

// GetPodResourceWithoutInitContainers returns Pod's resource request, it does not contain
// init containers' resource request.
func GetPodResourceWithoutInitContainers(pod *corev1.Pod) *Resource {
	result := EmptyResource()
	for _, container := range pod.Spec.Containers {
		result.Add(container.Resources.Requests)
	}

	return result
}
