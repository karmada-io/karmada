/*
Copyright 2023 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
