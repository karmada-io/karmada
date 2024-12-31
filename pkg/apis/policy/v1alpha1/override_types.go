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

package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceKindOverridePolicy is kind name of OverridePolicy.
	ResourceKindOverridePolicy = "OverridePolicy"
	// ResourceSingularOverridePolicy is singular name of OverridePolicy.
	ResourceSingularOverridePolicy = "overridepolicy"
	// ResourcePluralOverridePolicy is plural name of OverridePolicy.
	ResourcePluralOverridePolicy = "overridepolicies"
	// ResourceNamespaceScopedOverridePolicy indicates if OverridePolicy is NamespaceScoped.
	ResourceNamespaceScopedOverridePolicy = true

	// ResourceKindClusterOverridePolicy is kind name of ClusterOverridePolicy.
	ResourceKindClusterOverridePolicy = "ClusterOverridePolicy"
	// ResourceSingularClusterOverridePolicy is singular name of ClusterOverridePolicy.
	ResourceSingularClusterOverridePolicy = "clusteroverridepolicy"
	// ResourcePluralClusterOverridePolicy is kind plural of ClusterOverridePolicy.
	ResourcePluralClusterOverridePolicy = "clusteroverridepolicies"
	// ResourceNamespaceScopedClusterOverridePolicy indicates if ClusterOverridePolicy is NamespaceScoped.
	ResourceNamespaceScopedClusterOverridePolicy = false
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=overridepolicies,scope=Namespaced,shortName=op,categories={karmada-io}

// OverridePolicy represents the policy that overrides a group of resources to one or more clusters.
type OverridePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of OverridePolicy.
	Spec OverrideSpec `json:"spec"`
}

// OverrideSpec defines the desired behavior of OverridePolicy.
type OverrideSpec struct {
	// ResourceSelectors restricts resource types that this override policy applies to.
	// nil means matching all resources.
	// +optional
	ResourceSelectors []ResourceSelector `json:"resourceSelectors,omitempty"`

	// OverrideRules defines a collection of override rules on target clusters.
	// +optional
	OverrideRules []RuleWithCluster `json:"overrideRules,omitempty"`

	// TargetCluster defines restrictions on this override policy
	// that only applies to resources propagated to the matching clusters.
	// nil means matching all clusters.
	//
	// Deprecated: This filed is deprecated in v1.0 and please use the OverrideRules instead.
	// +optional
	TargetCluster *ClusterAffinity `json:"targetCluster,omitempty"`

	// Overriders represents the override rules that would apply on resources
	//
	// Deprecated: This filed is deprecated in v1.0 and please use the OverrideRules instead.
	// +optional
	Overriders Overriders `json:"overriders"`
}

// RuleWithCluster defines the override rules on clusters.
type RuleWithCluster struct {
	// TargetCluster defines restrictions on this override policy
	// that only applies to resources propagated to the matching clusters.
	// nil means matching all clusters.
	// +optional
	TargetCluster *ClusterAffinity `json:"targetCluster,omitempty"`

	// Overriders represents the override rules that would apply on resources
	// +required
	Overriders Overriders `json:"overriders"`
}

// Overriders offers various alternatives to represent the override rules.
//
// If more than one alternative exists, they will be applied with following order:
// - ImageOverrider
// - CommandOverrider
// - ArgsOverrider
// - LabelsOverrider
// - AnnotationsOverrider
// - FieldOverrider
// - Plaintext
type Overriders struct {
	// Plaintext represents override rules defined with plaintext overriders.
	// +optional
	Plaintext []PlaintextOverrider `json:"plaintext,omitempty"`

	// ImageOverrider represents the rules dedicated to handling image overrides.
	// +optional
	ImageOverrider []ImageOverrider `json:"imageOverrider,omitempty"`

	// CommandOverrider represents the rules dedicated to handling container command
	// +optional
	CommandOverrider []CommandArgsOverrider `json:"commandOverrider,omitempty"`

	// ArgsOverrider represents the rules dedicated to handling container args
	// +optional
	ArgsOverrider []CommandArgsOverrider `json:"argsOverrider,omitempty"`

	// LabelsOverrider represents the rules dedicated to handling workload labels
	// +optional
	LabelsOverrider []LabelAnnotationOverrider `json:"labelsOverrider,omitempty"`

	// AnnotationsOverrider represents the rules dedicated to handling workload annotations
	// +optional
	AnnotationsOverrider []LabelAnnotationOverrider `json:"annotationsOverrider,omitempty"`

	// FieldOverrider represents the rules dedicated to modifying a specific field in any Kubernetes resource.
	// This allows changing a single field within the resource with multiple operations.
	// It is designed to handle structured field values such as those found in ConfigMaps or Secrets.
	// The current implementation supports JSON and YAML formats, but can easily be extended to support XML in the future.
	// +optional
	FieldOverrider []FieldOverrider `json:"fieldOverrider,omitempty"`
}

