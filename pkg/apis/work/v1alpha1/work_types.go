package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// ResourceKindWork is kind name of Work.
	ResourceKindWork = "Work"
	// ResourceSingularWork is singular name of Work.
	ResourceSingularWork = "work"
	// ResourcePluralWork is plural name of Work.
	ResourcePluralWork = "works"
	// ResourceNamespaceScopedWork indicates if Work is NamespaceScoped.
	ResourceNamespaceScopedWork = true
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={karmada-io},shortName=wk
// +kubebuilder:printcolumn:JSONPath=`.status.conditions[?(@.type=="Applied")].status`,name="Applied",type=string
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date

// Work defines a list of resources to be deployed on the member cluster.
type Work struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of Work.
	Spec WorkSpec `json:"spec"`

	// Status represents the status of PropagationStatus.
	// +optional
	Status WorkStatus `json:"status,omitempty"`
}

// WorkSpec defines the desired state of Work.
type WorkSpec struct {
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

// WorkStatus defines the observed state of Work.
type WorkStatus struct {
	// Conditions contain the different condition statuses for this work.
	// Valid condition types are:
	// 1. Applied represents workload in Work is applied successfully on a managed cluster.
	// 2. Progressing represents workload in Work is being applied on a managed cluster.
	// 3. Available represents workload in Work exists on the managed cluster.
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
	// +required
	Identifier ResourceIdentifier `json:"identifier"`

	// Status reflects running status of current manifest.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Status *runtime.RawExtension `json:"status,omitempty"`

	// Health represents the healthy state of the current resource.
	// There maybe different rules for different resources to achieve health status.
	// +kubebuilder:validation:Enum=Healthy;Unhealthy;Unknown
	// +optional
	Health ResourceHealth `json:"health,omitempty"`
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
	// WorkApplied represents that the resource defined in Work is
	// successfully applied on the managed cluster.
	WorkApplied string = "Applied"
	// WorkProgressing represents that the resource defined in Work is
	// in the progress to be applied on the managed cluster.
	WorkProgressing string = "Progressing"
	// WorkAvailable represents that all resources of the Work exists on
	// the managed cluster.
	WorkAvailable string = "Available"
	// WorkDegraded represents that the current state of Work does not match
	// the desired state for a certain period.
	WorkDegraded string = "Degraded"
)

// ResourceHealth represents that the health status of the reference resource.
type ResourceHealth string

const (
	// ResourceHealthy represents that the health status of the current resource
	// that applied on the managed cluster is healthy.
	ResourceHealthy ResourceHealth = "Healthy"
	// ResourceUnhealthy represents that the health status of the current resource
	// that applied on the managed cluster is unhealthy.
	ResourceUnhealthy ResourceHealth = "Unhealthy"
	// ResourceUnknown represents that the health status of the current resource
	// that applied on the managed cluster is unknown.
	ResourceUnknown ResourceHealth = "Unknown"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkList is a collection of Work.
type WorkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of Work.
	Items []Work `json:"items"`
}
