/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=clusterclasses,shortName=cc,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of ClusterClass"

// ClusterClass is a template which can be used to create managed topologies.
type ClusterClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterClassSpec   `json:"spec,omitempty"`
	Status ClusterClassStatus `json:"status,omitempty"`
}

// ClusterClassSpec describes the desired state of the ClusterClass.
type ClusterClassSpec struct {
	// Infrastructure is a reference to a provider-specific template that holds
	// the details for provisioning infrastructure specific cluster
	// for the underlying provider.
	// The underlying provider is responsible for the implementation
	// of the template to an infrastructure cluster.
	// +optional
	Infrastructure LocalObjectTemplate `json:"infrastructure,omitempty"`

	// ControlPlane is a reference to a local struct that holds the details
	// for provisioning the Control Plane for the Cluster.
	// +optional
	ControlPlane ControlPlaneClass `json:"controlPlane,omitempty"`

	// Workers describes the worker nodes for the cluster.
	// It is a collection of node types which can be used to create
	// the worker nodes of the cluster.
	// +optional
	Workers WorkersClass `json:"workers,omitempty"`

	// Variables defines the variables which can be configured
	// in the Cluster topology and are then used in patches.
	// +optional
	Variables []ClusterClassVariable `json:"variables,omitempty"`

	// Patches defines the patches which are applied to customize
	// referenced templates of a ClusterClass.
	// Note: Patches will be applied in the order of the array.
	// +optional
	Patches []ClusterClassPatch `json:"patches,omitempty"`
}

// ControlPlaneClass defines the class for the control plane.
type ControlPlaneClass struct {
	// Metadata is the metadata applied to the ControlPlane and the Machines of the ControlPlane
	// if the ControlPlaneTemplate referenced is machine based. If not, it is applied only to the
	// ControlPlane.
	// At runtime this metadata is merged with the corresponding metadata from the topology.
	//
	// This field is supported if and only if the control plane provider template
	// referenced is Machine based.
	// +optional
	Metadata ObjectMeta `json:"metadata,omitempty"`

	// LocalObjectTemplate contains the reference to the control plane provider.
	LocalObjectTemplate `json:",inline"`

	// MachineInfrastructure defines the metadata and infrastructure information
	// for control plane machines.
	//
	// This field is supported if and only if the control plane provider template
	// referenced above is Machine based and supports setting replicas.
	//
	// +optional
	MachineInfrastructure *LocalObjectTemplate `json:"machineInfrastructure,omitempty"`

	// MachineHealthCheck defines a MachineHealthCheck for this ControlPlaneClass.
	// This field is supported if and only if the ControlPlane provider template
	// referenced above is Machine based and supports setting replicas.
	// +optional
	MachineHealthCheck *MachineHealthCheckClass `json:"machineHealthCheck,omitempty"`

	// NodeDrainTimeout is the total amount of time that the controller will spend on draining a node.
	// The default value is 0, meaning that the node can be drained without any time limitations.
	// NOTE: NodeDrainTimeout is different from `kubectl drain --timeout`
	// NOTE: This value can be overridden while defining a Cluster.Topology.
	// +optional
	NodeDrainTimeout *metav1.Duration `json:"nodeDrainTimeout,omitempty"`

	// NodeVolumeDetachTimeout is the total amount of time that the controller will spend on waiting for all volumes
	// to be detached. The default value is 0, meaning that the volumes can be detached without any time limitations.
	// NOTE: This value can be overridden while defining a Cluster.Topology.
	// +optional
	NodeVolumeDetachTimeout *metav1.Duration `json:"nodeVolumeDetachTimeout,omitempty"`

	// NodeDeletionTimeout defines how long the controller will attempt to delete the Node that the Machine
	// hosts after the Machine is marked for deletion. A duration of 0 will retry deletion indefinitely.
	// Defaults to 10 seconds.
	// NOTE: This value can be overridden while defining a Cluster.Topology.
	// +optional
	NodeDeletionTimeout *metav1.Duration `json:"nodeDeletionTimeout,omitempty"`
}

