package v1alpha1

const (
	// FederatedHPAKind is the kind of FederatedHPA in group autoscaling.karmada.io
	FederatedHPAKind = "FederatedHPA"

	// QuerySourceAnnotationKey is the annotation used in karmada-metrics-adapter to
	// record the query source cluster
	QuerySourceAnnotationKey = "resource.karmada.io/query-from-cluster"

	// ResourceSingularFederatedHPA is singular name of FederatedHPA.
	ResourceSingularFederatedHPA = "federatedhpa"
	// ResourcePluralFederatedHPA is plural name of FederatedHPA.
	ResourcePluralFederatedHPA = "federatedhpas"
	// ResourceNamespaceScopedFederatedHPA is the scope of the FederatedHPA
	ResourceNamespaceScopedFederatedHPA = true

	// ResourceKindCronFederatedHPA is kind name of CronFederatedHPA.
	ResourceKindCronFederatedHPA = "CronFederatedHPA"
	// ResourceSingularCronFederatedHPA is singular name of CronFederatedHPA.
	ResourceSingularCronFederatedHPA = "cronfederatedhpa"
	// ResourcePluralCronFederatedHPA is plural name of CronFederatedHPA.
	ResourcePluralCronFederatedHPA = "cronfederatedhpas"
	// ResourceNamespaceScopedCronFederatedHPA is the scope of the CronFederatedHPA
	ResourceNamespaceScopedCronFederatedHPA = true
)
