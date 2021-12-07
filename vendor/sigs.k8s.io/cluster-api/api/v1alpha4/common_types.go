/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha4

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ClusterLabelName is the label set on machines linked to a cluster and
	// external objects(bootstrap and infrastructure providers).
	ClusterLabelName = "cluster.x-k8s.io/cluster-name"

	// ClusterTopologyLabelName is the label set on all the object which are managed as part of a ClusterTopology.
	// Deprecated: use ClusterTopologyOwnedLabel instead.
	ClusterTopologyLabelName = "cluster.x-k8s.io/topology"

	// ClusterTopologyOwnedLabel is the label set on all the object which are managed as part of a ClusterTopology.
	ClusterTopologyOwnedLabel = "topology.cluster.x-k8s.io/owned"

	// ClusterTopologyMachineDeploymentLabelName is the label set on the generated  MachineDeployment objects
	// to track the name of the MachineDeployment topology it represents.
	ClusterTopologyMachineDeploymentLabelName = "topology.cluster.x-k8s.io/deployment-name"

	// ProviderLabelName is the label set on components in the provider manifest.
	// This label allows to easily identify all the components belonging to a provider; the clusterctl
	// tool uses this label for implementing provider's lifecycle operations.
	ProviderLabelName = "cluster.x-k8s.io/provider"

	// ClusterNameAnnotation is the annotation set on nodes identifying the name of the cluster the node belongs to.
	ClusterNameAnnotation = "cluster.x-k8s.io/cluster-name"

	// ClusterNamespaceAnnotation is the annotation set on nodes identifying the namespace of the cluster the node belongs to.
	ClusterNamespaceAnnotation = "cluster.x-k8s.io/cluster-namespace"

	// MachineAnnotation is the annotation set on nodes identifying the machine the node belongs to.
	MachineAnnotation = "cluster.x-k8s.io/machine"

	// OwnerKindAnnotation is the annotation set on nodes identifying the owner kind.
	OwnerKindAnnotation = "cluster.x-k8s.io/owner-kind"

	// OwnerNameAnnotation is the annotation set on nodes identifying the owner name.
	OwnerNameAnnotation = "cluster.x-k8s.io/owner-name"

	// PausedAnnotation is an annotation that can be applied to any Cluster API
	// object to prevent a controller from processing a resource.
	//
	// Controllers working with Cluster API objects must check the existence of this annotation
	// on the reconciled object.
	PausedAnnotation = "cluster.x-k8s.io/paused"

	// DisableMachineCreate is an annotation that can be used to signal a MachineSet to stop creating new machines.
	// It is utilized in the OnDelete MachineDeploymentStrategy to allow the MachineDeployment controller to scale down
	// older MachineSets when Machines are deleted and add the new replicas to the latest MachineSet.
	DisableMachineCreate = "cluster.x-k8s.io/disable-machine-create"

	// WatchLabel is a label othat can be applied to any Cluster API object.
	//
	// Controllers which allow for selective reconciliation may check this label and proceed
	// with reconciliation of the object only if this label and a configured value is present.
	WatchLabel = "cluster.x-k8s.io/watch-filter"

	// DeleteMachineAnnotation marks control plane and worker nodes that will be given priority for deletion
	// when KCP or a machineset scales down. This annotation is given top priority on all delete policies.
	DeleteMachineAnnotation = "cluster.x-k8s.io/delete-machine"

	// TemplateClonedFromNameAnnotation is the infrastructure machine annotation that stores the name of the infrastructure template resource
	// that was cloned for the machine. This annotation is set only during cloning a template. Older/adopted machines will not have this annotation.
	TemplateClonedFromNameAnnotation = "cluster.x-k8s.io/cloned-from-name"

	// TemplateClonedFromGroupKindAnnotation is the infrastructure machine annotation that stores the group-kind of the infrastructure template resource
	// that was cloned for the machine. This annotation is set only during cloning a template. Older/adopted machines will not have this annotation.
	TemplateClonedFromGroupKindAnnotation = "cluster.x-k8s.io/cloned-from-groupkind"

	// MachineSkipRemediationAnnotation is the annotation used to mark the machines that should not be considered for remediation by MachineHealthCheck reconciler.
	MachineSkipRemediationAnnotation = "cluster.x-k8s.io/skip-remediation"

	// ClusterSecretType defines the type of secret created by core components.
	ClusterSecretType corev1.SecretType = "cluster.x-k8s.io/secret" //nolint:gosec

	// InterruptibleLabel is the label used to mark the nodes that run on interruptible instances.
	InterruptibleLabel = "cluster.x-k8s.io/interruptible"

	// ManagedByAnnotation is an annotation that can be applied to InfraCluster resources to signify that
	// some external system is managing the cluster infrastructure.
	//
	// Provider InfraCluster controllers will ignore resources with this annotation.
	// An external controller must fulfill the contract of the InfraCluster resource.
	// External infrastructure providers should ensure that the annotation, once set, cannot be removed.
	ManagedByAnnotation = "cluster.x-k8s.io/managed-by"
)