// WorkersClass is a collection of deployment classes.
type WorkersClass struct {
	// MachineDeployments is a list of machine deployment classes that can be used to create
	// a set of worker nodes.
	// +optional
	MachineDeployments []MachineDeploymentClass `json:"machineDeployments,omitempty"`
}

// MachineDeploymentClass serves as a template to define a set of worker nodes of the cluster
// provisioned using the `ClusterClass`.
type MachineDeploymentClass struct {
	// Class denotes a type of worker node present in the cluster,
	// this name MUST be unique within a ClusterClass and can be referenced
	// in the Cluster to create a managed MachineDeployment.
	Class string `json:"class"`

	// Template is a local struct containing a collection of templates for creation of
	// MachineDeployment objects representing a set of worker nodes.
	Template MachineDeploymentClassTemplate `json:"template"`

	// MachineHealthCheck defines a MachineHealthCheck for this MachineDeploymentClass.
	// +optional
	MachineHealthCheck *MachineHealthCheckClass `json:"machineHealthCheck,omitempty"`

	// FailureDomain is the failure domain the machines will be created in.
	// Must match a key in the FailureDomains map stored on the cluster object.
	// NOTE: This value can be overridden while defining a Cluster.Topology using this MachineDeploymentClass.
	// +optional
	FailureDomain *string `json:"failureDomain,omitempty"`

	// NodeDrainTimeout is the total amount of time that the controller will spend on draining a node.
	// The default value is 0, meaning that the node can be drained without any time limitations.
	// NOTE: NodeDrainTimeout is different from `kubectl drain --timeout`
	// NOTE: This value can be overridden while defining a Cluster.Topology using this MachineDeploymentClass.
	// +optional
	NodeDrainTimeout *metav1.Duration `json:"nodeDrainTimeout,omitempty"`

	// NodeVolumeDetachTimeout is the total amount of time that the controller will spend on waiting for all volumes
	// to be detached. The default value is 0, meaning that the volumes can be detached without any time limitations.
	// NOTE: This value can be overridden while defining a Cluster.Topology using this MachineDeploymentClass.
	// +optional
	NodeVolumeDetachTimeout *metav1.Duration `json:"nodeVolumeDetachTimeout,omitempty"`

	// NodeDeletionTimeout defines how long the controller will attempt to delete the Node that the Machine
	// hosts after the Machine is marked for deletion. A duration of 0 will retry deletion indefinitely.
	// Defaults to 10 seconds.
	// NOTE: This value can be overridden while defining a Cluster.Topology using this MachineDeploymentClass.
	// +optional
	NodeDeletionTimeout *metav1.Duration `json:"nodeDeletionTimeout,omitempty"`

	// Minimum number of seconds for which a newly created machine should
	// be ready.
	// Defaults to 0 (machine will be considered available as soon as it
	// is ready)
	// NOTE: This value can be overridden while defining a Cluster.Topology using this MachineDeploymentClass.
	MinReadySeconds *int32 `json:"minReadySeconds,omitempty"`

	// The deployment strategy to use to replace existing machines with
	// new ones.
	// NOTE: This value can be overridden while defining a Cluster.Topology using this MachineDeploymentClass.
	Strategy *MachineDeploymentStrategy `json:"strategy,omitempty"`
}

// MachineDeploymentClassTemplate defines how a MachineDeployment generated from a MachineDeploymentClass
// should look like.
type MachineDeploymentClassTemplate struct {
	// Metadata is the metadata applied to the MachineDeployment and the machines of the MachineDeployment.
	// At runtime this metadata is merged with the corresponding metadata from the topology.
	// +optional
	Metadata ObjectMeta `json:"metadata,omitempty"`

	// Bootstrap contains the bootstrap template reference to be used
	// for the creation of worker Machines.
	Bootstrap LocalObjectTemplate `json:"bootstrap"`

	// Infrastructure contains the infrastructure template reference to be used
	// for the creation of worker Machines.
	Infrastructure LocalObjectTemplate `json:"infrastructure"`
}

