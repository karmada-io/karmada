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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	// MachineFinalizer is set on PrepareForCreate callback.
	MachineFinalizer = "machine.cluster.x-k8s.io"

	// MachineControlPlaneLabelName is the label set on machines or related objects that are part of a control plane.
	MachineControlPlaneLabelName = "cluster.x-k8s.io/control-plane"

	// ExcludeNodeDrainingAnnotation annotation explicitly skips node draining if set.
	ExcludeNodeDrainingAnnotation = "machine.cluster.x-k8s.io/exclude-node-draining"

	// MachineSetLabelName is the label set on machines if they're controlled by MachineSet.
	MachineSetLabelName = "cluster.x-k8s.io/set-name"

	// MachineDeploymentLabelName is the label set on machines if they're controlled by MachineDeployment.
	MachineDeploymentLabelName = "cluster.x-k8s.io/deployment-name"

	// PreDrainDeleteHookAnnotationPrefix annotation specifies the prefix we
	// search each annotation for during the pre-drain.delete lifecycle hook
	// to pause reconciliation of deletion. These hooks will prevent removal of
	// draining the associated node until all are removed.
	PreDrainDeleteHookAnnotationPrefix = "pre-drain.delete.hook.machine.cluster.x-k8s.io"

	// PreTerminateDeleteHookAnnotationPrefix annotation specifies the prefix we
	// search each annotation for during the pre-terminate.delete lifecycle hook
	// to pause reconciliation of deletion. These hooks will prevent removal of
	// an instance from an infrastructure provider until all are removed.
	PreTerminateDeleteHookAnnotationPrefix = "pre-terminate.delete.hook.machine.cluster.x-k8s.io"
)

// ANCHOR: MachineSpec

// MachineSpec defines the desired state of Machine.
type MachineSpec struct {
	// ClusterName is the name of the Cluster this object belongs to.
	// +kubebuilder:validation:MinLength=1
	ClusterName string `json:"clusterName"`

	// Bootstrap is a reference to a local struct which encapsulates
	// fields to configure the Machine’s bootstrapping mechanism.
	Bootstrap Bootstrap `json:"bootstrap"`

	// InfrastructureRef is a required reference to a custom resource
	// offered by an infrastructure provider.
	InfrastructureRef corev1.ObjectReference `json:"infrastructureRef"`

	// Version defines the desired Kubernetes version.
	// This field is meant to be optionally used by bootstrap providers.
	// +optional
	Version *string `json:"version,omitempty"`

	// ProviderID is the identification ID of the machine provided by the provider.
	// This field must match the provider ID as seen on the node object corresponding to this machine.
	// This field is required by higher level consumers of cluster-api. Example use case is cluster autoscaler
	// with cluster-api as provider. Clean-up logic in the autoscaler compares machines to nodes to find out
	// machines at provider which could not get registered as Kubernetes nodes. With cluster-api as a
	// generic out-of-tree provider for autoscaler, this field is required by autoscaler to be
	// able to have a provider view of the list of machines. Another list of nodes is queried from the k8s apiserver
	// and then a comparison is done to find out unregistered machines and are marked for delete.
	// This field will be set by the actuators and consumed by higher level entities like autoscaler that will
	// be interfacing with cluster-api as generic provider.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// FailureDomain is the failure domain the machine will be created in.
	// Must match a key in the FailureDomains map stored on the cluster object.
	// +optional
	FailureDomain *string `json:"failureDomain,omitempty"`

	// NodeDrainTimeout is the total amount of time that the controller will spend on draining a node.
	// The default value is 0, meaning that the node can be drained without any time limitations.
	// NOTE: NodeDrainTimeout is different from `kubectl drain --timeout`
	// +optional
	NodeDrainTimeout *metav1.Duration `json:"nodeDrainTimeout,omitempty"`
}

// ANCHOR_END: MachineSpec

// ANCHOR: MachineStatus

// MachineStatus defines the observed state of Machine.
type MachineStatus struct {
	// NodeRef will point to the corresponding Node if it exists.
	// +optional
	NodeRef *corev1.ObjectReference `json:"nodeRef,omitempty"`

	// NodeInfo is a set of ids/uuids to uniquely identify the node.
	// More info: https://kubernetes.io/docs/concepts/nodes/node/#info
	// +optional
	NodeInfo *corev1.NodeSystemInfo `json:"nodeInfo,omitempty"`

	// LastUpdated identifies when the phase of the Machine last transitioned.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *capierrors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Addresses is a list of addresses assigned to the machine.
	// This field is copied from the infrastructure provider reference.
	// +optional
	Addresses MachineAddresses `json:"addresses,omitempty"`

	// Phase represents the current phase of machine actuation.
	// E.g. Pending, Running, Terminating, Failed etc.
	// +optional
	Phase string `json:"phase,omitempty"`

	// BootstrapReady is the state of the bootstrap provider.
	// +optional
	BootstrapReady bool `json:"bootstrapReady"`

	// InfrastructureReady is the state of the infrastructure provider.
	// +optional
	InfrastructureReady bool `json:"infrastructureReady"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions defines current service state of the Machine.
	// +optional
	Conditions Conditions `json:"conditions,omitempty"`
}

// ANCHOR_END: MachineStatus

// SetTypedPhase sets the Phase field to the string representation of MachinePhase.
func (m *MachineStatus) SetTypedPhase(p MachinePhase) {
	m.Phase = string(p)
}

// GetTypedPhase attempts to parse the Phase field and return
// the typed MachinePhase representation as described in `machine_phase_types.go`.
func (m *MachineStatus) GetTypedPhase() MachinePhase {
	switch phase := MachinePhase(m.Phase); phase {
	case
		MachinePhasePending,
		MachinePhaseProvisioning,
		MachinePhaseProvisioned,
		MachinePhaseRunning,
		MachinePhaseDeleting,
		MachinePhaseDeleted,
		MachinePhaseFailed:
		return phase
	default:
		return MachinePhaseUnknown
	}
}

// ANCHOR: Bootstrap

// Bootstrap encapsulates fields to configure the Machine’s bootstrapping mechanism.
type Bootstrap struct {
	// ConfigRef is a reference to a bootstrap provider-specific resource
	// that holds configuration details. The reference is optional to
	// allow users/operators to specify Bootstrap.DataSecretName without
	// the need of a controller.
	// +optional
	ConfigRef *corev1.ObjectReference `json:"configRef,omitempty"`

	// DataSecretName is the name of the secret that stores the bootstrap data script.
	// If nil, the Machine should remain in the Pending state.
	// +optional
	DataSecretName *string `json:"dataSecretName,omitempty"`
}

// ANCHOR_END: Bootstrap

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machines,shortName=ma,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterName",description="Cluster"
// +kubebuilder:printcolumn:name="NodeName",type="string",JSONPath=".status.nodeRef.name",description="Node name associated with this machine"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Machine status such as Terminating/Pending/Running/Failed etc"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Machine"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version",description="Kubernetes version associated with this Machine"

// Machine is the Schema for the machines API.
type Machine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineSpec   `json:"spec,omitempty"`
	Status MachineStatus `json:"status,omitempty"`
}

// GetConditions returns the set of conditions for this object.
func (m *Machine) GetConditions() Conditions {
	return m.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (m *Machine) SetConditions(conditions Conditions) {
	m.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// MachineList contains a list of Machine.
type MachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Machine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Machine{}, &MachineList{})
}
