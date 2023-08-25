package v1alpha1

const (
	// PropagationPolicyReferenceUID is the uid of PropagationPolicy object.
	PropagationPolicyReferenceUID = "propagationpolicy.karmada.io/uid"

	// PropagationPolicyNamespaceAnnotation is added to objects to specify associated PropagationPolicy namespace.
	PropagationPolicyNamespaceAnnotation = "propagationpolicy.karmada.io/namespace"

	// PropagationPolicyNamespaceLabel is added to objects to specify associated PropagationPolicy namespace.
	PropagationPolicyNamespaceLabel = "propagationpolicy.karmada.io/namespace"

	// PropagationPolicyNameAnnotation is added to objects to specify associated PropagationPolicy name.
	PropagationPolicyNameAnnotation = "propagationpolicy.karmada.io/name"

	// PropagationPolicyNameLabel is added to objects to specify associated PropagationPolicy's name.
	PropagationPolicyNameLabel = "propagationpolicy.karmada.io/name"

	// ClusterPropagationPolicyReferenceUID is the uid of ClusterPropagationPolicy object.
	ClusterPropagationPolicyReferenceUID = "clusterpropagationpolicy.karmada.io/uid"

	// ClusterPropagationPolicyAnnotation is added to objects to specify associated ClusterPropagationPolicy name.
	ClusterPropagationPolicyAnnotation = "clusterpropagationpolicy.karmada.io/name"

	// ClusterPropagationPolicyLabel is added to objects to specify associated ClusterPropagationPolicy.
	ClusterPropagationPolicyLabel = "clusterpropagationpolicy.karmada.io/name"

	// NamespaceSkipAutoPropagationLabel is added to namespace objects to indicate if
	// the namespace should be skipped from propagating by the namespace controller.
	// For example, a namespace with the following label will be skipped:
	//   labels:
	//     namespace.karmada.io/skip-auto-propagation: "true"
	//
	// NOTE: If create a ns without this label, then patch it with this label, the ns will not be
	// synced to new member clusters, but old member clusters still have it.
	NamespaceSkipAutoPropagationLabel = "namespace.karmada.io/skip-auto-propagation"
)
