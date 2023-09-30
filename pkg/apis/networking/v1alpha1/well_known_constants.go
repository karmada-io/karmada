package v1alpha1

const (
	// ResourceKindMultiClusterIngress is kind name of MultiClusterIngress.
	ResourceKindMultiClusterIngress = "MultiClusterIngress"
	// ResourceKindMultiMultiClusterService is kind name of MultiClusterService.
	ResourceKindMultiMultiClusterService = "MultiClusterService"
	// ResourceSingularMultiClusterIngress is singular name of MultiClusterIngress.
	ResourceSingularMultiClusterIngress = "multiclusteringress"
	// ResourcePluralMultiClusterIngress is plural name of MultiClusterIngress.
	ResourcePluralMultiClusterIngress = "multiclusteringresses"
	// ResourceNamespaceScopedMultiClusterIngress indicates if MultiClusterIngress is NamespaceScoped.
	ResourceNamespaceScopedMultiClusterIngress = true
	// ResourceKindMultiMultiClusterServiceUsedBy indicates which type of MultiClusterService is currently using the resource.
	ResourceKindMultiMultiClusterServiceUsedBy = "multiclusterservices.networking.karmada.io/used-by"
)
