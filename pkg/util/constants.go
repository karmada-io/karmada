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
