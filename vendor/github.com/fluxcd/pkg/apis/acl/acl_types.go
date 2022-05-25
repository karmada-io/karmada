/*
Copyright 2021 The Flux authors

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

package acl

// AccessFrom defines an ACL for allowing cross-namespace references to a source object
// based on the caller's namespace labels.
type AccessFrom struct {
	// NamespaceSelectors is the list of namespace selectors to which this ACL applies.
	// Items in this list are evaluated using a logical OR operation.
	// +required
	NamespaceSelectors []NamespaceSelector `json:"namespaceSelectors"`
}

// NamespaceSelector selects the namespaces to which this ACL applies.
// An empty map of MatchLabels matches all namespaces in a cluster.
type NamespaceSelector struct {
	// MatchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
	// map is equivalent to an element of matchExpressions, whose key field is "key", the
	// operator is "In", and the values array contains only "value". The requirements are ANDed.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}
