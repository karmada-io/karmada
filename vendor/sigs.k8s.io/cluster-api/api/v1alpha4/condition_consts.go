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

// ANCHOR: CommonConditions

// Common ConditionTypes used by Cluster API objects.
const (
	// ReadyCondition defines the Ready condition type that summarizes the operational state of a Cluster API object.
	ReadyCondition ConditionType = "Ready"
)

// Common ConditionReason used by Cluster API objects.
const (
	// DeletingReason (Severity=Info) documents an condition not in Status=True because the underlying object it is currently being deleted.
	DeletingReason = "Deleting"

	// DeletionFailedReason (Severity=Warning) documents an condition not in Status=True because the underlying object
	// encountered problems during deletion. This is a warning because the reconciler will retry deletion.
	DeletionFailedReason = "DeletionFailed"

	// DeletedReason (Severity=Info) documents an condition not in Status=True because the underlying object was deleted.
	DeletedReason = "Deleted"

	// IncorrectExternalRefReason (Severity=Error) documents a CAPI object with an incorrect external object reference.
	IncorrectExternalRefReason = "IncorrectExternalRef"
)

const (
	// InfrastructureReadyCondition reports a summary of current status of the infrastructure object defined for this cluster/machine/machinepool.
	// This condition is mirrored from the Ready condition in the infrastructure ref object, and
	// the absence of this condition might signal problems in the reconcile external loops or the fact that
	// the infrastructure provider does not implement the Ready condition yet.
	InfrastructureReadyCondition ConditionType = "InfrastructureReady"

	// WaitingForInfrastructureFallbackReason (Severity=Info) documents a cluster/machine/machinepool waiting for the underlying infrastructure
	// to be available.
	// NOTE: This reason is used only as a fallback when the infrastructure object is not reporting its own ready condition.
	WaitingForInfrastructureFallbackReason = "WaitingForInfrastructure"
)

// ANCHOR_END: CommonConditions

// Conditions and condition Reasons for the Cluster object

const (
	// ControlPlaneInitializedCondition reports if the cluster's control plane has been initialized such that the
	// cluster's apiserver is reachable and at least one control plane Machine has a node reference. Once this
	// condition is marked true, its value is never changed. See the ControlPlaneReady condition for an indication of
	// the current readiness of the cluster's control plane.
	ControlPlaneInitializedCondition ConditionType = "ControlPlaneInitialized"

	// MissingNodeRefReason (Severity=Info) documents a cluster waiting for at least one control plane Machine to have
	// its node reference populated.
	MissingNodeRefReason = "MissingNodeRef"

	// WaitingForControlPlaneProviderInitializedReason (Severity=Info) documents a cluster waiting for the control plane
	// provider to report successful control plane initialization.
	WaitingForControlPlaneProviderInitializedReason = "WaitingForControlPlaneProviderInitialized"

	// ControlPlaneReadyCondition reports the ready condition from the control plane object defined for this cluster.
	// This condition is mirrored from the Ready condition in the control plane ref object, and
	// the absence of this condition might signal problems in the reconcile external loops or the fact that
	// the control plane provider does not not implements the Ready condition yet.
	ControlPlaneReadyCondition ConditionType = "ControlPlaneReady"

	// WaitingForControlPlaneFallbackReason (Severity=Info) documents a cluster waiting for the control plane
	// to be available.
	// NOTE: This reason is used only as a fallback when the control plane object is not reporting its own ready condition.
	WaitingForControlPlaneFallbackReason = "WaitingForControlPlane"

	// WaitingForControlPlaneAvailableReason (Severity=Info) documents a Cluster API object
	// waiting for the control plane machine to be available.
	//
	// NOTE: Having the control plane machine available is a pre-condition for joining additional control planes
	// or workers nodes.
	WaitingForControlPlaneAvailableReason = "WaitingForControlPlaneAvailable"
)

// Conditions and condition Reasons for the Machine object

