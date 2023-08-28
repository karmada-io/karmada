package algorithm

import "sort"

func selectGroupsByScore[T Group](groups []T, ctx *ConstraintContext) []T {
	groupsNum := int64(len(groups))
	if groupsNum < ctx.selfConstraint.MinGroups {
		return nil
	} else if groupsNum <= ctx.selfConstraint.MaxGroups {
		return groups
	} else if groupsNum > 1 {
		sort.Slice(groups, func(i, j int) bool {
			if groups[i].GetScore() != groups[j].GetScore() {
				return groups[i].GetScore() > groups[j].GetScore()
			}

			return groups[i].GetName() < groups[j].GetName()
		})
	}

	return groups[:ctx.selfConstraint.MaxGroups]
}
