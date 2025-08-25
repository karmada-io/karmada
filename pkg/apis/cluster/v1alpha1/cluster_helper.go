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

// IsAPIEnabledAndTrusted checks if the specific GVK is enabled on the cluster
// and if the check result is trusted.
//
// If the API is enabled and the result is trusted, it is recommended to
// consider that the API is enabled.
// If the API is not enabled and the result is trusted, it is recommended to
// consider that the API is not enabled.
// If the API is enabled and the result is untrusted, this case should
// never happen.
// If the API is not enabled and the result is untrusted, it is recommended to
// consider that the API is enabled.
func (c *Cluster) IsAPIEnabledAndTrusted(gvk schema.GroupVersionKind) (enabled bool, trusted bool) {
	enabled = false
	targetGroupVersion := gvk.GroupVersion().String()
	for _, apiEnablement := range c.Status.APIEnablements {
		if apiEnablement.GroupVersion != targetGroupVersion {
			continue
		}

		for _, resource := range apiEnablement.Resources {
			if resource.Kind == gvk.Kind {
				enabled = true
				break
			}
		}

		if enabled {
			break
		}
	}

	// if the API is enabled, it is always trusted.
	if enabled {
		return true, true
	}

	apiCompleted := meta.FindStatusCondition(c.Status.Conditions, ClusterConditionCompleteAPIEnablements)
	if apiCompleted == nil {
		// never reach here, because cluster-status-controller will always
		// set this condition with the api enablements.
		return false, false
	}

	return false, apiCompleted.Status == metav1.ConditionTrue
}
