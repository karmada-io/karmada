/*
Copyright 2022 The Karmada Authors.

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

package events

// Define events for cluster objects.
const (
	// EventReasonCreateExecutionSpaceFailed indicates that create execution space failed.
	EventReasonCreateExecutionSpaceFailed = "CreateExecutionSpaceFailed"
	// EventReasonCreateExecutionSpaceSucceed indicates that create execution space succeed.
	EventReasonCreateExecutionSpaceSucceed = "CreateExecutionSpaceSucceed"
	// EventReasonRemoveExecutionSpaceFailed indicates that remove execution space failed.
	EventReasonRemoveExecutionSpaceFailed = "RemoveExecutionSpaceFailed"
	// EventReasonRemoveExecutionSpaceSucceed indicates that remove execution space succeed.
	EventReasonRemoveExecutionSpaceSucceed = "RemoveExecutionSpaceSucceed"
	// EventReasonTaintClusterFailed indicates that taint cluster failed.
	EventReasonTaintClusterFailed = "TaintClusterFailed"
	// EventReasonTaintClusterSucceed indicates that taint cluster succeed.
	EventReasonTaintClusterSucceed = "TaintClusterSucceed"
	// EventReasonSyncImpersonationConfigSucceed indicates that sync impersonation config succeed.
	EventReasonSyncImpersonationConfigSucceed = "SyncImpersonationConfigSucceed"
	// EventReasonSyncImpersonationConfigFailed indicates that sync impersonation config failed.
	EventReasonSyncImpersonationConfigFailed = "SyncImpersonationConfigFailed"
)

// Define events for work objects.
const (
	// EventReasonReflectStatusSucceed indicates that reflect status to work succeed.
	EventReasonReflectStatusSucceed = "ReflectStatusSucceed"
	// EventReasonReflectStatusFailed indicates that reflect status to work failed.
	EventReasonReflectStatusFailed = "ReflectStatusFailed"
	// EventReasonInterpretHealthSucceed indicates that interpret health succeed.
	EventReasonInterpretHealthSucceed = "InterpretHealthSucceed"
	// EventReasonInterpretHealthFailed indicates that interpret health failed.
	EventReasonInterpretHealthFailed = "InterpretHealthFailed"
)

// Define events for work objects and their associated resources.
const (
	// EventReasonSyncWorkloadFailed indicates that Sync workload failed.
	EventReasonSyncWorkloadFailed = "SyncFailed"
	// EventReasonSyncWorkloadSucceed indicates that Sync workload succeed.
	EventReasonSyncWorkloadSucceed = "SyncSucceed"
	// EventReasonWorkDispatching indicates that work is dispatching or not.
	EventReasonWorkDispatching = "WorkDispatching"
)

// Define events for ResourceBinding and ClusterResourceBinding objects.
const (
	// EventReasonCleanupWorkFailed indicates that Cleanup work failed.
	EventReasonCleanupWorkFailed = "CleanupWorkFailed"
	// EventReasonSyncScheduleResultToDependenciesSucceed indicates sync schedule result to attached bindings succeed.
	EventReasonSyncScheduleResultToDependenciesSucceed = "SyncScheduleResultToDependenciesSucceed"
	// EventReasonSyncScheduleResultToDependenciesFailed indicates sync schedule result to attached bindings failed.
	EventReasonSyncScheduleResultToDependenciesFailed = "SyncScheduleResultToDependenciesFailed"
	// EventReasonDependencyPolicyConflict indicates a dependency policy conflict was detected.
	EventReasonDependencyPolicyConflict = "DependencyPolicyConflict"
)

// Define events for ResourceBinding, ClusterResourceBinding objects and their associated resources.
const (
	// EventReasonSyncWorkFailed indicates that Sync work failed.
	EventReasonSyncWorkFailed = "SyncWorkFailed"
	// EventReasonSyncWorkSucceed indicates that Sync work succeed.
	EventReasonSyncWorkSucceed = "SyncWorkSucceed"
	// EventReasonAggregateStatusFailed indicates that aggregate status failed.
	EventReasonAggregateStatusFailed = "AggregateStatusFailed"
	// EventReasonAggregateStatusSucceed indicates that aggregate status succeed.
	EventReasonAggregateStatusSucceed = "AggregateStatusSucceed"
	// EventReasonScheduleBindingFailed indicates that schedule binding failed.
	EventReasonScheduleBindingFailed = "ScheduleBindingFailed"
	// EventReasonScheduleBindingSucceed indicates that schedule binding succeed.
	EventReasonScheduleBindingSucceed = "ScheduleBindingSucceed"
	// EventReasonDescheduleBindingFailed indicates that deschedule binding failed.
	EventReasonDescheduleBindingFailed = "DescheduleBindingFailed"
	// EventReasonDescheduleBindingSucceed indicates that deschedule binding succeed.
	EventReasonDescheduleBindingSucceed = "DescheduleBindingSucceed"
	// EventReasonEvictWorkloadFromClusterSucceed indicates that evict workload from cluster succeed.
	EventReasonEvictWorkloadFromClusterSucceed = "EvictWorkloadFromClusterSucceed"
	// EventReasonEvictWorkloadFromClusterFailed indicates that evict workload from cluster failed.
	EventReasonEvictWorkloadFromClusterFailed = "EvictWorkloadFromClusterFailed"
)

// Define events for FederatedResourceQuota.
const (
	// EventReasonSyncFederatedResourceQuotaFailed indicates that Sync work failed.
	EventReasonSyncFederatedResourceQuotaFailed = "SyncWorkFailed"
	// EventReasonSyncFederatedResourceQuotaSucceed indicates that Sync work succeed.
	EventReasonSyncFederatedResourceQuotaSucceed = "SyncWorkSucceed"
	// EventReasonCollectFederatedResourceQuotaStatusFailed indicates that aggregate status failed.
	EventReasonCollectFederatedResourceQuotaStatusFailed = "AggregateStatusFailed"
	// EventReasonCollectFederatedResourceQuotaStatusSucceed indicates that aggregate status succeed.
	EventReasonCollectFederatedResourceQuotaStatusSucceed = "AggregateStatusSucceed"
	// EventReasonCollectFederatedResourceQuotaOverallStatusFailed indicates that Collect Overall Status failed.
	EventReasonCollectFederatedResourceQuotaOverallStatusFailed = "CollectOverallStatusFailed"
	// EventReasonCollectFederatedResourceQuotaOverallStatusSucceed indicates that Collect Overall Status succeed.
	EventReasonCollectFederatedResourceQuotaOverallStatusSucceed = "CollectOverallStatusSucceed"
)

// Define events for resource templates.
const (
	// EventReasonApplyPolicyFailed indicates that apply policy for resource failed.
	EventReasonApplyPolicyFailed = "ApplyPolicyFailed"
	// EventReasonApplyPolicySucceed indicates that apply policy for resource succeed.
	EventReasonApplyPolicySucceed = "ApplyPolicySucceed"
	// EventReasonApplyOverridePolicyFailed indicates that apply override policy failed.
	EventReasonApplyOverridePolicyFailed = "ApplyOverridePolicyFailed"
	// EventReasonApplyOverridePolicySucceed indicates that apply override policy succeed.
	EventReasonApplyOverridePolicySucceed = "ApplyOverridePolicySucceed"
	// EventReasonGetDependenciesSucceed indicates get dependencies of resource template succeed.
	EventReasonGetDependenciesSucceed = "GetDependenciesSucceed"
	// EventReasonGetDependenciesFailed indicates get dependencies of resource template failed.
	EventReasonGetDependenciesFailed = "GetDependenciesFailed"
	// EventReasonGetComponentsSucceed indicates get components of resource template succeed.
	EventReasonGetComponentsSucceed = "GetComponentsSucceed"
	// EventReasonGetComponentsFailed indicates get components of resource template failed.
	EventReasonGetComponentsFailed = "GetComponentsFailed"
	// EventReasonGetReplicasSucceed indicates get replicas of resource template succeed.
	EventReasonGetReplicasSucceed = "GetReplicasSucceed"
	// EventReasonGetReplicasFailed indicates get replicas of resource template failed.
	EventReasonGetReplicasFailed = "GetReplicasFailed"
	// EventReasonPreemptPolicySucceed indicates policy preemption of resource template succeed.
	EventReasonPreemptPolicySucceed = "PreemptPolicySucceed"
	// EventReasonPreemptPolicyFailed indicates policy preemption of resource template failed.
	EventReasonPreemptPolicyFailed = "PreemptPolicyFailed"
)

// Define events for ServiceImport objects.
const (
	// EventReasonSyncDerivedServiceSucceed indicates that sync derived service succeed.
	EventReasonSyncDerivedServiceSucceed = "SyncDerivedServiceSucceed"
	// EventReasonSyncDerivedServiceFailed indicates that sync derived service failed.
	EventReasonSyncDerivedServiceFailed = "SyncDerivedServiceFailed"
)

// Define events for MultiClusterService objects with CrossCluster type.
const (
	// EventReasonSyncServiceFailed is indicates that sync service failed.
	EventReasonSyncServiceFailed string = "SyncServiceFailed"
	// EventReasonSyncServiceSucceed is indicates that sync service succeed.
	EventReasonSyncServiceSucceed string = "SyncServiceSucceed"
	// EventReasonDispatchEndpointSliceFailed indicates that dispatch endpointslice failed.
	EventReasonDispatchEndpointSliceFailed = "DispatchEndpointSliceFailed"
	// EventReasonDispatchEndpointSliceSucceed indicates that dispatch endpointslice succeed.
	EventReasonDispatchEndpointSliceSucceed = "DispatchEndpointSliceSucceed"
	// EventReasonClusterNotFound indicates that the cluster configured in MultiClusterService does not exist.
	EventReasonClusterNotFound = "ClusterNotFound"
	// EventReasonAPIIncompatible indicates that the MultiClusterService may not function properly as some member clusters do not support EndpointSlice.
	EventReasonAPIIncompatible = "APIIncompatible"
)

// Define event reasons for CronFederatedHPA. Values are kept byte-identical
// to the strings previously emitted as inline literals, so any external
// watcher keyed on these strings continues to match.
const (
	// EventReasonStartCronFederatedHPARuleFailed indicates that starting the
	// cron executor for a CronFederatedHPA rule failed.
	EventReasonStartCronFederatedHPARuleFailed = "StartRuleFailed"
	// EventReasonUpdateCronFederatedHPAFailed indicates that updating the
	// CronFederatedHPA object failed.
	EventReasonUpdateCronFederatedHPAFailed = "UpdateCronFederatedHPAFailed"
	// EventReasonScaleCronFederatedHPAFailed indicates that scaling the target
	// referenced by a CronFederatedHPA failed.
	EventReasonScaleCronFederatedHPAFailed = "ScaleFailed"
	// EventReasonUpdateCronFederatedHPAStatusFailed indicates that updating the
	// CronFederatedHPA status with execution history failed.
	EventReasonUpdateCronFederatedHPAStatusFailed = "UpdateStatusFailed"
)

// Event actions for events.k8s.io/v1 Events, established by issue #7251.
//
// Action describes the operation a controller performed, independent of
// outcome (e.g. "ScheduleBinding" pairs with both "ScheduleBindingSucceed"
// and "ScheduleBindingFailed" reasons). The events.k8s.io/v1 API requires
// action as a machine-readable string, unlike the deprecated core/v1 Event.
// A single action may be paired with multiple reasons over time as new
// failure modes are surfaced; reusing one action across those reasons is
// the intended pattern, matching upstream Kubernetes (e.g. kube-scheduler
// emits reason "Scheduled" or "FailedScheduling" with action "Binding").
//
// Action names derive from paired reasons by dropping the Failed/Succeed
// suffix; singleton reasons (no paired success/failure variant) get an
// action named after the emitting operation.
const (
	// EventActionCreateExecutionSpace indicates the action of creating an execution space for a cluster.
	EventActionCreateExecutionSpace = "CreateExecutionSpace"
	// EventActionRemoveExecutionSpace indicates the action of removing the execution space of a cluster.
	EventActionRemoveExecutionSpace = "RemoveExecutionSpace"
	// EventActionTaintCluster indicates the action of tainting or untainting a cluster.
	EventActionTaintCluster = "TaintCluster"
	// EventActionSyncImpersonationConfig indicates the action of syncing impersonation config for a cluster.
	EventActionSyncImpersonationConfig = "SyncImpersonationConfig"

	// EventActionReflectStatus indicates the action of reflecting member-cluster status into a Work.
	EventActionReflectStatus = "ReflectStatus"
	// EventActionInterpretHealth indicates the action of interpreting workload health.
	EventActionInterpretHealth = "InterpretHealth"

	// EventActionSyncWorkload indicates the action of syncing a workload into a member cluster.
	EventActionSyncWorkload = "SyncWorkload"
	// EventActionDispatchWork indicates the action of dispatching a Work to a member cluster.
	EventActionDispatchWork = "DispatchWork"

	// EventActionCleanupWork indicates the action of cleaning up Work objects for a binding.
	EventActionCleanupWork = "CleanupWork"
	// EventActionSyncScheduleResultToDependencies indicates the action of syncing a schedule result to dependent bindings.
	EventActionSyncScheduleResultToDependencies = "SyncScheduleResultToDependencies"
	// EventActionResolveDependencyPolicy indicates the action of resolving the policy governing a dependency relationship.
	EventActionResolveDependencyPolicy = "ResolveDependencyPolicy"

	// EventActionSyncWork indicates the action of syncing a Work derived from a (Cluster)ResourceBinding.
	EventActionSyncWork = "SyncWork"
	// EventActionAggregateStatus indicates the action of aggregating per-cluster status into a binding.
	EventActionAggregateStatus = "AggregateStatus"
	// EventActionScheduleBinding indicates the action of scheduling a (Cluster)ResourceBinding to member clusters.
	EventActionScheduleBinding = "ScheduleBinding"
	// EventActionDescheduleBinding indicates the action of descheduling a (Cluster)ResourceBinding from a member cluster.
	EventActionDescheduleBinding = "DescheduleBinding"
	// EventActionEvictWorkloadFromCluster indicates the action of evicting a workload from a member cluster.
	EventActionEvictWorkloadFromCluster = "EvictWorkloadFromCluster"

	// EventActionSyncFederatedResourceQuota indicates the action of syncing a FederatedResourceQuota into member clusters.
	EventActionSyncFederatedResourceQuota = "SyncFederatedResourceQuota"
	// EventActionCollectFederatedResourceQuotaStatus indicates the action of collecting per-cluster status of a FederatedResourceQuota.
	EventActionCollectFederatedResourceQuotaStatus = "CollectFederatedResourceQuotaStatus"
	// EventActionCollectFederatedResourceQuotaOverallStatus indicates the action of computing the overall status of a FederatedResourceQuota.
	EventActionCollectFederatedResourceQuotaOverallStatus = "CollectFederatedResourceQuotaOverallStatus"

	// EventActionApplyPolicy indicates the action of applying a (Cluster)PropagationPolicy to a resource template.
	EventActionApplyPolicy = "ApplyPolicy"
	// EventActionApplyOverridePolicy indicates the action of applying a (Cluster)OverridePolicy.
	EventActionApplyOverridePolicy = "ApplyOverridePolicy"
	// EventActionPreemptPolicy indicates the action of one policy preempting another's resource template selection.
	EventActionPreemptPolicy = "PreemptPolicy"
	// EventActionGetDependencies indicates the action of resolving dependent resources via the interpreter.
	EventActionGetDependencies = "GetDependencies"
	// EventActionGetComponents indicates the action of resolving components of a resource template via the interpreter.
	EventActionGetComponents = "GetComponents"
	// EventActionGetReplicas indicates the action of resolving the replica count of a resource template via the interpreter.
	EventActionGetReplicas = "GetReplicas"

	// EventActionSyncDerivedService indicates the action of syncing a derived service for a ServiceImport.
	EventActionSyncDerivedService = "SyncDerivedService"

	// EventActionSyncService indicates the action of syncing a MultiClusterService across member clusters.
	EventActionSyncService = "SyncService"
	// EventActionDispatchEndpointSlice indicates the action of dispatching EndpointSlices for a MultiClusterService.
	EventActionDispatchEndpointSlice = "DispatchEndpointSlice"
	// EventActionResolveCluster indicates the action of resolving the member clusters configured on a MultiClusterService.
	EventActionResolveCluster = "ResolveCluster"
	// EventActionValidateClusterAPI indicates the action of validating that member clusters expose the APIs required by a MultiClusterService.
	EventActionValidateClusterAPI = "ValidateClusterAPI"

	// EventActionStartCronFederatedHPARule indicates the action of starting the cron executor for a CronFederatedHPA rule.
	EventActionStartCronFederatedHPARule = "StartCronFederatedHPARule"
	// EventActionUpdateCronFederatedHPA indicates the action of updating a CronFederatedHPA object.
	EventActionUpdateCronFederatedHPA = "UpdateCronFederatedHPA"
	// EventActionScaleCronFederatedHPA indicates the action of scaling the target referenced by a CronFederatedHPA.
	EventActionScaleCronFederatedHPA = "ScaleCronFederatedHPA"
	// EventActionUpdateCronFederatedHPAStatus indicates the action of updating a CronFederatedHPA's status with execution history.
	EventActionUpdateCronFederatedHPAStatus = "UpdateCronFederatedHPAStatus"
)
