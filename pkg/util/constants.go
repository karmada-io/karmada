package util

const (
	// PolicyClaimLabel will set in kubernetes resource, indicates that
	// the resource is occupied by propagationPolicy
	PolicyClaimLabel = "karmada.io/driven-by"
	// OwnerLabel will set in karmada CRDs, indicates that who created it.
	// We can use labelSelector to find who created it quickly.
	// example1: set it in propagationBinding, the label value is propagationPolicy.
	// example2: set it in propagationWork, the label value is propagationBinding.
	OwnerLabel = "karmada.io/created-by"
	// OverrideClaimKey will set in propagationwork resource, indicates that
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
	// MemberClusterControllerFinalizer is added to MemberCluster to ensure PropagationWork as well as the
	// execution space (namespace) is deleted before itself is deleted.
	MemberClusterControllerFinalizer = "karmada.io/membercluster-controller"

	// ExecutionControllerFinalizer is added to PropagationWork to ensure manifests propagated to member cluster
	// is deleted before PropagationWork itself is deleted.
	ExecutionControllerFinalizer = "karmada.io/execution-controller"
)
