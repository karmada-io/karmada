package v1alpha2

const (
	// ResourceBindingReferenceKey is the key of ResourceBinding object.
	// It is usually a unique hash value of ResourceBinding object's namespace and name, intended to be added to the Work object.
	// It will be used to retrieve all Works objects that derived from a specific ResourceBinding object.
	ResourceBindingReferenceKey = "resourcebinding.karmada.io/key"

	// ClusterResourceBindingReferenceKey is the key of ClusterResourceBinding object.
	// It is usually a unique hash value of ClusterResourceBinding object's namespace and name, intended to be added to the Work object.
	// It will be used to retrieve all Works objects that derived by a specific ClusterResourceBinding object.
	ClusterResourceBindingReferenceKey = "clusterresourcebinding.karmada.io/key"

	// ResourceBindingNamespaceAnnotationKey is added to object to describe the associated ResourceBinding's namespace.
	// It is added to:
	// - Work object: describes the namespace of ResourceBinding which the Work derived from.
	// - Manifest in Work object: describes the namespace of ResourceBinding which the manifest derived from.
	ResourceBindingNamespaceAnnotationKey = "resourcebinding.karmada.io/namespace"

	// ResourceBindingNameAnnotationKey is added to object to describe the associated ResourceBinding's name.
	// It is added to:
	// - Work object: describes the name of ResourceBinding which the Work derived from.
	// - Manifest in Work object: describes the name of ResourceBinding which the manifest derived from.
	ResourceBindingNameAnnotationKey = "resourcebinding.karmada.io/name"

	// ClusterResourceBindingAnnotationKey is added to object to describe associated ClusterResourceBinding's name.
	// It is added to:
	// - Work object: describes the name of ClusterResourceBinding which the Work derived from.
	// - Manifest in Work object: describes the name of ClusterResourceBinding which the manifest derived from.
	ClusterResourceBindingAnnotationKey = "clusterresourcebinding.karmada.io/name"

	// WorkNamespaceLabel is added to objects to specify associated Work's namespace.
	WorkNamespaceLabel = "work.karmada.io/namespace"

	// WorkNameLabel is added to objects to specify associated Work's name.
	WorkNameLabel = "work.karmada.io/name"
)

// Define resource conflict resolution
const (
	// ResourceConflictResolutionAnnotation is added to the resource template to specify how to resolve the conflict
	// in case of resource already existing in member clusters.
	// The valid value is:
	//   - overwrite: always overwrite the resource if already exist. The resource will be overwritten with the
	//     configuration from control plane.
	// Note: Propagation of the resource template without this annotation will fail in case of already exists.
	ResourceConflictResolutionAnnotation = "work.karmada.io/conflict-resolution"

	// ResourceConflictResolutionOverwrite is the value of ResourceConflictResolutionAnnotation, indicating the overwrite strategy.
	ResourceConflictResolutionOverwrite = "overwrite"
)

// Define annotations that are added to the resource template.
const (
	// ResourceTemplateUIDAnnotation is the annotation that is added to the manifest in the Work object.
	// The annotation is used to identify the resource template which the manifest is derived from.
	// The annotation can also be used to fire events when syncing Work to member clusters.
	// For more details about UID, please refer to:
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids
	ResourceTemplateUIDAnnotation = "resourcetemplate.karmada.io/uid"
)
