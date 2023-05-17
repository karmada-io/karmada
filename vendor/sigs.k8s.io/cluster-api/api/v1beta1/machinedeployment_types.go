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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// MachineDeploymentTopologyFinalizer is the finalizer used by the topology MachineDeployment controller to
	// clean up referenced template resources if necessary when a MachineDeployment is being deleted.
	MachineDeploymentTopologyFinalizer = "machinedeployment.topology.cluster.x-k8s.io"
)

// MachineDeploymentStrategyType defines the type of MachineDeployment rollout strategies.
type MachineDeploymentStrategyType string

const (
	// RollingUpdateMachineDeploymentStrategyType replaces the old MachineSet by new one using rolling update
	// i.e. gradually scale down the old MachineSet and scale up the new one.
	RollingUpdateMachineDeploymentStrategyType MachineDeploymentStrategyType = "RollingUpdate"

	// OnDeleteMachineDeploymentStrategyType replaces old MachineSets when the deletion of the associated machines are completed.
	OnDeleteMachineDeploymentStrategyType MachineDeploymentStrategyType = "OnDelete"

	// RevisionAnnotation is the revision annotation of a machine deployment's machine sets which records its rollout sequence.
	RevisionAnnotation = "machinedeployment.clusters.x-k8s.io/revision"

	// RevisionHistoryAnnotation maintains the history of all old revisions that a machine set has served for a machine deployment.
	RevisionHistoryAnnotation = "machinedeployment.clusters.x-k8s.io/revision-history"

	// DesiredReplicasAnnotation is the desired replicas for a machine deployment recorded as an annotation
	// in its machine sets. Helps in separating scaling events from the rollout process and for
	// determining if the new machine set for a deployment is really saturated.
	DesiredReplicasAnnotation = "machinedeployment.clusters.x-k8s.io/desired-replicas"

	// MaxReplicasAnnotation is the maximum replicas a deployment can have at a given point, which
	// is machinedeployment.spec.replicas + maxSurge. Used by the underlying machine sets to estimate their
	// proportions in case the deployment has surge replicas.
	MaxReplicasAnnotation = "machinedeployment.clusters.x-k8s.io/max-replicas"

	// MachineDeploymentUniqueLabel is used to uniquely identify the Machines of a MachineSet.
	// The MachineDeployment controller will set this label on a MachineSet when it is created.
	// The label is also applied to the Machines of the MachineSet and used in the MachineSet selector.
	// Note: For the lifetime of the MachineSet the label's value has to stay the same, otherwise the
	// MachineSet selector would no longer match its Machines.
	// Note: In previous Cluster API versions (< v1.4.0), the label value was the hash of the full machine template.
	// With the introduction of in-place mutation the machine template of the MachineSet can change.
	// Because of that it is impossible that the label's value to always be the hash of the full machine template.
	// (Because the hash changes when the machine template changes).
	// As a result, we use the hash of the machine template while ignoring all in-place mutable fields, i.e. the
	// machine template with only fields that could trigger a rollout for the machine-template-hash, making it
	// independent of the changes to any in-place mutable fields.
	// A random string is appended at the end of the label value (label value format is "<hash>-<random string>"))
	// to distinguish duplicate MachineSets that have the exact same spec but were created as a result of rolloutAfter.
	MachineDeploymentUniqueLabel = "machine-template-hash"
)

// ANCHOR: MachineDeploymentSpec

