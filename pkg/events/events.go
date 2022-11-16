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
	// EventReasonTaintClusterByConditionFailed indicates that taint cluster by condition failed.
	EventReasonTaintClusterByConditionFailed = "TaintClusterByConditionFailed"
	// EventReasonRemoveTargetClusterFailed indicates that failed to remove target cluster from binding.
	EventReasonRemoveTargetClusterFailed = "RemoveTargetClusterFailed"
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
)
