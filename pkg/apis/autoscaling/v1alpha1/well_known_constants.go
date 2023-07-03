package v1alpha1

const (
	// FederatedHPAKind is the kind of FederatedHPA in group autoscaling.karmada.io
	FederatedHPAKind = "FederatedHPA"

	// QuerySourceAnnotationKey is the annotation used in karmada-metrics-adapter to
	// record the query source cluster
	QuerySourceAnnotationKey = "resource.karmada.io/query-from-cluster"
)
