package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

// Workload is a simple Deployment.
type Workload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the specification of the desired behavior.
	// +required
	Spec WorkloadSpec `json:"spec"`

	// Status represents most recently observed status of the Workload.
	// +optional
	Status WorkloadStatus `json:"status,omitempty"`
}

// WorkloadSpec is the specification of the desired behavior of the Workload.
type WorkloadSpec struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Template describes the pods that will be created.
	Template corev1.PodTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"`

	// Paused indicates that the deployment is paused.
	// Note: both user and controllers might set this field.
	// +optional
	Paused bool `json:"paused,omitempty"`
}

// WorkloadStatus represents most recently observed status of the Workload.
type WorkloadStatus struct {
	// ReadyReplicas represents the total number of ready pods targeted by this Workload.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Conditions is an array of current cluster conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
