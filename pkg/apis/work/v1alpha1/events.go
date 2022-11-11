package v1alpha1

// Define events for work objects.
const (
	// EventReasonSyncWorkFailed indicates that Sync work failed.
	EventReasonSyncWorkFailed = "SyncFailed"

	// EventReasonSyncWorkSucceed indicates that Sync work succeed.
	EventReasonSyncWorkSucceed = "SyncSucceed"

	// EventReasonReflectStatusSucceed indicates that reflect status to work succeed.
	EventReasonReflectStatusSucceed = "ReflectStatusSucceed"

	// EventReasonReflectStatusFailed indicates that reflect status to work failed.
	EventReasonReflectStatusFailed = "ReflectStatusFailed"

	// EventReasonInterpretHealthSucceed indicates that interpret health succeed.
	EventReasonInterpretHealthSucceed = "InterpretHealthSucceed"

	// EventReasonInterpretHealthFailed indicates that interpret health failed.
	EventReasonInterpretHealthFailed = "InterpretHealthFailed"
)
