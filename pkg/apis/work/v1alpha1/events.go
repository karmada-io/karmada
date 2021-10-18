package v1alpha1

// Define events for work objects.
const (
	// EventReasonSyncWorkFailed indicates that Sync work failed.
	EventReasonSyncWorkFailed = "SyncFailed"

	// EventReasonSyncWorkSucceed indicates that Sync work succeed.
	EventReasonSyncWorkSucceed = "SyncSucceed"

	// EventReasonApplyWorkFailed indicates that apply work failed.
	EventReasonApplyWorkFailed = "ApplyFailed"

	// EventReasonApplyWorkSucceed indicates that apply work succeed.
	EventReasonApplyWorkSucceed = "ApplySucceed"
)
