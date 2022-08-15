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
	// TaintClusterTerminating will be added when cluster is terminating.
	TaintClusterTerminating = "cluster.karmada.io/terminating"

	// CacheSourceAnnotationKey is the annotation that added to a resource to
	// represent which cluster it cached from.
	CacheSourceAnnotationKey = "resource.karmada.io/cached-from-cluster"
)