const (
	// TemplateSuffix is the object kind suffix used by template types.
	TemplateSuffix = "Template"
)

var (
	// ZeroDuration is a zero value of the metav1.Duration type.
	ZeroDuration = metav1.Duration{}
)

const (
	// MachineNodeNameIndex is used by the Machine Controller to index Machines by Node name, and add a watch on Nodes.
	// Deprecated: Use api/v1alpha4/index.MachineNodeNameField instead.
	MachineNodeNameIndex = "status.nodeRef.name"

	// MachineProviderIDIndex is used to index Machines by ProviderID. It's useful to find Machines
	// in a management cluster from Nodes in a workload cluster.
	// Deprecated: Use api/v1alpha4/index.MachineProviderIDField instead.
	MachineProviderIDIndex = "spec.providerID"
)

// MachineAddressType describes a valid MachineAddress type.
type MachineAddressType string

// Define the MachineAddressType constants.
const (
	MachineHostName    MachineAddressType = "Hostname"
	MachineExternalIP  MachineAddressType = "ExternalIP"
	MachineInternalIP  MachineAddressType = "InternalIP"
	MachineExternalDNS MachineAddressType = "ExternalDNS"
	MachineInternalDNS MachineAddressType = "InternalDNS"
)

// MachineAddress contains information for the node's address.
type MachineAddress struct {
	// Machine address type, one of Hostname, ExternalIP or InternalIP.
	Type MachineAddressType `json:"type"`

	// The machine address.
	Address string `json:"address"`
}

// MachineAddresses is a slice of MachineAddress items to be used by infrastructure providers.
type MachineAddresses []MachineAddress

// ObjectMeta is metadata that all persisted resources must have, which includes all objects
// users must create. This is a copy of customizable fields from metav1.ObjectMeta.
//
// ObjectMeta is embedded in `Machine.Spec`, `MachineDeployment.Template` and `MachineSet.Template`,
// which are not top-level Kubernetes objects. Given that metav1.ObjectMeta has lots of special cases
// and read-only fields which end up in the generated CRD validation, having it as a subset simplifies
// the API and some issues that can impact user experience.
//
// During the [upgrade to controller-tools@v2](https://github.com/kubernetes-sigs/cluster-api/pull/1054)
// for v1alpha2, we noticed a failure would occur running Cluster API test suite against the new CRDs,
// specifically `spec.metadata.creationTimestamp in body must be of type string: "null"`.
// The investigation showed that `controller-tools@v2` behaves differently than its previous version
// when handling types from [metav1](k8s.io/apimachinery/pkg/apis/meta/v1) package.
//
// In more details, we found that embedded (non-top level) types that embedded `metav1.ObjectMeta`
// had validation properties, including for `creationTimestamp` (metav1.Time).
// The `metav1.Time` type specifies a custom json marshaller that, when IsZero() is true, returns `null`
// which breaks validation because the field isn't marked as nullable.
//
// In future versions, controller-tools@v2 might allow overriding the type and validation for embedded
// types. When that happens, this hack should be revisited.
type ObjectMeta struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}
