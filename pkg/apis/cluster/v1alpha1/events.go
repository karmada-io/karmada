package v1alpha1

// Define events for execute space objects.
const (
	// EventReasonCreateExecutionSpaceFailed indicates that create execution space failed.
	EventReasonCreateExecutionSpaceFailed = "CreateExecutionSpaceFailed"
	// EventReasonRemoveExecutionSpaceFailed indicates that remove execution space failed.
	EventReasonRemoveExecutionSpaceFailed = "RemoveExecutionSpaceFailed"
	// EventReasonTaintClusterByConditionFailed indicates that taint cluster by condition
	EventReasonTaintClusterByConditionFailed = "TaintClusterByCondition"
)
