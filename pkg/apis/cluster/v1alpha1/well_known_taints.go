package v1alpha1

const (
	// TaintClusterUnscheduler will be added when cluster becomes unschedulable
	// and removed when cluster becomes scheduable.
	TaintClusterUnscheduler = "cluster.karmada.io/unschedulable"

	// TaintClusterNotReady will be added when cluster is not ready
	// and removed when cluster becomes ready.
	TaintClusterNotReady = "cluster.karmada.io/not-ready"
	// TaintClusterUnreachable will be added when cluster becomes unreachable
	// (corresponding to ClusterConditionReady status ConditionUnknown)
	// and removed when cluster becomes reachable (ClusterConditionReady status ConditionTrue).
	TaintClusterUnreachable = "cluster.karmada.io/unreachable"
)
