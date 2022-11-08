package v1alpha1

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
)