// MachineHealthCheckClass defines a MachineHealthCheck for a group of Machines.
type MachineHealthCheckClass struct {
	// UnhealthyConditions contains a list of the conditions that determine
	// whether a node is considered unhealthy. The conditions are combined in a
	// logical OR, i.e. if any of the conditions is met, the node is unhealthy.
	UnhealthyConditions []UnhealthyCondition `json:"unhealthyConditions,omitempty"`

	// Any further remediation is only allowed if at most "MaxUnhealthy" machines selected by
	// "selector" are not healthy.
	// +optional
	MaxUnhealthy *intstr.IntOrString `json:"maxUnhealthy,omitempty"`

	// Any further remediation is only allowed if the number of machines selected by "selector" as not healthy
	// is within the range of "UnhealthyRange". Takes precedence over MaxUnhealthy.
	// Eg. "[3-5]" - This means that remediation will be allowed only when:
	// (a) there are at least 3 unhealthy machines (and)
	// (b) there are at most 5 unhealthy machines
	// +optional
	// +kubebuilder:validation:Pattern=^\[[0-9]+-[0-9]+\]$
	UnhealthyRange *string `json:"unhealthyRange,omitempty"`

	// Machines older than this duration without a node will be considered to have
	// failed and will be remediated.
	// If you wish to disable this feature, set the value explicitly to 0.
	// +optional
	NodeStartupTimeout *metav1.Duration `json:"nodeStartupTimeout,omitempty"`

	// RemediationTemplate is a reference to a remediation template
	// provided by an infrastructure provider.
	//
	// This field is completely optional, when filled, the MachineHealthCheck controller
	// creates a new object from the template referenced and hands off remediation of the machine to
	// a controller that lives outside of Cluster API.
	// +optional
	RemediationTemplate *corev1.ObjectReference `json:"remediationTemplate,omitempty"`
}

// IsZero returns true if none of the values of MachineHealthCheckClass are defined.
func (m MachineHealthCheckClass) IsZero() bool {
	return reflect.ValueOf(m).IsZero()
}

// ClusterClassVariable defines a variable which can
// be configured in the Cluster topology and used in patches.
type ClusterClassVariable struct {
	// Name of the variable.
	Name string `json:"name"`

	// Required specifies if the variable is required.
	// Note: this applies to the variable as a whole and thus the
	// top-level object defined in the schema. If nested fields are
	// required, this will be specified inside the schema.
	Required bool `json:"required"`

	// Schema defines the schema of the variable.
	Schema VariableSchema `json:"schema"`
}

// VariableSchema defines the schema of a variable.
type VariableSchema struct {
	// OpenAPIV3Schema defines the schema of a variable via OpenAPI v3
	// schema. The schema is a subset of the schema used in
	// Kubernetes CRDs.
	OpenAPIV3Schema JSONSchemaProps `json:"openAPIV3Schema"`
}