// LabelAnnotationOverrider represents the rules dedicated to handling workload labels/annotations
type LabelAnnotationOverrider struct {
	// Operator represents the operator which will apply on the workload.
	// +kubebuilder:validation:Enum=add;remove;replace
	// +required
	Operator OverriderOperator `json:"operator"`

	// Value to be applied to annotations/labels of workload.
	// Items in Value which will be appended after annotations/labels when Operator is 'add'.
	// Items in Value which match in annotations/labels will be deleted when Operator is 'remove'.
	// Items in Value which match in annotations/labels will be replaced when Operator is 'replace'.
	// +required
	Value map[string]string `json:"value,omitempty"`
}

// ImageOverrider represents the rules dedicated to handling image overrides.
type ImageOverrider struct {
	// Predicate filters images before applying the rule.
	//
	// Defaults to nil, in that case, the system will automatically detect image fields if the resource type is
	// Pod, ReplicaSet, Deployment, StatefulSet, DaemonSet or Job by following rule:
	//   - Pod: /spec/containers/<N>/image
	//   - ReplicaSet: /spec/template/spec/containers/<N>/image
	//   - Deployment: /spec/template/spec/containers/<N>/image
	//   - DaemonSet: /spec/template/spec/containers/<N>/image
	//   - StatefulSet: /spec/template/spec/containers/<N>/image
	//   - Job: /spec/template/spec/containers/<N>/image
	// In addition, all images will be processed if the resource object has more than one container.
	//
	// If not nil, only images matches the filters will be processed.
	// +optional
	Predicate *ImagePredicate `json:"predicate,omitempty"`

	// Component is part of image name.
	// Basically we presume an image can be made of '[registry/]repository[:tag]'.
	// The registry could be:
	// - registry.k8s.io
	// - fictional.registry.example:10443
	// The repository could be:
	// - kube-apiserver
	// - fictional/nginx
	// The tag cloud be:
	// - latest
	// - v1.19.1
	// - @sha256:dbcc1c35ac38df41fd2f5e4130b32ffdb93ebae8b3dbe638c23575912276fc9c
	//
	// +kubebuilder:validation:Enum=Registry;Repository;Tag
	// +required
	Component ImageComponent `json:"component"`

	// Operator represents the operator which will apply on the image.
	// +kubebuilder:validation:Enum=add;remove;replace
	// +required
	Operator OverriderOperator `json:"operator"`

	// Value to be applied to image.
	// Must not be empty when operator is 'add' or 'replace'.
	// Defaults to empty and ignored when operator is 'remove'.
	// +optional
	Value string `json:"value,omitempty"`
}

// ImagePredicate describes images filter.
type ImagePredicate struct {
	// Path indicates the path of target field
	// +required
	Path string `json:"path"`
}

// ImageComponent indicates the components for image.
type ImageComponent string

// CommandArgsOverrider represents the rules dedicated to handling command/args overrides.
type CommandArgsOverrider struct {
	// The name of container
	// +required
	ContainerName string `json:"containerName"`

	// Operator represents the operator which will apply on the command/args.
	// +kubebuilder:validation:Enum=add;remove
	// +required
	Operator OverriderOperator `json:"operator"`

	// Value to be applied to command/args.
	// Items in Value which will be appended after command/args when Operator is 'add'.
	// Items in Value which match in command/args will be deleted when Operator is 'remove'.
	// If Value is empty, then the command/args will remain the same.
	// +optional
	Value []string `json:"value,omitempty"`
}

