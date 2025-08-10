package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceSelector struct definition
type ResourceSelector struct {
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

func (in *ResourceSelector) GetObjectKind() schema.ObjectKind {
	return &schema.EmptyObjectKind
}

// DeepCopy copies the receiver, creating a new ResourceSelector.
func (in *ResourceSelector) DeepCopy() *ResourceSelector {
	if in == nil {
		return nil
	}
	out := new(ResourceSelector)
	if in.LabelSelector != nil {
		out.LabelSelector = in.LabelSelector.DeepCopy()
	}
	return out
}

// Add this method to satisfy runtime.Object:
func (in *ResourceSelector) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
