package v1alpha1

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceKindMultiClusterIngress is kind name of MultiClusterIngress.
	ResourceKindMultiClusterIngress = "MultiClusterIngress"
	// ResourceSingularMultiClusterIngress is singular name of MultiClusterIngress.
	ResourceSingularMultiClusterIngress = "multiclusteringress"
	// ResourcePluralMultiClusterIngress is plural name of MultiClusterIngress.
	ResourcePluralMultiClusterIngress = "multiclusteringresses"
	// ResourceNamespaceScopedMultiClusterIngress indicates if MultiClusterIngress is NamespaceScoped.
	ResourceNamespaceScopedMultiClusterIngress = true
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mci,categories={karmada-io}

// MultiClusterIngress is a collection of rules that allow inbound connections to reach the
// endpoints defined by a backend. The structure of MultiClusterIngress is same as Ingress,
// indicates the Ingress in multi-clusters.
type MultiClusterIngress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the MultiClusterIngress.
	// +optional
	Spec networkingv1.IngressSpec `json:"spec,omitempty"`

	// Status is the current state of the MultiClusterIngress.
	// +optional
	Status networkingv1.IngressStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MultiClusterIngressList is a collection of MultiClusterIngress.
type MultiClusterIngressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of MultiClusterIngress.
	Items []MultiClusterIngress `json:"items"`
}