// MachineDeploymentSpec defines the desired state of MachineDeployment.
type MachineDeploymentSpec struct {
	// ClusterName is the name of the Cluster this object belongs to.
	// +kubebuilder:validation:MinLength=1
	ClusterName string `json:"clusterName"`

	// Number of desired machines.
	// This is a pointer to distinguish between explicit zero and not specified.
	//
	// Defaults to:
	// * if the Kubernetes autoscaler min size and max size annotations are set:
	//   - if it's a new MachineDeployment, use min size
	//   - if the replicas field of the old MachineDeployment is < min size, use min size
	//   - if the replicas field of the old MachineDeployment is > max size, use max size
	//   - if the replicas field of the old MachineDeployment is in the (min size, max size) range, keep the value from the oldMD
	// * otherwise use 1
	// Note: Defaulting will be run whenever the replicas field is not set:
	// * A new MachineDeployment is created with replicas not set.
	// * On an existing MachineDeployment the replicas field was first set and is now unset.
	// Those cases are especially relevant for the following Kubernetes autoscaler use cases:
	// * A new MachineDeployment is created and replicas should be managed by the autoscaler
	// * An existing MachineDeployment which initially wasn't controlled by the autoscaler
	//   should be later controlled by the autoscaler
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// RolloutAfter is a field to indicate a rollout should be performed
	// after the specified time even if no changes have been made to the
	// MachineDeployment.
	// Example: In the YAML the time can be specified in the RFC3339 format.
	// To specify the rolloutAfter target as March 9, 2023, at 9 am UTC
	// use "2023-03-09T09:00:00Z".
	// +optional
	RolloutAfter *metav1.Time `json:"rolloutAfter,omitempty"`

	// Label selector for machines. Existing MachineSets whose machines are
	// selected by this will be the ones affected by this deployment.
	// It must match the machine template's labels.
	Selector metav1.LabelSelector `json:"selector"`

	// Template describes the machines that will be created.
	Template MachineTemplateSpec `json:"template"`

	// The deployment strategy to use to replace existing machines with
	// new ones.
	// +optional
	Strategy *MachineDeploymentStrategy `json:"strategy,omitempty"`

	// Minimum number of seconds for which a newly created machine should
	// be ready.
	// Defaults to 0 (machine will be considered available as soon as it
	// is ready)
	// +optional
	MinReadySeconds *int32 `json:"minReadySeconds,omitempty"`

	// The number of old MachineSets to retain to allow rollback.
	// This is a pointer to distinguish between explicit zero and not specified.
	// Defaults to 1.
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`

	// Indicates that the deployment is paused.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// The maximum time in seconds for a deployment to make progress before it
	// is considered to be failed. The deployment controller will continue to
	// process failed deployments and a condition with a ProgressDeadlineExceeded
	// reason will be surfaced in the deployment status. Note that progress will
	// not be estimated during the time a deployment is paused. Defaults to 600s.
	// +optional
	ProgressDeadlineSeconds *int32 `json:"progressDeadlineSeconds,omitempty"`
}

// ANCHOR_END: MachineDeploymentSpec

// ANCHOR: MachineDeploymentStrategy

// MachineDeploymentStrategy describes how to replace existing machines
// with new ones.
type MachineDeploymentStrategy struct {
	// Type of deployment.
	// Default is RollingUpdate.
	// +kubebuilder:validation:Enum=RollingUpdate;OnDelete
	// +optional
	Type MachineDeploymentStrategyType `json:"type,omitempty"`

	// Rolling update config params. Present only if
	// MachineDeploymentStrategyType = RollingUpdate.
	// +optional
	RollingUpdate *MachineRollingUpdateDeployment `json:"rollingUpdate,omitempty"`
}

// ANCHOR_END: MachineDeploymentStrategy

// ANCHOR: MachineRollingUpdateDeployment

// MachineRollingUpdateDeployment is used to control the desired behavior of rolling update.
type MachineRollingUpdateDeployment struct {
	// The maximum number of machines that can be unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of desired
	// machines (ex: 10%).
	// Absolute number is calculated from percentage by rounding down.
	// This can not be 0 if MaxSurge is 0.
	// Defaults to 0.
	// Example: when this is set to 30%, the old MachineSet can be scaled
	// down to 70% of desired machines immediately when the rolling update
	// starts. Once new machines are ready, old MachineSet can be scaled
	// down further, followed by scaling up the new MachineSet, ensuring
	// that the total number of machines available at all times
	// during the update is at least 70% of desired machines.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// The maximum number of machines that can be scheduled above the
	// desired number of machines.
	// Value can be an absolute number (ex: 5) or a percentage of
	// desired machines (ex: 10%).
	// This can not be 0 if MaxUnavailable is 0.
	// Absolute number is calculated from percentage by rounding up.
	// Defaults to 1.
	// Example: when this is set to 30%, the new MachineSet can be scaled
	// up immediately when the rolling update starts, such that the total
	// number of old and new machines do not exceed 130% of desired
	// machines. Once old machines have been killed, new MachineSet can
	// be scaled up further, ensuring that total number of machines running
	// at any time during the update is at most 130% of desired machines.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`

	// DeletePolicy defines the policy used by the MachineDeployment to identify nodes to delete when downscaling.
	// Valid values are "Random, "Newest", "Oldest"
	// When no value is supplied, the default DeletePolicy of MachineSet is used
	// +kubebuilder:validation:Enum=Random;Newest;Oldest
	// +optional
	DeletePolicy *string `json:"deletePolicy,omitempty"`
}

// ANCHOR_END: MachineRollingUpdateDeployment

// ANCHOR: MachineDeploymentStatus