const (
	// Registry is the registry component of an image with format '[registry/]repository[:tag]'.
	Registry ImageComponent = "Registry"

	// Repository is the repository component of an image with format '[registry/]repository[:tag]'.
	Repository ImageComponent = "Repository"

	// Tag is the tag component of an image with format '[registry/]repository[:tag]'.
	Tag ImageComponent = "Tag"
)

// PlaintextOverrider is a simple overrider that overrides target fields
// according to path, operator and value.
type PlaintextOverrider struct {
	// Path indicates the path of target field
	Path string `json:"path"`
	// Operator indicates the operation on target field.
	// Available operators are: add, replace and remove.
	// +kubebuilder:validation:Enum=add;remove;replace
	Operator OverriderOperator `json:"operator"`
	// Value to be applied to target field.
	// Must be empty when operator is Remove.
	// +optional
	Value apiextensionsv1.JSON `json:"value,omitempty"`
}

// OverriderOperator is the set of operators that can be used in an overrider.
type OverriderOperator string

// These are valid overrider operators.
const (
	OverriderOpAdd     OverriderOperator = "add"
	OverriderOpRemove  OverriderOperator = "remove"
	OverriderOpReplace OverriderOperator = "replace"
)

// FieldOverrider represents the rules dedicated to modifying a specific field in any Kubernetes resource.
// This allows changing a single field within the resource with multiple operations.
// It is designed to handle structured field values such as those found in ConfigMaps or Secrets.
// The current implementation supports JSON and YAML formats, but can easily be extended to support XML in the future.
// Note: In any given instance, FieldOverrider processes either JSON or YAML fields, but not both simultaneously.
type FieldOverrider struct {
	// FieldPath specifies the initial location in the instance document where the operation should take place.
	// The path uses RFC 6901 for navigating into nested structures. For example, the path "/data/db-config.yaml"
	// specifies the configuration data key named "db-config.yaml" in a ConfigMap: "/data/db-config.yaml".
	// +required
	FieldPath string `json:"fieldPath"`

	// JSON represents the operations performed on the JSON document specified by the FieldPath.
	// +optional
	JSON []JSONPatchOperation `json:"json,omitempty"`

	// YAML represents the operations performed on the YAML document specified by the FieldPath.
	// +optional
	YAML []YAMLPatchOperation `json:"yaml,omitempty"`
}

// JSONPatchOperation represents a single field modification operation for JSON format.
type JSONPatchOperation struct {
	// SubPath specifies the relative location within the initial FieldPath where the operation should take place.
	// The path uses RFC 6901 for navigating into nested structures.
	// +required
	SubPath string `json:"subPath"`

	// Operator indicates the operation on target field.
	// Available operators are: "add", "remove", and "replace".
	// +kubebuilder:validation:Enum=add;remove;replace
	// +required
	Operator OverriderOperator `json:"operator"`

	// Value is the new value to set for the specified field if the operation is "add" or "replace".
	// For "remove" operation, this field is ignored.
	// +optional
	Value apiextensionsv1.JSON `json:"value,omitempty"`
}

// YAMLPatchOperation represents a single field modification operation for YAML format.
type YAMLPatchOperation struct {
	// SubPath specifies the relative location within the initial FieldPath where the operation should take place.
	// The path uses RFC 6901 for navigating into nested structures.
	// +required
	SubPath string `json:"subPath"`

	// Operator indicates the operation on target field.
	// Available operators are: "add", "remove", and "replace".
	// +kubebuilder:validation:Enum=add;remove;replace
	// +required
	Operator OverriderOperator `json:"operator"`

	// Value is the new value to set for the specified field if the operation is "add" or "replace".
	// For "remove" operation, this field is ignored.
	// +optional
	Value apiextensionsv1.JSON `json:"value,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OverridePolicyList is a collection of OverridePolicy.
type OverridePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of OverridePolicy.
	Items []OverridePolicy `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:path=clusteroverridepolicies,scope="Cluster",shortName=cop,categories={karmada-io}
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterOverridePolicy represents the cluster-wide policy that overrides a group of resources to one or more clusters.
type ClusterOverridePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of ClusterOverridePolicy.
	Spec OverrideSpec `json:"spec"`
}

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterOverridePolicyList is a collection of ClusterOverridePolicy.
type ClusterOverridePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of ClusterOverridePolicy.
	Items []ClusterOverridePolicy `json:"items"`
}
