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
