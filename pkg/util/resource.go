package util

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	schedutil "k8s.io/kubernetes/pkg/scheduler/util"
)

// Resource is a collection of compute resource.
type Resource struct {
	MilliCPU int64
	Memory   int64

	// ScalarResources
	ScalarResources map[corev1.ResourceName]int64
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
			if schedutil.IsScalarResourceName(rName) {
				r.AddScalar(rName, rQuant.Value())
			}
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
			if schedutil.IsScalarResourceName(rName) {
				if value := rQuant.Value(); value > r.ScalarResources[rName] {
					r.SetScalar(rName, value)
				}
			}
		}
	}
}

// AddScalar adds a resource by a scalar value of this resource.
func (r *Resource) AddScalar(name corev1.ResourceName, quantity int64) {
	r.SetScalar(name, r.ScalarResources[name]+quantity)
}

// SetScalar sets a resource by a scalar value of this resource.
func (r *Resource) SetScalar(name corev1.ResourceName, quantity int64) {
	// Lazily allocate scalar resource map.
	if r.ScalarResources == nil {
		r.ScalarResources = map[corev1.ResourceName]int64{}
	}
	r.ScalarResources[name] = quantity
}

// ResourceList returns a resource list of this resource.
func (r *Resource) ResourceList() corev1.ResourceList {
	result := corev1.ResourceList{
		corev1.ResourceCPU:    *resource.NewMilliQuantity(r.MilliCPU, resource.DecimalSI),
		corev1.ResourceMemory: *resource.NewQuantity(r.Memory, resource.BinarySI),
	}
	for rName, rQuant := range r.ScalarResources {
		if v1helper.IsHugePageResourceName(rName) {
			result[rName] = *resource.NewQuantity(rQuant, resource.BinarySI)
		} else {
			result[rName] = *resource.NewQuantity(rQuant, resource.DecimalSI)
		}
	}
	return result
}

// AddPodRequest add the effective request resource of a pod to the origin resource.
// The Pod's effective request is the higher of:
// - the sum of all app containers(spec.Containers) request for a resource.
// - the effective init containers(spec.InitContainers) request for a resource.
// The effective init containers request is the highest request on all init containers.
func (r *Resource) AddPodRequest(podSpec *corev1.PodSpec) *Resource {
	for _, container := range podSpec.Containers {
		r.Add(container.Resources.Requests)
	}
	for _, container := range podSpec.InitContainers {
		r.SetMaxResource(container.Resources.Requests)
	}
	return r
}
