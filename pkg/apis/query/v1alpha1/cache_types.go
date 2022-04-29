package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceKindClusterCache is kind name of ClusterCache.
	ResourceKindClusterCache = "ClusterCache"
	// ResourceSingularClusterCache is singular name of ClusterCache.
	ResourceSingularClusterCache = "clusterCache"
	// ResourcePluralClusterCache is plural name of ClusterCache.
	ResourcePluralClusterCache = "clusterCaches"
	// ResourceNamespaceScopedClusterCache indicates if ClusterCache is not NamespaceScoped.
	ResourceNamespaceScopedClusterCache = false
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster"

// ClusterCache defines a list of member cluster to be cached.
type ClusterCache struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of ClusterCache.
	Spec ClusterCacheSpec `json:"spec,omitempty"`

	// Status represents the status of ClusterCache.
	// +optional
	Status ClusterCacheStatus `json:"status,omitempty"`
}

// ClusterCacheSpec defines the desired state of ClusterCache
type ClusterCacheSpec struct {
	// ClusterSelectors represents the filter to select clusters.
	// +required
	ClusterSelectors []ClusterSelector `json:"clusterSelectors"`

	// ResourceSelectors used to select resources.
	// +required
	ResourceSelectors []ResourceSelector `json:"resourceSelectors"`

	// StatusUpdatePeriodSeconds is the period to update the status of the resource.
	// default is 10s.
	// +optional
	StatusUpdatePeriodSeconds uint32 `json:"statusUpdatePeriodSeconds,omitempty"`
}

// ClusterSelector represents the filter to select clusters.
type ClusterSelector struct {
	// LabelSelector is a filter to select member clusters by labels.
	// If non-nil and non-empty, only the clusters match this filter will be selected.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// FieldSelector is a filter to select member clusters by fields.
	// If non-nil and non-empty, only the clusters match this filter will be selected.
	// +optional
	FieldSelector *FieldSelector `json:"fieldSelector,omitempty"`

	// ClusterNames is the list of clusters to be selected.
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`

	// ExcludedClusters is the list of clusters to be ignored.
	// +optional
	ExcludeClusters []string `json:"exclude,omitempty"`
}

// FieldSelector is a field filter.
type FieldSelector struct {
	// A list of field selector requirements.
	MatchExpressions []corev1.NodeSelectorRequirement `json:"matchExpressions,omitempty"`
}

// ResourceSelector the resources will be selected.
type ResourceSelector struct {
	// APIVersion represents the API version of the target resources.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of the target resources.
	// +required
	Kind string `json:"kind"`

	// Namespace of the target resource.
	// Default is empty, which means all namespaces.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ClusterCacheStatus defines the observed state of ClusterCache
type ClusterCacheStatus struct {
	// +optional
	ResourceStatuses []ResourceStatusRef `json:"resources,omitempty"`
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`
}

// ResourceStatusRef is the status of the resource.
type ResourceStatusRef struct {
	// +required
	Cluster string `json:"cluster"`
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
	// +optional
	Kind string `json:"kind"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +required
	State CachePhase `json:"state"`
	// +required
	TotalNum int32 `json:"totalNum"`
	// +required
	UpdateTime *metav1.Time `json:"updateTime"`
}

// CachePhase is the current state of the cache
// +enum
type CachePhase string

// These are the valid statuses of cache.
const (
	CacheRunning CachePhase = "Running"
	CacheFailed  CachePhase = "Failed"
	CacheUnknown CachePhase = "Unknown"
)

// ResourceStateRef is the state of the resource.
type ResourceStateRef struct {
	// +required
	Phase CachePhase `json:"phase"`
	// +optional
	Reason string `json:"reason"`
}

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterCacheList if a collection of ClusterCache.
type ClusterCacheList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of ClusterCache.
	Items []ClusterCache `json:"items"`
}
