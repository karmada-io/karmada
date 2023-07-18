package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

const (
	// ResourceKindResourceBinding is kind name of ResourceBinding.
	ResourceKindResourceBinding = "ResourceBinding"
	// ResourceSingularResourceBinding is singular name of ResourceBinding.
	ResourceSingularResourceBinding = "resourcebinding"
	// ResourcePluralResourceBinding is plural name of ResourceBinding.
	ResourcePluralResourceBinding = "resourcebindings"
	// ResourceNamespaceScopedResourceBinding indicates if ResourceBinding is NamespaceScoped.
	ResourceNamespaceScopedResourceBinding = true

	// ResourceKindClusterResourceBinding is kind name of ClusterResourceBinding.
	ResourceKindClusterResourceBinding = "ClusterResourceBinding"
	// ResourceSingularClusterResourceBinding is singular name of ClusterResourceBinding.
	ResourceSingularClusterResourceBinding = "clusterresourcebinding"
	// ResourcePluralClusterResourceBinding is kind plural of ClusterResourceBinding.
	ResourcePluralClusterResourceBinding = "clusterresourcebindings"
	// ResourceNamespaceScopedClusterResourceBinding indicates if ClusterResourceBinding is NamespaceScoped.
	ResourceNamespaceScopedClusterResourceBinding = false
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=rb,categories={karmada-io}
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:JSONPath=`.status.conditions[?(@.type=="Scheduled")].status`,name="Scheduled",type=string
// +kubebuilder:printcolumn:JSONPath=`.status.conditions[?(@.type=="FullyApplied")].status`,name="FullyApplied",type=string
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date

// ResourceBinding represents a binding of a kubernetes resource with a propagation policy.
type ResourceBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior.
	Spec ResourceBindingSpec `json:"spec"`

	// Status represents the most recently observed status of the ResourceBinding.
	// +optional
	Status ResourceBindingStatus `json:"status,omitempty"`
}

// ResourceBindingSpec represents the expectation of ResourceBinding.
type ResourceBindingSpec struct {
	// Resource represents the Kubernetes resource to be propagated.
	Resource ObjectReference `json:"resource"`

	// PropagateDeps tells if relevant resources should be propagated automatically.
	// It is inherited from PropagationPolicy or ClusterPropagationPolicy.
	// default false.
	// +optional
	PropagateDeps bool `json:"propagateDeps,omitempty"`

	// ReplicaRequirements represents the requirements required by each replica.
	// +optional
	ReplicaRequirements *ReplicaRequirements `json:"replicaRequirements,omitempty"`

	// Replicas represents the replica number of the referencing resource.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Clusters represents target member clusters where the resource to be deployed.
	// +optional
	Clusters []TargetCluster `json:"clusters,omitempty"`

	// Placement represents the rule for select clusters to propagate resources.
	// +optional
	Placement *policyv1alpha1.Placement `json:"placement,omitempty"`

	// GracefulEvictionTasks holds the eviction tasks that are expected to perform
	// the eviction in a graceful way.
	// The intended workflow is:
	// 1. Once the controller(such as 'taint-manager') decided to evict the resource that
	//    is referenced by current ResourceBinding or ClusterResourceBinding from a target
	//    cluster, it removes(or scale down the replicas) the target from Clusters(.spec.Clusters)
	//    and builds a graceful eviction task.
	// 2. The scheduler may perform a re-scheduler and probably select a substitute cluster
	//    to take over the evicting workload(resource).
	// 3. The graceful eviction controller takes care of the graceful eviction tasks and
	//    performs the final removal after the workload(resource) is available on the substitute
	//    cluster or exceed the grace termination period(defaults to 10 minutes).
	//
	// +optional
	GracefulEvictionTasks []GracefulEvictionTask `json:"gracefulEvictionTasks,omitempty"`

	// RequiredBy represents the list of Bindings that depend on the referencing resource.
	// +optional
	RequiredBy []BindingSnapshot `json:"requiredBy,omitempty"`

	// SchedulerName represents which scheduler to proceed the scheduling.
	// It inherits directly from the associated PropagationPolicy(or ClusterPropagationPolicy).
	// +optional
	SchedulerName string `json:"schedulerName,omitempty"`

	// Failover indicates how Karmada migrates applications in case of failures.
	// It inherits directly from the associated PropagationPolicy(or ClusterPropagationPolicy).
	// +optional
	Failover *policyv1alpha1.FailoverBehavior `json:"failover,omitempty"`
}

