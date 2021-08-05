package v1alpha1

const (
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
