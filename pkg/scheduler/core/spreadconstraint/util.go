/*
Copyright 2022 The Karmada Authors.

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

package spreadconstraint

import (
	"sort"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

const (
	// InvalidClusterID indicate a invalid cluster
	InvalidClusterID = -1
	// InvalidReplicas indicate that don't care about the available resource
	InvalidReplicas = -1
)

// IsSpreadConstraintExisted judge if the specific field is existed in the spread constraints
func IsSpreadConstraintExisted(spreadConstraints []policyv1alpha1.SpreadConstraint, field policyv1alpha1.SpreadFieldValue) bool {
	for _, spreadConstraint := range spreadConstraints {
		if spreadConstraint.SpreadByField == field {
			return true
		}
	}

	return false
}

func sortClusters(infos []ClusterDetailInfo, compareFunctions ...func(*ClusterDetailInfo, *ClusterDetailInfo) *bool) {
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Score != infos[j].Score {
			return infos[i].Score > infos[j].Score
		}

		for _, compareFunc := range compareFunctions {
			if result := compareFunc(&infos[i], &infos[j]); result != nil {
				return *result
			}
		}

		return infos[i].Name < infos[j].Name
	})
}
