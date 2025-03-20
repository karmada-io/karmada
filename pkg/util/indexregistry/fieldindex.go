/*
Copyright 2025 The Karmada Authors.

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

package indexregistry

const (
	// WorkIndexByResourceBindingID is the index name for Works that are associated with a ResourceBinding ID.
	// This index allows efficient lookup of Works by their `metadata.labels.<ResourceBindingPermanentIDLabel>`,
	// which references the ID of the bound ResourceBinding object.
	WorkIndexByResourceBindingID = "WorkIndexByResourceBindingID"

	// WorkIndexByClusterResourceBindingID is the index name for Works associated with a ClusterResourceBinding ID.
	// The index is built using `metadata.labels.<ClusterResourceBindingPermanentIDLabel>` to enable fast queries
	// of Works linked to specific ClusterResourceBinding objects across all namespaces.
	WorkIndexByClusterResourceBindingID = "WorkIndexByClusterResourceBindingID"
)