const (
	// BootstrapReadyCondition reports a summary of current status of the bootstrap object defined for this machine.
	// This condition is mirrored from the Ready condition in the bootstrap ref object, and
	// the absence of this condition might signal problems in the reconcile external loops or the fact that
	// the bootstrap provider does not implement the Ready condition yet.
	BootstrapReadyCondition ConditionType = "BootstrapReady"

	// WaitingForDataSecretFallbackReason (Severity=Info) documents a machine waiting for the bootstrap data secret
	// to be available.
	// NOTE: This reason is used only as a fallback when the bootstrap object is not reporting its own ready condition.
	WaitingForDataSecretFallbackReason = "WaitingForDataSecret"

	// DrainingSucceededCondition provide evidence of the status of the node drain operation which happens during the machine
	// deletion process.
	DrainingSucceededCondition ConditionType = "DrainingSucceeded"

	// DrainingReason (Severity=Info) documents a machine node being drained.
	DrainingReason = "Draining"

	// DrainingFailedReason (Severity=Warning) documents a machine node drain operation failed.
	DrainingFailedReason = "DrainingFailed"

	// PreDrainDeleteHookSucceededCondition reports a machine waiting for a PreDrainDeleteHook before being delete.
	PreDrainDeleteHookSucceededCondition ConditionType = "PreDrainDeleteHookSucceeded"

	// PreTerminateDeleteHookSucceededCondition reports a machine waiting for a PreDrainDeleteHook before being delete.
	PreTerminateDeleteHookSucceededCondition ConditionType = "PreTerminateDeleteHookSucceeded"

	// WaitingExternalHookReason (Severity=Info) provide evidence that we are waiting for an external hook to complete.
	WaitingExternalHookReason = "WaitingExternalHook"

	// VolumeDetachSucceededCondition reports a machine waiting for volumes to be detached.
	VolumeDetachSucceededCondition ConditionType = "VolumeDetachSucceeded"

	// WaitingForVolumeDetachReason (Severity=Info) provide evidence that a machine node waiting for volumes to be attached.
	WaitingForVolumeDetachReason = "WaitingForVolumeDetach"
)

const (
	// MachineHealthCheckSuccededCondition is set on machines that have passed a healthcheck by the MachineHealthCheck controller.
	// In the event that the health check fails it will be set to False.
	MachineHealthCheckSuccededCondition ConditionType = "HealthCheckSucceeded"

	// MachineHasFailureReason is the reason used when a machine has either a FailureReason or a FailureMessage set on its status.
	MachineHasFailureReason = "MachineHasFailure"

	// NodeStartupTimeoutReason is the reason used when a machine's node does not appear within the specified timeout.
	NodeStartupTimeoutReason = "NodeStartupTimeout"

	// UnhealthyNodeConditionReason is the reason used when a machine's node has one of the MachineHealthCheck's unhealthy conditions.
	UnhealthyNodeConditionReason = "UnhealthyNode"
)

const (
	// MachineOwnerRemediatedCondition is set on machines that have failed a healthcheck by the MachineHealthCheck controller.
	// MachineOwnerRemediatedCondition is set to False after a health check fails, but should be changed to True by the owning controller after remediation succeeds.
	MachineOwnerRemediatedCondition ConditionType = "OwnerRemediated"

	// WaitingForRemediationReason is the reason used when a machine fails a health check and remediation is needed.
	WaitingForRemediationReason = "WaitingForRemediation"

	// RemediationFailedReason is the reason used when a remediation owner fails to remediate an unhealthy machine.
	RemediationFailedReason = "RemediationFailed"

	// RemediationInProgressReason is the reason used when an unhealthy machine is being remediated by the remediation owner.
	RemediationInProgressReason = "RemediationInProgress"

	// ExternalRemediationTemplateAvailable is set on machinehealthchecks when MachineHealthCheck controller uses external remediation.
	// ExternalRemediationTemplateAvailable is set to false if external remediation template is not found.
	ExternalRemediationTemplateAvailable ConditionType = "ExternalRemediationTemplateAvailable"

	// ExternalRemediationTemplateNotFound is the reason used when a machine health check fails to find external remediation template.
	ExternalRemediationTemplateNotFound = "ExternalRemediationTemplateNotFound"

	// ExternalRemediationRequestAvailable is set on machinehealthchecks when MachineHealthCheck controller uses external remediation.
	// ExternalRemediationRequestAvailable is set to false if creating external remediation request fails.
	ExternalRemediationRequestAvailable ConditionType = "ExternalRemediationRequestAvailable"

	// ExternalRemediationRequestCreationFailed is the reason used when a machine health check fails to create external remediation request.
	ExternalRemediationRequestCreationFailed = "ExternalRemediationRequestCreationFailed"
)

