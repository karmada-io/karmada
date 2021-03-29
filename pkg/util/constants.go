package util

const (
	// PropagationPolicyNamespaceLabel is added to objects to specify associated PropagationPolicy namespace.
	PropagationPolicyNamespaceLabel = "propagationpolicy.karmada.io/namespace"

	// PropagationPolicyNameLabel is added to objects to specify associated PropagationPolicy's name.
	PropagationPolicyNameLabel = "propagationpolicy.karmada.io/name"

	// ClusterPropagationPolicyLabel is added to objects to specify associated ClusterPropagationPolicy.
	ClusterPropagationPolicyLabel = "clusterpropagationpolicy.karmada.io/name"

	// ResourceBindingNamespaceLabel is added to objects to specify associated ResourceBinding's namespace.
	ResourceBindingNamespaceLabel = "resourcebinding.karmada.io/namespace"

	// ResourceBindingNameLabel is added to objects to specify associated ResourceBinding's name.
	ResourceBindingNameLabel = "resourcebinding.karmada.io/name"

	// ClusterResourceBindingLabel is added to objects to specify associated ClusterResourceBinding.
	ClusterResourceBindingLabel = "clusterresourcebinding.karmada.io/name"

	// WorkNamespaceLabel is added to objects to specify associated Work's namespace.
	WorkNamespaceLabel = "work.karmada.io/namespace"

	// WorkNameLabel is added to objects to specify associated Work's name.
	WorkNameLabel = "work.karmada.io/name"
)

// Define annotations used by karmada system.
const (
	// PolicyPlacementAnnotation is the annotation of a policy's placement.
	// It is intended to set on ResourceBinding or ClusterResourceBinding objects to record applied placement declaration.
	// The placement could be either PropagationPolicy's or ClusterPropagationPolicy's.
	PolicyPlacementAnnotation = "policy.karmada.io/applied-placement"

	// AppliedOverrides is the annotation which used to record override items an object applied.
	// It is intended to set on Work objects to record applied overrides.
	// The overrides items should be sorted alphabetically in ascending order by OverridePolicy's name.
	AppliedOverrides = "policy.karmada.io/applied-overrides"

	// AppliedClusterOverrides is the annotation which used to record override items an object applied.
	// It is intended to set on Work objects to record applied overrides.
	// The overrides items should be sorted alphabetically in ascending order by ClusterOverridePolicy's name.
	AppliedClusterOverrides = "policy.karmada.io/applied-cluster-overrides"
)

// Define finalizers used by karmada system.
const (
	// ClusterControllerFinalizer is added to Cluster to ensure Work as well as the
	// execution space (namespace) is deleted before itself is deleted.
	ClusterControllerFinalizer = "karmada.io/cluster-controller"

	// ExecutionControllerFinalizer is added to Work to ensure manifests propagated to member cluster
	// is deleted before Work itself is deleted.
	ExecutionControllerFinalizer = "karmada.io/execution-controller"
)
