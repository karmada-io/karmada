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
	// MultiClusterServicePermanentIDLabel is the identifier of a MultiClusterService object.
	// Karmada generates a unique identifier, such as metadata.UUID, for each MultiClusterService object.
	// This identifier will be used as a label selector to locate corresponding work of service.
	// The reason for generating a new unique identifier instead of simply using metadata.UUID is because:
	// In backup scenarios, when applying the backup resource manifest in a new cluster, the UUID may change.
	MultiClusterServicePermanentIDLabel = "multiclusterservice.karmada.io/permanent-id"

	// MultiClusterServiceNameAnnotation is the name of a MultiClusterService object.
	// This annotation will be added to the resource template and ResourceBinding
	MultiClusterServiceNameAnnotation = "multiclusterservice.karmada.io/name"

	// MultiClusterServiceNamespaceAnnotation is the namespace of a MultiClusterService object.
	// This annotation will be added to the resource template and ResourceBinding
	MultiClusterServiceNamespaceAnnotation = "multiclusterservice.karmada.io/namespace"
)
