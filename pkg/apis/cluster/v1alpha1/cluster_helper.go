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

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// String returns a well-formatted string for the Cluster object.
func (c *Cluster) String() string {
	return c.Name
}

// APIEnablementStatus is the status of the specific API on the cluster.
type APIEnablementStatus string

const (
	// APIEnabled means the cluster supports the specified API.
	APIEnabled APIEnablementStatus = "APIEnabled"
	// APIDisabled means the cluster does not support the specified API.
	APIDisabled APIEnablementStatus = "APIDisabled"
	// APIUnknown means it is unknown whether the cluster supports the specified API.
	APIUnknown APIEnablementStatus = "APIUnknown"
)

// APIEnablement checks if the target API (or CRD) referenced by gvk has been installed in the cluster.
// The check takes the CompleteAPIEnablements condition into account. If the CompleteAPIEnablements condition indicates
// the current APIEnablements is Partial, it returns APIEnabled if the gvk is found in the list; otherwise, the status is considered APIUnknown.
// This means that when the APIEnablements is Partial and the gvk is not present, we cannot definitively say the API is disabled.
func (c *Cluster) APIEnablement(gvk schema.GroupVersionKind) APIEnablementStatus {
	targetGroupVersion := gvk.GroupVersion().String()
	for _, apiEnablement := range c.Status.APIEnablements {
		if apiEnablement.GroupVersion != targetGroupVersion {
			continue
		}
		for _, resource := range apiEnablement.Resources {
			if resource.Kind != gvk.Kind {
				continue
			}
			return APIEnabled
		}
	}

	// If we have the complete APIEnablements list for the cluster,
	// we can confidently determine that the API is disabled if it was not found above.
	if meta.IsStatusConditionPresentAndEqual(c.Status.Conditions, ClusterConditionCompleteAPIEnablements, metav1.ConditionTrue) {
		return APIDisabled
	}

	return APIUnknown
}