// Conditions and condition Reasons for the Machine's Node object.
const (
	// MachineNodeHealthyCondition provides info about the operational state of the Kubernetes node hosted on the machine by summarizing  node conditions.
	// If the conditions defined in a Kubernetes node (i.e., NodeReady, NodeMemoryPressure, NodeDiskPressure, NodePIDPressure, and NodeNetworkUnavailable) are in a healthy state, it will be set to True.
	MachineNodeHealthyCondition ConditionType = "NodeHealthy"

	// WaitingForNodeRefReason (Severity=Info) documents a machine.spec.providerId is not assigned yet.
	WaitingForNodeRefReason = "WaitingForNodeRef"

	// NodeProvisioningReason (Severity=Info) documents machine in the process of provisioning a node.
	// NB. provisioning --> NodeRef == "".
	NodeProvisioningReason = "NodeProvisioning"

	// NodeNotFoundReason (Severity=Error) documents a machine's node has previously been observed but is now gone.
	// NB. provisioned --> NodeRef != "".
	NodeNotFoundReason = "NodeNotFound"

	// NodeConditionsFailedReason (Severity=Warning) documents a node is not in a healthy state due to the failed state of at least 1 Kubelet condition.
	NodeConditionsFailedReason = "NodeConditionsFailed"
)

// Conditions and condition Reasons for the MachineHealthCheck object

const (
	// RemediationAllowedCondition is set on MachineHealthChecks to show the status of whether the MachineHealthCheck is
	// allowed to remediate any Machines or whether it is blocked from remediating any further.
	RemediationAllowedCondition ConditionType = "RemediationAllowed"

	// TooManyUnhealthyReason is the reason used when too many Machines are unhealthy and the MachineHealthCheck is blocked
	// from making any further remediations.
	TooManyUnhealthyReason = "TooManyUnhealthy"
)

// Conditions and condition Reasons for  MachineDeployments

const (
	// MachineDeploymentAvailableCondition means the MachineDeployment is available, that is, at least the minimum available
	// machines required (i.e. Spec.Replicas-MaxUnavailable when MachineDeploymentStrategyType = RollingUpdate) are up and running for at least minReadySeconds.
	MachineDeploymentAvailableCondition ConditionType = "Available"

	// WaitingForAvailableMachinesReason (Severity=Warning) reflects the fact that the required minimum number of machines for a machinedeployment are not available.
	WaitingForAvailableMachinesReason = "WaitingForAvailableMachines"
)

// Conditions and condition Reasons for  MachineSets

const (
	// MachinesCreatedCondition documents that the machines controlled by the MachineSet are created.
	// When this condition is false, it indicates that there was an error when cloning the infrastructure/bootstrap template or
	// when generating the machine object.
	MachinesCreatedCondition ConditionType = "MachinesCreated"

	// MachinesReadyCondition reports an aggregate of current status of the machines controlled by the MachineSet.
	MachinesReadyCondition ConditionType = "MachinesReady"

	// BootstrapTemplateCloningFailedReason (Severity=Error) documents a MachineSet failing to
	// clone the bootstrap template.
	BootstrapTemplateCloningFailedReason = "BootstrapTemplateCloningFailed"

	// InfrastructureTemplateCloningFailedReason (Severity=Error) documents a MachineSet failing to
	// clone the infrastructure template.
	InfrastructureTemplateCloningFailedReason = "InfrastructureTemplateCloningFailed"

	// MachineCreationFailedReason (Severity=Error) documents a MachineSet failing to
	// generate a machine object.
	MachineCreationFailedReason = "MachineCreationFailed"

	// ResizedCondition documents a MachineSet is resizing the set of controlled machines.
	ResizedCondition ConditionType = "Resized"

	// ScalingUpReason (Severity=Info) documents a MachineSet is increasing the number of replicas.
	ScalingUpReason = "ScalingUp"

	// ScalingDownReason (Severity=Info) documents a MachineSet is decreasing the number of replicas.
	ScalingDownReason = "ScalingDown"
)