// JSONSchemaProps is a JSON-Schema following Specification Draft 4 (http://json-schema.org/).
// This struct has been initially copied from apiextensionsv1.JSONSchemaProps, but all fields
// which are not supported in CAPI have been removed.
type JSONSchemaProps struct {
	// Description is a human-readable description of this variable.
	Description string `json:"description,omitempty"`

	// Example is an example for this variable.
	Example *apiextensionsv1.JSON `json:"example,omitempty"`

	// Type is the type of the variable.
	// Valid values are: object, array, string, integer, number or boolean.
	Type string `json:"type"`

	// Properties specifies fields of an object.
	// NOTE: Can only be set if type is object.
	// NOTE: Properties is mutually exclusive with AdditionalProperties.
	// NOTE: This field uses PreserveUnknownFields and Schemaless,
	// because recursive validation is not possible.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Properties map[string]JSONSchemaProps `json:"properties,omitempty"`

	// AdditionalProperties specifies the schema of values in a map (keys are always strings).
	// NOTE: Can only be set if type is object.
	// NOTE: AdditionalProperties is mutually exclusive with Properties.
	// NOTE: This field uses PreserveUnknownFields and Schemaless,
	// because recursive validation is not possible.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	AdditionalProperties *JSONSchemaProps `json:"additionalProperties,omitempty"`

	// Required specifies which fields of an object are required.
	// NOTE: Can only be set if type is object.
	// +optional
	Required []string `json:"required,omitempty"`

	// Items specifies fields of an array.
	// NOTE: Can only be set if type is array.
	// NOTE: This field uses PreserveUnknownFields and Schemaless,
	// because recursive validation is not possible.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Items *JSONSchemaProps `json:"items,omitempty"`

	// MaxItems is the max length of an array variable.
	// NOTE: Can only be set if type is array.
	// +optional
	MaxItems *int64 `json:"maxItems,omitempty"`

	// MinItems is the min length of an array variable.
	// NOTE: Can only be set if type is array.
	// +optional
	MinItems *int64 `json:"minItems,omitempty"`

	// UniqueItems specifies if items in an array must be unique.
	// NOTE: Can only be set if type is array.
	// +optional
	UniqueItems bool `json:"uniqueItems,omitempty"`

	// Format is an OpenAPI v3 format string. Unknown formats are ignored.
	// For a list of supported formats please see: (of the k8s.io/apiextensions-apiserver version we're currently using)
	// https://github.com/kubernetes/apiextensions-apiserver/blob/master/pkg/apiserver/validation/formats.go
	// NOTE: Can only be set if type is string.
	// +optional
	Format string `json:"format,omitempty"`

	// MaxLength is the max length of a string variable.
	// NOTE: Can only be set if type is string.
	// +optional
	MaxLength *int64 `json:"maxLength,omitempty"`

	// MinLength is the min length of a string variable.
	// NOTE: Can only be set if type is string.
	// +optional
	MinLength *int64 `json:"minLength,omitempty"`

	// Pattern is the regex which a string variable must match.
	// NOTE: Can only be set if type is string.
	// +optional
	Pattern string `json:"pattern,omitempty"`

	// Maximum is the maximum of an integer or number variable.
	// If ExclusiveMaximum is false, the variable is valid if it is lower than, or equal to, the value of Maximum.
	// If ExclusiveMaximum is true, the variable is valid if it is strictly lower than the value of Maximum.
	// NOTE: Can only be set if type is integer or number.
	// +optional
	Maximum *int64 `json:"maximum,omitempty"`

	// ExclusiveMaximum specifies if the Maximum is exclusive.
	// NOTE: Can only be set if type is integer or number.
	// +optional
	ExclusiveMaximum bool `json:"exclusiveMaximum,omitempty"`

	// Minimum is the minimum of an integer or number variable.
	// If ExclusiveMinimum is false, the variable is valid if it is greater than, or equal to, the value of Minimum.
	// If ExclusiveMinimum is true, the variable is valid if it is strictly greater than the value of Minimum.
	// NOTE: Can only be set if type is integer or number.
	// +optional
	Minimum *int64 `json:"minimum,omitempty"`

	// ExclusiveMinimum specifies if the Minimum is exclusive.
	// NOTE: Can only be set if type is integer or number.
	// +optional
	ExclusiveMinimum bool `json:"exclusiveMinimum,omitempty"`

	// XPreserveUnknownFields allows setting fields in a variable object
	// which are not defined in the variable schema. This affects fields recursively,
	// except if nested properties or additionalProperties are specified in the schema.
	// +optional
	XPreserveUnknownFields bool `json:"x-kubernetes-preserve-unknown-fields,omitempty"`

	// Enum is the list of valid values of the variable.
	// NOTE: Can be set for all types.
	// +optional
	Enum []apiextensionsv1.JSON `json:"enum,omitempty"`

	// Default is the default value of the variable.
	// NOTE: Can be set for all types.
	// +optional
	Default *apiextensionsv1.JSON `json:"default,omitempty"`
}