// MachineDeploymentStatus defines the observed state of MachineDeployment.
type MachineDeploymentStatus struct {
	// The generation observed by the deployment controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Selector is the same as the label selector but in the string format to avoid introspection
	// by clients. The string will be in the same format as the query-param syntax.
	// More info about label selectors: http://kubernetes.io/docs/user-guide/labels#label-selectors
	// +optional
	Selector string `json:"selector,omitempty"`

	// Total number of non-terminated machines targeted by this deployment
	// (their labels match the selector).
	// +optional
	Replicas int32 `json:"replicas"`

	// Total number of non-terminated machines targeted by this deployment
	// that have the desired template spec.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas"`

	// Total number of ready machines targeted by this deployment.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas"`

	// Total number of available machines (ready for at least minReadySeconds)
	// targeted by this deployment.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas"`

	// Total number of unavailable machines targeted by this deployment.
	// This is the total number of machines that are still required for
	// the deployment to have 100% available capacity. They may either
	// be machines that are running but not yet available or machines
	// that still have not been created.
	// +optional
	UnavailableReplicas int32 `json:"unavailableReplicas"`

	// Phase represents the current phase of a MachineDeployment (ScalingUp, ScalingDown, Running, Failed, or Unknown).
	// +optional
	Phase string `json:"phase,omitempty"`

	// Conditions defines current service state of the MachineDeployment.
	// +optional
	Conditions Conditions `json:"conditions,omitempty"`
}

// ANCHOR_END: MachineDeploymentStatus

// MachineDeploymentPhase indicates the progress of the machine deployment.
type MachineDeploymentPhase string

const (
	// MachineDeploymentPhaseScalingUp indicates the MachineDeployment is scaling up.
	MachineDeploymentPhaseScalingUp = MachineDeploymentPhase("ScalingUp")

	// MachineDeploymentPhaseScalingDown indicates the MachineDeployment is scaling down.
	MachineDeploymentPhaseScalingDown = MachineDeploymentPhase("ScalingDown")

	// MachineDeploymentPhaseRunning indicates scaling has completed and all Machines are running.
	MachineDeploymentPhaseRunning = MachineDeploymentPhase("Running")

	// MachineDeploymentPhaseFailed indicates there was a problem scaling and user intervention might be required.
	MachineDeploymentPhaseFailed = MachineDeploymentPhase("Failed")

	// MachineDeploymentPhaseUnknown indicates the state of the MachineDeployment cannot be determined.
	MachineDeploymentPhaseUnknown = MachineDeploymentPhase("Unknown")
)

// SetTypedPhase sets the Phase field to the string representation of MachineDeploymentPhase.
func (md *MachineDeploymentStatus) SetTypedPhase(p MachineDeploymentPhase) {
	md.Phase = string(p)
}

// GetTypedPhase attempts to parse the Phase field and return
// the typed MachineDeploymentPhase representation.
func (md *MachineDeploymentStatus) GetTypedPhase() MachineDeploymentPhase {
	switch phase := MachineDeploymentPhase(md.Phase); phase {
	case
		MachineDeploymentPhaseScalingDown,
		MachineDeploymentPhaseScalingUp,
		MachineDeploymentPhaseRunning,
		MachineDeploymentPhaseFailed:
		return phase
	default:
		return MachineDeploymentPhaseUnknown
	}
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machinedeployments,shortName=md,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterName",description="Cluster"
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=".spec.replicas",description="Total number of machines desired by this MachineDeployment",priority=10
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".status.replicas",description="Total number of non-terminated machines targeted by this MachineDeployment"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas",description="Total number of ready machines targeted by this MachineDeployment"
// +kubebuilder:printcolumn:name="Updated",type=integer,JSONPath=".status.updatedReplicas",description="Total number of non-terminated machines targeted by this deployment that have the desired template spec"
// +kubebuilder:printcolumn:name="Unavailable",type=integer,JSONPath=".status.unavailableReplicas",description="Total number of unavailable machines targeted by this MachineDeployment"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="MachineDeployment status such as ScalingUp/ScalingDown/Running/Failed/Unknown"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of MachineDeployment"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.template.spec.version",description="Kubernetes version associated with this MachineDeployment"

// MachineDeployment is the Schema for the machinedeployments API.
type MachineDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineDeploymentSpec   `json:"spec,omitempty"`
	Status MachineDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MachineDeploymentList contains a list of MachineDeployment.
type MachineDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MachineDeployment{}, &MachineDeploymentList{})
}

// GetConditions returns the set of conditions for the machinedeployment.
func (m *MachineDeployment) GetConditions() Conditions {
	return m.Status.Conditions
}

// SetConditions updates the set of conditions on the machinedeployment.
func (m *MachineDeployment) SetConditions(conditions Conditions) {
	m.Status.Conditions = conditions
}
