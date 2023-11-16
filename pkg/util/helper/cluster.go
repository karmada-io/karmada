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

package helper

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

// IsAPIEnabled checks if target API (or CRD) referencing by groupVersion and kind has been installed.
func IsAPIEnabled(APIEnablements []clusterv1alpha1.APIEnablement, groupVersion string, kind string) bool {
	for _, APIEnablement := range APIEnablements {
		if APIEnablement.GroupVersion != groupVersion {
			continue
		}

		for _, resource := range APIEnablement.Resources {
			if resource.Kind != kind {
				continue
			}
			return true
		}
	}

	return false
}