// ClusterClassPatch defines a patch which is applied to customize the referenced templates.
type ClusterClassPatch struct {
	// Name of the patch.
	Name string `json:"name"`

	// Description is a human-readable description of this patch.
	Description string `json:"description,omitempty"`

	// EnabledIf is a Go template to be used to calculate if a patch should be enabled.
	// It can reference variables defined in .spec.variables and builtin variables.
	// The patch will be enabled if the template evaluates to `true`, otherwise it will
	// be disabled.
	// If EnabledIf is not set, the patch will be enabled per default.
	// +optional
	EnabledIf *string `json:"enabledIf,omitempty"`

	// Definitions define inline patches.
	// Note: Patches will be applied in the order of the array.
	// Note: Exactly one of Definitions or External must be set.
	// +optional
	Definitions []PatchDefinition `json:"definitions,omitempty"`

	// External defines an external patch.
	// Note: Exactly one of Definitions or External must be set.
	// +optional
	External *ExternalPatchDefinition `json:"external,omitempty"`
}

// PatchDefinition defines a patch which is applied to customize the referenced templates.
type PatchDefinition struct {
	// Selector defines on which templates the patch should be applied.
	Selector PatchSelector `json:"selector"`

	// JSONPatches defines the patches which should be applied on the templates
	// matching the selector.
	// Note: Patches will be applied in the order of the array.
	JSONPatches []JSONPatch `json:"jsonPatches"`
}

// PatchSelector defines on which templates the patch should be applied.
// Note: Matching on APIVersion and Kind is mandatory, to enforce that the patches are
// written for the correct version. The version of the references in the ClusterClass may
// be automatically updated during reconciliation if there is a newer version for the same contract.
// Note: The results of selection based on the individual fields are ANDed.
type PatchSelector struct {
	// APIVersion filters templates by apiVersion.
	APIVersion string `json:"apiVersion"`

	// Kind filters templates by kind.
	Kind string `json:"kind"`

	// MatchResources selects templates based on where they are referenced.
	MatchResources PatchSelectorMatch `json:"matchResources"`
}

// PatchSelectorMatch selects templates based on where they are referenced.
// Note: The selector must match at least one template.
// Note: The results of selection based on the individual fields are ORed.
type PatchSelectorMatch struct {
	// ControlPlane selects templates referenced in .spec.ControlPlane.
	// Note: this will match the controlPlane and also the controlPlane
	// machineInfrastructure (depending on the kind and apiVersion).
	// +optional
	ControlPlane bool `json:"controlPlane,omitempty"`

	// InfrastructureCluster selects templates referenced in .spec.infrastructure.
	// +optional
	InfrastructureCluster bool `json:"infrastructureCluster,omitempty"`

	// MachineDeploymentClass selects templates referenced in specific MachineDeploymentClasses in
	// .spec.workers.machineDeployments.
	// +optional
	MachineDeploymentClass *PatchSelectorMatchMachineDeploymentClass `json:"machineDeploymentClass,omitempty"`
}

// PatchSelectorMatchMachineDeploymentClass selects templates referenced
// in specific MachineDeploymentClasses in .spec.workers.machineDeployments.
type PatchSelectorMatchMachineDeploymentClass struct {
	// Names selects templates by class names.
	// +optional
	Names []string `json:"names,omitempty"`
}

// JSONPatch defines a JSON patch.
type JSONPatch struct {
	// Op defines the operation of the patch.
	// Note: Only `add`, `replace` and `remove` are supported.
	Op string `json:"op"`

	// Path defines the path of the patch.
	// Note: Only the spec of a template can be patched, thus the path has to start with /spec/.
	// Note: For now the only allowed array modifications are `append` and `prepend`, i.e.:
	// * for op: `add`: only index 0 (prepend) and - (append) are allowed
	// * for op: `replace` or `remove`: no indexes are allowed
	Path string `json:"path"`

	// Value defines the value of the patch.
	// Note: Either Value or ValueFrom is required for add and replace
	// operations. Only one of them is allowed to be set at the same time.
	// Note: We have to use apiextensionsv1.JSON instead of our JSON type,
	// because controller-tools has a hard-coded schema for apiextensionsv1.JSON
	// which cannot be produced by another type (unset type field).
	// Ref: https://github.com/kubernetes-sigs/controller-tools/blob/d0e03a142d0ecdd5491593e941ee1d6b5d91dba6/pkg/crd/known_types.go#L106-L111
	// +optional
	Value *apiextensionsv1.JSON `json:"value,omitempty"`

	// ValueFrom defines the value of the patch.
	// Note: Either Value or ValueFrom is required for add and replace
	// operations. Only one of them is allowed to be set at the same time.
	// +optional
	ValueFrom *JSONPatchValue `json:"valueFrom,omitempty"`
}

