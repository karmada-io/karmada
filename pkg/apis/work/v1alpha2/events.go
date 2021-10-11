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
)
