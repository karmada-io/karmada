package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kquota

// KarmadaQuota sets aggregate quota restrictions enforced per namespace
type KarmadaQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the desired quota.
	// +optional
	Spec KarmadaQuotaSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// Status defines the actual enforced quota and its current usage.
	// +optional
	Status KarmadaQuotaStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// KarmadaQuotaSpec defines the desired hard limits to enforce for Quota.
type KarmadaQuotaSpec struct {
	// hard is the set of desired hard limits for each named resource.
	// +optional
	Hard corev1.ResourceList `json:"hard,omitempty" protobuf:"bytes,1,rep,name=hard,casttype=ResourceList,castkey=ResourceName"`
	// A collection of filters that must match each object tracked by a quota.
	// If not specified, the quota matches all objects.
	// +optional
	Scopes []corev1.ResourceQuotaScope `json:"scopes,omitempty" protobuf:"bytes,2,rep,name=scopes,casttype=ResourceQuotaScope"`
	// scopeSelector is also a collection of filters like scopes that must match each object tracked by a quota
	// but expressed using ScopeSelectorOperator in combination with possible values.
	// For a resource to match, both scopes AND scopeSelector (if specified in spec), must be matched.
	// +optional
	ScopeSelector *corev1.ScopeSelector `json:"scopeSelector,omitempty" protobuf:"bytes,3,opt,name=scopeSelector"`
}

// KarmadaQuotaStatus defines the enforced hard limits and observed use.
type KarmadaQuotaStatus struct {
	// Hard is the set of enforced hard limits for each named resource.
	// +optional
	Hard corev1.ResourceList `json:"hard,omitempty" protobuf:"bytes,1,rep,name=hard,casttype=ResourceList,castkey=ResourceName"`
	// Used is the current observed total usage of the resource in the namespace.
	// +optional
	Used corev1.ResourceList `json:"used,omitempty" protobuf:"bytes,2,rep,name=used,casttype=ResourceList,castkey=ResourceName"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KarmadaQuotaList is a list of KarmadaQuota resources.
type KarmadaQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []KarmadaQuota `json:"items" protobuf:"bytes,2,rep,name=items"`
}
