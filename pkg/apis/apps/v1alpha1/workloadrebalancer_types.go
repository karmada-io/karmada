/*
Copyright 2024 The Karmada Authors.

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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// ResourceKindWorkloadRebalancer is kind name of WorkloadRebalancer.
	ResourceKindWorkloadRebalancer = "WorkloadRebalancer"
	// ResourceSingularWorkloadRebalancer is singular name of WorkloadRebalancer.
	ResourceSingularWorkloadRebalancer = "workloadrebalancer"
	// ResourcePluralWorkloadRebalancer is kind plural name of WorkloadRebalancer.
	ResourcePluralWorkloadRebalancer = "workloadrebalancers"
	// ResourceNamespaceScopedWorkloadRebalancer indicates if WorkloadRebalancer is NamespaceScoped.
	ResourceNamespaceScopedWorkloadRebalancer = false
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:path=workloadrebalancers,scope="Cluster"
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadRebalancer represents the desired behavior and status of a job which can enforces a resource rebalance.
type WorkloadRebalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the specification of the desired behavior of WorkloadRebalancer.
	// +required
	Spec WorkloadRebalancerSpec `json:"spec"`

	// Status represents the status of WorkloadRebalancer.
	// +optional
	Status WorkloadRebalancerStatus `json:"status,omitempty"`
}

// WorkloadRebalancerSpec represents the specification of the desired behavior of Reschedule.
type WorkloadRebalancerSpec struct {
	// Workloads used to specify the list of expected resource.
	// Nil or empty list is not allowed.
	// +kubebuilder:validation:MinItems=1
	// +required
	Workloads []ObjectReference `json:"workloads"`

	// TTLSecondsAfterFinished limits the lifetime of a WorkloadRebalancer that has finished execution (means each
	// target workload is finished with result of Successful or Failed).
	// If this field is set, ttlSecondsAfterFinished after the WorkloadRebalancer finishes, it is eligible to be automatically deleted.
	// If this field is unset, the WorkloadRebalancer won't be automatically deleted.
	// If this field is set to zero, the WorkloadRebalancer becomes eligible to be deleted immediately after it finishes.
	// +optional
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}

// ObjectReference the expected resource.
type ObjectReference struct {
	// APIVersion represents the API version of the target resource.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of the target resource.
	// +required
	Kind string `json:"kind"`

	// Name of the target resource.
	// +required
	Name string `json:"name"`

	// Namespace of the target resource.
	// Default is empty, which means it is a non-namespacescoped resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// WorkloadRebalancerStatus contains information about the current status of a WorkloadRebalancer
// updated periodically by schedule trigger controller.
type WorkloadRebalancerStatus struct {
	// ObservedWorkloads contains information about the execution states and messages of target resources.
	// +optional
	ObservedWorkloads []ObservedWorkload `json:"observedWorkloads,omitempty"`

	// ObservedGeneration is the generation(.metadata.generation) observed by the controller.
	// If ObservedGeneration is less than the generation in metadata means the controller hasn't confirmed
	// the rebalance result or hasn't done the rebalance yet.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// FinishTime represents the finish time of rebalancer.
	// +optional
	FinishTime *metav1.Time `json:"finishTime,omitempty"`
}

// ObservedWorkload the observed resource.
type ObservedWorkload struct {
	// Workload the observed resource.
	// +required
	Workload ObjectReference `json:"workload"`

	// Result the observed rebalance result of resource.
	// +optional
	Result RebalanceResult `json:"result,omitempty"`

	// Reason represents a machine-readable description of why this resource rebalanced failed.
	// +optional
	Reason RebalanceFailedReason `json:"reason,omitempty"`
}

// RebalanceResult the specific extent to which the resource has been rebalanced
type RebalanceResult string

const (
	// RebalanceFailed the resource has been rebalance failed.
	RebalanceFailed RebalanceResult = "Failed"
	// RebalanceSuccessful the resource has been successfully rebalanced.
	RebalanceSuccessful RebalanceResult = "Successful"
)

// RebalanceFailedReason represents a machine-readable description of why this resource rebalanced failed.
type RebalanceFailedReason string

const (
	// RebalanceObjectNotFound the resource referenced binding not found.
	RebalanceObjectNotFound RebalanceFailedReason = "ReferencedBindingNotFound"
)

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadRebalancerList contains a list of WorkloadRebalancer
type WorkloadRebalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of WorkloadRebalancer.
	Items []WorkloadRebalancer `json:"items"`
}
