package util

const (
	// PropagationPolicyNamespaceLabel is added to objects to specify associated PropagationPolicy namespace.
	PropagationPolicyNamespaceLabel = "propagationpolicy.karmada.io/namespace"

	// PropagationPolicyNameLabel is added to objects to specify associated PropagationPolicy's name.
	PropagationPolicyNameLabel = "propagationpolicy.karmada.io/name"

	// ClusterPropagationPolicyLabel is added to objects to specify associated ClusterPropagationPolicy.
	ClusterPropagationPolicyLabel = "clusterpropagationpolicy.karmada.io/name"

	// OwnerLabel will set in karmada CRDs, indicates that who created it.
	// We can use labelSelector to find who created it quickly.
	// example1: set it in propagationBinding, the label value is propagationPolicy.
	// example2: set it in Work, the label value is propagationBinding.
	// example3: set it in Work, the label value is HPA.
	OwnerLabel = "karmada.io/created-by"
	// OverrideClaimKey will set in Work resource, indicates that
	// the resource is overridden by override policies
	OverrideClaimKey = "karmada.io/overridden-by"

	// AppliedOverrideKey is the key of a OverridePolicy list.
	// It is used to annotates what override policies have been applied for a specific manifest.
	// The value is a comma-separated list of override policy names, the namespace is omitted.
	AppliedOverrideKey = "karmada.io/override"

	// AppliedClusterOverrideKey is the key of a ClusterOverridePolicy list.
	// it is used to annotates what cluster override policies have been applied for a specific manifest.
	// The value is a comma-separated list of cluster override policy names.
	AppliedClusterOverrideKey = "karmada.io/cluster-override"
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
