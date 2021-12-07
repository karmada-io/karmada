package v1alpha1

const (
	// TaintClusterUnscheduler will be added when cluster becomes unschedulable
	// and removed when node becomes scheduable.
	TaintClusterUnscheduler = "cluster.karmada.io/unschedulable"
)
