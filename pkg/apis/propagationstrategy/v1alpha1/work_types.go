package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PropagationWork defines a list of resources to be deployed on the member cluster.
type PropagationWork struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of PropagationWork.
	Spec PropagationWorkSpec `json:"spec"`

	// Status represents the status of PropagationStatus.
	// +optional
	Status PropagationWorkStatus `json:"status,omitempty"`
}

// PropagationWorkSpec defines the desired state of PropagationWork.
type PropagationWorkSpec struct {
	// Workload represents the manifest workload to be deployed on managed cluster.
	Workload WorkloadTemplate `json:"workload,omitempty"`
}

// WorkloadTemplate represents the manifest workload to be deployed on managed cluster.
type WorkloadTemplate struct {
	// Manifests represents a list of Kubernetes resources to be deployed on the managed cluster.
	// +optional
	Manifests []Manifest `json:"manifests,omitempty"`
}

// Manifest represents a resource to be deployed on managed cluster.
type Manifest struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	runtime.RawExtension `json:",inline"`
}

// PropagationWorkStatus defines the observed state of PropagationWork.
type PropagationWorkStatus struct {
	// Conditions contain the different condition statuses for this work.
	// Valid condition types are:
	// 1. Applied represents workload in PropagationWork is applied successfully on a managed cluster.
	// 2. Progressing represents workload in PropagationWork is being applied on a managed cluster.
	// 3. Available represents workload in PropagationWork exists on the managed cluster.
	// 4. Degraded represents the current state of workload does not match the desired
	// state for a certain period.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ManifestStatuses contains running status of manifests in spec.
	// +optional
	ManifestStatuses []ManifestStatus `json:"manifestStatuses,omitempty"`
}

// ManifestStatus contains running status of a specific manifest in spec.
type ManifestStatus struct {
	// Identifier represents the identity of a resource linking to manifests in spec.
	Identifier ResourceIdentifier `json:"identifier"`

	// Status reflects running status of current manifest.
	// +kubebuilder:pruning:PreserveUnknownFields
	Status runtime.RawExtension `json:",inline"`
}

// ResourceIdentifier provides the identifiers needed to interact with any arbitrary object.
type ResourceIdentifier struct {
	// Ordinal represents an index in manifests list, so the condition can still be linked
	// to a manifest even though manifest cannot be parsed successfully.
	Ordinal int `json:"ordinal"`

	// Group is the group of the resource.
	Group string `json:"group,omitempty"`

	// Version is the version of the resource.
	Version string `json:"version"`

	// Kind is the kind of the resource.
	Kind string `json:"kind"`

	// Resource is the resource type of the resource
	Resource string `json:"resource"`

	// Namespace is the namespace of the resource, the resource is cluster scoped if the value
	// is empty
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of the resource
	Name string `json:"name"`
}

const (
	// WorkApplied represents that the resource defined in PropagationWork is
	// successfully applied on the managed cluster.
	WorkApplied string = "Applied"
	// WorkProgressing represents that the resource defined in PropagationWork is
	// in the progress to be applied on the managed cluster.
	WorkProgressing string = "Progressing"
	// WorkAvailable represents that all resources of the PropagationWork exists on
	// the managed cluster.
	WorkAvailable string = "Available"
	// WorkDegraded represents that the current state of PropagationWork does not match
	// the desired state for a certain period.
	WorkDegraded string = "Degraded"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PropagationWorkList is a collection of PropagationWork.
type PropagationWorkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of PropagationWork.
	Items []PropagationWork `json:"items"`
}