// ObjectReference contains enough information to locate the referenced object inside current cluster.
type ObjectReference struct {
	// APIVersion represents the API version of the referent.
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of the referent.
	Kind string `json:"kind"`

	// Namespace represents the namespace for the referent.
	// For non-namespace scoped resources(e.g. 'ClusterRole')，do not need specify Namespace,
	// and for namespace scoped resources, Namespace is required.
	// If Namespace is not specified, means the resource is non-namespace scoped.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name represents the name of the referent.
	Name string `json:"name"`

	// UID of the referent.
	// +optional
	UID types.UID `json:"uid,omitempty"`

	// ResourceVersion represents the internal version of the referenced object, that can be used by clients to
	// determine when object has changed.
	// +optional
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

// ReplicaRequirements represents the requirements required by each replica.
type ReplicaRequirements struct {
	// NodeClaim represents the node claim HardNodeAffinity, NodeSelector and Tolerations required by each replica.
	// +optional
	NodeClaim *NodeClaim `json:"nodeClaim,omitempty"`

	// ResourceRequest represents the resources required by each replica.
	// +optional
	ResourceRequest corev1.ResourceList `json:"resourceRequest,omitempty"`
}

// NodeClaim represents the node claim HardNodeAffinity, NodeSelector and Tolerations required by each replica.
type NodeClaim struct {
	// A node selector represents the union of the results of one or more label queries over a set of
	// nodes; that is, it represents the OR of the selectors represented by the node selector terms.
	// Note that only PodSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	// is included here because it has a hard limit on pod scheduling.
	// +optional
	HardNodeAffinity *corev1.NodeSelector `json:"hardNodeAffinity,omitempty"`
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// If specified, the pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// TargetCluster represents the identifier of a member cluster.
type TargetCluster struct {
	// Name of target cluster.
	Name string `json:"name"`
	// Replicas in target cluster
	// +optional
	Replicas int32 `json:"replicas,omitempty"`
}

// GracefulEvictionTask represents a graceful eviction task.
type GracefulEvictionTask struct {
	// FromCluster which cluster the eviction perform from.
	// +required
	FromCluster string `json:"fromCluster"`

	// Replicas indicates the number of replicas should be evicted.
	// Should be ignored for resource type that doesn't have replica.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Reason contains a programmatic identifier indicating the reason for the eviction.
	// Producers may define expected values and meanings for this field,
	// and whether the values are considered a guaranteed API.
	// The value should be a CamelCase string.
	// This field may not be empty.
	// +required
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`
	Reason string `json:"reason"`

	// Message is a human-readable message indicating details about the eviction.
	// This may be an empty string.
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	Message string `json:"message,omitempty"`

	// Producer indicates the controller who triggered the eviction.
	// +required
	Producer string `json:"producer"`

	// GracePeriodSeconds is the maximum waiting duration in seconds before the item
	// should be deleted. If the application on the new cluster cannot reach a Healthy state,
	// Karmada will delete the item after GracePeriodSeconds is reached.
	// Value must be positive integer.
	// It can not co-exist with SuppressDeletion.
	// +optional
	GracePeriodSeconds *int32 `json:"gracePeriodSeconds,omitempty"`

	// SuppressDeletion represents the grace period will be persistent until
	// the tools or human intervention stops it.
	// It can not co-exist with GracePeriodSeconds.
	// +optional
	SuppressDeletion *bool `json:"suppressDeletion,omitempty"`

	// CreationTimestamp is a timestamp representing the server time when this object was
	// created.
	// Clients should not set this value to avoid the time inconsistency issue.
	// It is represented in RFC3339 form(like '2021-04-25T10:02:10Z') and is in UTC.
	//
	// Populated by the system. Read-only.
	// +optional
	CreationTimestamp metav1.Time `json:"creationTimestamp,omitempty"`
}

// BindingSnapshot is a snapshot of a ResourceBinding or ClusterResourceBinding.
type BindingSnapshot struct {
	// Namespace represents the namespace of the Binding.
	// It is required for ResourceBinding.
	// If Namespace is not specified, means the referencing is ClusterResourceBinding.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name represents the name of the Binding.
	// +required
	Name string `json:"name"`

	// Clusters represents the scheduled result.
	// +optional
	Clusters []TargetCluster `json:"clusters,omitempty"`
}

// ResourceBindingStatus represents the overall status of the strategy as well as the referenced resources.
type ResourceBindingStatus struct {
	// SchedulerObservedGeneration is the generation(.metadata.generation) observed by the scheduler.
	// If SchedulerObservedGeneration is less than the generation in metadata means the scheduler hasn't confirmed
	// the scheduling result or hasn't done the schedule yet.
	// +optional
	SchedulerObservedGeneration int64 `json:"schedulerObservedGeneration,omitempty"`

	// SchedulerObservedAffinityName is the name of affinity term that is
	// the basis of current scheduling.
	// +optional
	SchedulerObservedAffinityName string `json:"schedulerObservingAffinityName,omitempty"`

	// Conditions contain the different condition statuses.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AggregatedStatus represents status list of the resource running in each member cluster.
	// +optional
	AggregatedStatus []AggregatedStatusItem `json:"aggregatedStatus,omitempty"`
}

// AggregatedStatusItem represents status of the resource running in a member cluster.
type AggregatedStatusItem struct {
	// ClusterName represents the member cluster name which the resource deployed on.
	// +required
	ClusterName string `json:"clusterName"`

	// Status reflects running status of current manifest.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Status *runtime.RawExtension `json:"status,omitempty"`

	// Applied represents if the resource referencing by ResourceBinding or ClusterResourceBinding
	// is successfully applied on the cluster.
	// +optional
	Applied bool `json:"applied,omitempty"`

	// AppliedMessage is a human readable message indicating details about the applied status.
	// This is usually holds the error message in case of apply failed.
	// +optional
	AppliedMessage string `json:"appliedMessage,omitempty"`

	// Health represents the healthy state of the current resource.
	// There maybe different rules for different resources to achieve health status.
	// +kubebuilder:validation:Enum=Healthy;Unhealthy;Unknown
	// +optional
	Health ResourceHealth `json:"health,omitempty"`
}

// Conditions definition
const (
	// Scheduled represents the condition that the ResourceBinding or ClusterResourceBinding has been scheduled.
	Scheduled string = "Scheduled"

	// FullyApplied represents the condition that the resource referencing by ResourceBinding or ClusterResourceBinding
	// has been applied to all scheduled clusters.
	FullyApplied string = "FullyApplied"
)

// These are reasons for a binding's transition to a Scheduled condition.
const (
	// BindingReasonSuccess reason in Scheduled condition means that binding has been scheduled successfully.
	BindingReasonSuccess = "Success"

	// BindingReasonSchedulerError reason in Scheduled condition means that some internal error happens
	// during scheduling, for example due to api-server connection error.
	BindingReasonSchedulerError = "SchedulerError"

	// BindingReasonNoClusterFit reason in Scheduled condition means that scheduling has finished
	// due to no fit cluster.
	BindingReasonNoClusterFit = "NoClusterFit"

	// BindingReasonUnschedulable reason in Scheduled condition means that the scheduler can't schedule
	// the binding right now, for example due to insufficient resources in the clusters.
	BindingReasonUnschedulable = "Unschedulable"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceBindingList contains a list of ResourceBinding.
type ResourceBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of ResourceBinding.
	Items []ResourceBinding `json:"items"`
}

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

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope="Cluster",shortName=crb,categories={karmada-io}
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:JSONPath=`.status.conditions[?(@.type=="Scheduled")].status`,name="Scheduled",type=string
// +kubebuilder:printcolumn:JSONPath=`.status.conditions[?(@.type=="FullyApplied")].status`,name="FullyApplied",type=string
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date

// ClusterResourceBinding represents a binding of a kubernetes resource with a ClusterPropagationPolicy.
type ClusterResourceBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior.
	Spec ResourceBindingSpec `json:"spec"`

	// Status represents the most recently observed status of the ResourceBinding.
	// +optional
	Status ResourceBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterResourceBindingList contains a list of ClusterResourceBinding.
type ClusterResourceBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of ClusterResourceBinding.
	Items []ClusterResourceBinding `json:"items"`
}
