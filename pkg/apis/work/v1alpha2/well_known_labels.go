/*
Copyright The Karmada Authors.

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

package v1alpha2

const (
	// ResourceBindingReferenceKey is the key of ResourceBinding object.
	// It is usually a unique hash value of ResourceBinding object's namespace and name, intended to be added to the Work object.
	// It will be used to retrieve all Works objects that derived from a specific ResourceBinding object.
	ResourceBindingReferenceKey = "resourcebinding.karmada.io/key"

	// ClusterResourceBindingReferenceKey is the key of ClusterResourceBinding object.
	// It is usually a unique hash value of ClusterResourceBinding object's namespace and name, intended to be added to the Work object.
	// It will be used to retrieve all Works objects that derived by a specific ClusterResourceBinding object.
	ClusterResourceBindingReferenceKey = "clusterresourcebinding.karmada.io/key"

	// ResourceBindingNamespaceLabel is added to objects to specify associated ResourceBinding's namespace.
	ResourceBindingNamespaceLabel = "resourcebinding.karmada.io/namespace"

	// ResourceBindingNameLabel is added to objects to specify associated ResourceBinding's name.
	ResourceBindingNameLabel = "resourcebinding.karmada.io/name"

	// ClusterResourceBindingLabel is added to objects to specify associated ClusterResourceBinding.
	ClusterResourceBindingLabel = "clusterresourcebinding.karmada.io/name"

	// WorkNamespaceLabel is added to objects to specify associated Work's namespace.
	WorkNamespaceLabel = "work.karmada.io/namespace"

	// WorkNameLabel is added to objects to specify associated Work's name.
	WorkNameLabel = "work.karmada.io/name"
)