// JSONPatchValue defines the value of a patch.
// Note: Only one of the fields is allowed to be set at the same time.
type JSONPatchValue struct {
	// Variable is the variable to be used as value.
	// Variable can be one of the variables defined in .spec.variables or a builtin variable.
	// +optional
	Variable *string `json:"variable,omitempty"`

	// Template is the Go template to be used to calculate the value.
	// A template can reference variables defined in .spec.variables and builtin variables.
	// Note: The template must evaluate to a valid YAML or JSON value.
	// +optional
	Template *string `json:"template,omitempty"`
}

// ExternalPatchDefinition defines an external patch.
// Note: At least one of GenerateExtension or ValidateExtension must be set.
type ExternalPatchDefinition struct {
	// GenerateExtension references an extension which is called to generate patches.
	// +optional
	GenerateExtension *string `json:"generateExtension,omitempty"`

	// ValidateExtension references an extension which is called to validate the topology.
	// +optional
	ValidateExtension *string `json:"validateExtension,omitempty"`

	// DiscoverVariablesExtension references an extension which is called to discover variables.
	// +optional
	DiscoverVariablesExtension *string `json:"discoverVariablesExtension,omitempty"`

	// Settings defines key value pairs to be passed to the extensions.
	// Values defined here take precedence over the values defined in the
	// corresponding ExtensionConfig.
	// +optional
	Settings map[string]string `json:"settings,omitempty"`
}

// LocalObjectTemplate defines a template for a topology Class.
type LocalObjectTemplate struct {
	// Ref is a required reference to a custom resource
	// offered by a provider.
	Ref *corev1.ObjectReference `json:"ref"`
}

// ANCHOR: ClusterClassStatus

// ClusterClassStatus defines the observed state of the ClusterClass.
type ClusterClassStatus struct {
	// Variables is a list of ClusterClassStatusVariable that are defined for the ClusterClass.
	// +optional
	Variables []ClusterClassStatusVariable `json:"variables,omitempty"`

	// Conditions defines current observed state of the ClusterClass.
	// +optional
	Conditions Conditions `json:"conditions,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ClusterClassStatusVariable defines a variable which appears in the status of a ClusterClass.
type ClusterClassStatusVariable struct {
	// Name is the name of the variable.
	Name string `json:"name"`

	// DefinitionsConflict specifies whether or not there are conflicting definitions for a single variable name.
	// +optional
	DefinitionsConflict bool `json:"definitionsConflict"`

	// Definitions is a list of definitions for a variable.
	Definitions []ClusterClassStatusVariableDefinition `json:"definitions"`
}

// ClusterClassStatusVariableDefinition defines a variable which appears in the status of a ClusterClass.
type ClusterClassStatusVariableDefinition struct {
	// From specifies the origin of the variable definition.
	// This will be `inline` for variables defined in the ClusterClass or the name of a patch defined in the ClusterClass
	// for variables discovered from a DiscoverVariables runtime extensions.
	From string `json:"from"`

	// Required specifies if the variable is required.
	// Note: this applies to the variable as a whole and thus the
	// top-level object defined in the schema. If nested fields are
	// required, this will be specified inside the schema.
	Required bool `json:"required"`

	// Schema defines the schema of the variable.
	Schema VariableSchema `json:"schema"`
}

// GetConditions returns the set of conditions for this object.
func (c *ClusterClass) GetConditions() Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *ClusterClass) SetConditions(conditions Conditions) {
	c.Status.Conditions = conditions
}

// ANCHOR_END: ClusterClassStatus

// +kubebuilder:object:root=true

// ClusterClassList contains a list of Cluster.
type ClusterClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterClass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterClass{}, &ClusterClassList{})
}
