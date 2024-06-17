/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha2

const (
	// ResourceBindingPermanentIDLabel is the identifier of a ResourceBinding object.
	// Karmada generates a unique identifier, such as metadata.UUID, for each ResourceBinding object.
	// This identifier will be used as a label selector to locate corresponding resources, such as Work.
	// The reason for generating a new unique identifier instead of simply using metadata.UUID is because:
	// In backup scenarios, when applying the backup resource manifest in a new cluster, the UUID may change.
	ResourceBindingPermanentIDLabel = "resourcebinding.karmada.io/permanent-id"

	// ClusterResourceBindingPermanentIDLabel is the identifier of a ClusterResourceBinding object.
	// Karmada generates a unique identifier, such as metadata.UUID, for each ClusterResourceBinding object.
	// This identifier will be used as a label selector to locate corresponding resources, such as Work.
	// The reason for generating a new unique identifier instead of simply using metadata.UUID is because:
	// In backup scenarios, when applying the backup resource manifest in a new cluster, the UUID may change.
	ClusterResourceBindingPermanentIDLabel = "clusterresourcebinding.karmada.io/permanent-id"

	// WorkPermanentIDLabel is the ID of Work object.
	WorkPermanentIDLabel = "work.karmada.io/permanent-id"

	// WorkNamespaceAnnotation is added to objects to specify associated Work's namespace.
	WorkNamespaceAnnotation = "work.karmada.io/namespace"

	// WorkNameAnnotation is added to objects to specify associated Work's name.
	WorkNameAnnotation = "work.karmada.io/name"

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

	// BindingManagedByLabel is added to ResourceBinding to represent what kind of resource manages this Binding.
	BindingManagedByLabel = "binding.karmada.io/managed-by"

	// ResourceTemplateGenerationAnnotationKey records the generation of resource template in Karmada APIServer,
	// It will be injected into the resource when propagating to member clusters, to denote the specific version of
	// the resource template from which the resource is derived. It might be helpful in the following cases:
	// 1. Facilitating observation from member clusters to ascertain if the most recent resource template has been
	//    completely synced.
	// 2. The annotation will be synced back to Karmada during the process of syncing resource status,
	//    by leveraging this annotation, Karmada can infer if the most recent resource template has been completely
	//    synced on member clusters, then generates accurate observed generation(like Deployment's .status.observedGeneration)
	//    which might be required by the release system.
	ResourceTemplateGenerationAnnotationKey = "resourcetemplate.karmada.io/generation"
)

// Define resource conflict resolution
const (
	// ResourceConflictResolutionAnnotation is added to the resource template to specify how to resolve the conflict
	// in case of resource already existing in member clusters.
	// The valid value is:
	//   - overwrite: always overwrite the resource if already exist. The resource will be overwritten with the
	//     configuration from control plane.
	//   - abort: do not resolve the conflict and stop propagating to avoid unexpected overwrites (default value)
	// Note: Propagation of the resource template without this annotation will fail in case of already exists.
	ResourceConflictResolutionAnnotation = "work.karmada.io/conflict-resolution"

	// ResourceConflictResolutionOverwrite is a value of ResourceConflictResolutionAnnotation, indicating the overwrite strategy.
	ResourceConflictResolutionOverwrite = "overwrite"

	// ResourceConflictResolutionAbort is a value of ResourceConflictResolutionAnnotation, indicating stop propagating.
	ResourceConflictResolutionAbort = "abort"
)

// Define annotations that are added to the resource template.
const (
	// ResourceTemplateUIDAnnotation is the annotation that is added to the manifest in the Work object.
	// The annotation is used to identify the resource template which the manifest is derived from.
	// The annotation can also be used to fire events when syncing Work to member clusters.
	// For more details about UID, please refer to:
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids
	ResourceTemplateUIDAnnotation = "resourcetemplate.karmada.io/uid"
	// ManagedLabels is the annotation that is added to the manifest in the Work object.
	// It is used to identify the label keys that are present in the resource template.
	// With this annotation, Karmada is able to accurately synchronize label changes
	// against resource template and avoid the problem of accidentally retaining
	// the deleted labels.
	// E.g. "resourcetemplate.karmada.io/managed-labels: bar,foo".
	// Note: the keys will be sorted in alphabetical order.
	ManagedLabels = "resourcetemplate.karmada.io/managed-labels"

	// ManagedAnnotation is the annotation that is added to the manifest in the Work object.
	// It is used to identify the annotation keys that are present in the resource template.
	// With this annotation, Karmada is able to accurately synchronize annotation changes
	// against resource template and avoid the problem of accidentally retaining
	// the deleted annotations.
	// E.g. "resourcetemplate.karmada.io/managed-annotations: bar,foo".
	// Note: the keys will be sorted in alphabetical order.
	ManagedAnnotation = "resourcetemplate.karmada.io/managed-annotations"

	// DeletionProtectionLabelKey If a user assigns the DeletionProtectionLabelKey label to a specific resource,
	// and the value of this label is DeletionProtectionAlways, then deletion requests
	// for this resource will be denied.
	// In the current design, only the Value set to 'Always' will be protected,
	// Additional options will be added here in the future.
	DeletionProtectionLabelKey = "resourcetemplate.karmada.io/deletion-protected"
	DeletionProtectionAlways   = "Always"
)

// Define eviction reasons.
const (
	// EvictionReasonTaintUntolerated describes the eviction is triggered
	// because can not tolerate taint or exceed toleration period of time.
	EvictionReasonTaintUntolerated = "TaintUntolerated"

	// EvictionReasonApplicationFailure describes the eviction is triggered
	// because the application fails and reaches the condition of ApplicationFailoverBehavior.
	EvictionReasonApplicationFailure = "ApplicationFailure"
)

// Define eviction producers.
const (
	// EvictionProducerTaintManager represents the name of taint manager.
	EvictionProducerTaintManager = "TaintManager"
)
