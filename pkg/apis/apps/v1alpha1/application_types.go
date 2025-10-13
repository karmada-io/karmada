package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceKindApplication is kind name of Application.
	ResourceKindApplication = "Application"
	// ResourceSingularApplication is singular name of Application.
	ResourceSingularApplication = "application"
	// ResourcePluralApplication is plural name of Application.
	ResourcePluralApplication = "applications"
	// ResourceNamespaceScopedApplication indicates if Application is NamespaceScoped.
	ResourceNamespaceScopedApplication = true
)

// Application represents a Karmada application
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ApplicationSpec `json:"spec,omitempty"`
}

// ApplicationSpec defines the desired state of Application
type ApplicationSpec struct {
	// RelatedApplications specifies other applications that should be migrated together during failover.
	// These applications must exist in the same namespace as this application.
	// +optional
	RelatedApplications []string `json:"relatedApplications,omitempty"`
}

// ApplicationList contains a list of Application
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}
