package v1alpha2

// Define events for ResourceBinding and ClusterResourceBinding objects.
const (
	// EventReasonCleanupWorkFailed indicates that Cleanup work failed.
	EventReasonCleanupWorkFailed = "CleanupWorkFailed"
	// EventReasonSyncWorkFailed indicates that Sync work failed.
	EventReasonSyncWorkFailed = "SyncWorkFailed"
	// EventReasonSyncWorkSucceed indicates that Sync work succeed.
	EventReasonSyncWorkSucceed = "SyncWorkSucceed"
	// EventReasonAggregateStatusFailed indicates that aggregate status failed.
	EventReasonAggregateStatusFailed = "AggregateStatusFailed"
	// EventReasonAggregateStatusSucceed indicates that aggregate status succeed.
	EventReasonAggregateStatusSucceed = "AggregateStatusSucceed"
	// EventReasonApplyPolicyFailed indicates that apply policy for resource failed.
	EventReasonApplyPolicyFailed = "ApplyPolicyFailed"
	// EventReasonApplyPolicySucceed indicates that apply policy for resource succeed.
	EventReasonApplyPolicySucceed = "ApplyPolicySucceed"
	// EventReasonApplyOverridePolicyFailed indicates that apply override policy failed.
	EventReasonApplyOverridePolicyFailed = "ApplyOverridePolicyFailed"
	// EventReasonApplyOverridePolicySucceed indicates that apply override policy succeed.
	EventReasonApplyOverridePolicySucceed = "ApplyOverridePolicySucceed"
	// EventReasonScheduleBindingFailed indicates that schedule binding failed.
	EventReasonScheduleBindingFailed = "ScheduleBindingFailed"
	// EventReasonScheduleBindingSucceed indicates that schedule binding succeed.
	EventReasonScheduleBindingSucceed = "ScheduleBindingSucceed"
	// EventReasonDescheduleBindingFailed indicates that deschedule binding failed.
	EventReasonDescheduleBindingFailed = "DescheduleBindingFailed"
	// EventReasonDescheduleBindingSucceed indicates that deschedule binding succeed.
	EventReasonDescheduleBindingSucceed = "DescheduleBindingSucceed"
)
