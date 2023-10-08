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
)

// Define events for ResourceBinding and ClusterResourceBinding objects.
const (
	// EventReasonCleanupWorkFailed indicates that Cleanup work failed.
	EventReasonCleanupWorkFailed = "CleanupWorkFailed"
	// EventReasonSyncScheduleResultToDependenciesSucceed indicates sync schedule result to attached bindings succeed.
	EventReasonSyncScheduleResultToDependenciesSucceed = "SyncScheduleResultToDependenciesSucceed"
	// EventReasonSyncScheduleResultToDependenciesFailed indicates sync schedule result to attached bindings failed.
	EventReasonSyncScheduleResultToDependenciesFailed = "SyncScheduleResultToDependenciesFailed"
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
