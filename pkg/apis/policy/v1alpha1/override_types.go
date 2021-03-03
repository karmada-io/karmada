package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OverridePolicy represents the policy that overrides a group of resources to one or more clusters.
type OverridePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of OverridePolicy.
	Spec OverrideSpec `json:"spec"`
}

// OverrideSpec defines the desired behavior of OverridePolicy.
type OverrideSpec struct {
	// ResourceSelectors restricts resource types that this override policy applies to.
	ResourceSelectors []ResourceSelector `json:"resourceSelectors,omitempty"`

	// TargetCluster defines restrictions on this override policy
	// that only applies to resources propagated to the matching clusters
	TargetCluster ClusterAffinity `json:"targetCluster,omitempty"`

	// Overriders represents the override rules that would apply on resources
	Overriders Overriders `json:"overriders,omitempty"`
}

// Overriders represents the override rules that would apply on resources
type Overriders struct {
	// Plaintext represents override rules defined with plaintext overriders.
	Plaintext []PlaintextOverrider `json:"plaintext,omitempty"`
}

// PlaintextOverrider is a simple overrider that overrides target fields
// according to path, operator and value.
type PlaintextOverrider struct {
	// Path indicates the path of target field
	Path string `json:"path"`
	// Operator indicates the operation on target field.
	// Available operators are: add, update and remove.
	// +kubebuilder:validation:Enum=add;remove;replace
	Operator OverriderOperator `json:"operator"`
	// Value to be applied to target field.
	// Must be empty when operator is Remove.
	// +optional
	Value apiextensionsv1.JSON `json:"value,omitempty"`
}

// OverriderOperator is the set of operators that can be used in an overrider.
type OverriderOperator string

// These are valid overrider operators.
const (
	OverriderOpAdd     OverriderOperator = "add"
	OverriderOpRemove  OverriderOperator = "remove"
	OverriderOpReplace OverriderOperator = "replace"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OverridePolicyList is a collection of OverridePolicy.
type OverridePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of OverridePolicy.
	Items []OverridePolicy `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterOverridePolicy represents the cluster-wide policy that overrides a group of resources to one or more clusters.
type ClusterOverridePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of ClusterOverridePolicy.
	Spec OverrideSpec `json:"spec"`
}

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterOverridePolicyList is a collection of ClusterOverridePolicy.
type ClusterOverridePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of ClusterOverridePolicy.
	Items []ClusterOverridePolicy `json:"items"`
}
