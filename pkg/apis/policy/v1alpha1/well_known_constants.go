/*
Copyright 2021 The Karmada Authors.

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

// The well-known label key constant.
const (
	// PropagationPolicyPermanentIDLabel is the identifier of a PropagationPolicy object.
	// Karmada generates a unique identifier, such as metadata.UUID, for each PropagationPolicy object.
	// This identifier will be used as a label selector to locate corresponding resources, such as ResourceBinding.
	// The reason for generating a new unique identifier instead of simply using metadata.UUID is because:
	// In backup scenarios, when applying the backup resource manifest in a new cluster, the UUID may change.
	PropagationPolicyPermanentIDLabel = "propagationpolicy.karmada.io/permanent-id"

	// ClusterPropagationPolicyPermanentIDLabel is the identifier of a ClusterPropagationPolicy object.
	// Karmada generates a unique identifier, such as metadata.UUID, for each ClusterPropagationPolicy object.
	// This identifier will be used as a label selector to locate corresponding resources, such as ResourceBinding.
	// The reason for generating a new unique identifier instead of simply using metadata.UUID is because:
	// In backup scenarios, when applying the backup resource manifest in a new cluster, the UUID may change.
	ClusterPropagationPolicyPermanentIDLabel = "clusterpropagationpolicy.karmada.io/permanent-id"

	// NamespaceSkipAutoPropagationLabel is added to namespace objects to indicate if
	// the namespace should be skipped from propagating by the namespace controller.
	// For example, a namespace with the following label will be skipped:
	//   labels:
	//     namespace.karmada.io/skip-auto-propagation: "true"
	//
	// NOTE: If create a ns without this label, then patch it with this label, the ns will not be
	// synced to new member clusters, but old member clusters still have it.
	NamespaceSkipAutoPropagationLabel = "namespace.karmada.io/skip-auto-propagation"
)

// The well-known annotation key constant.
const (
	// PropagationPolicyNamespaceAnnotation is added to objects to specify associated PropagationPolicy namespace.
	PropagationPolicyNamespaceAnnotation = "propagationpolicy.karmada.io/namespace"

	// PropagationPolicyNameAnnotation is added to objects to specify associated PropagationPolicy name.
	PropagationPolicyNameAnnotation = "propagationpolicy.karmada.io/name"

	// ClusterPropagationPolicyAnnotation is added to objects to specify associated ClusterPropagationPolicy name.
	ClusterPropagationPolicyAnnotation = "clusterpropagationpolicy.karmada.io/name"
)
