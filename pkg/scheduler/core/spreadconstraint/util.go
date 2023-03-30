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
