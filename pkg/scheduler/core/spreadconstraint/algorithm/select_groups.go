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

import (
	"math"
)

// SelectGroups select best groups in all groups by constraints.
// Depending on the number of current constraints, an appropriate selection algorithm will be used.
func SelectGroups[T Group](selectByKind string, groups []T, ctx *ConstraintContext) []T {
	if len(groups) == 0 || len(ctx.Constraints) == 0 {
		return groups
	}

	setDefaultConstraints(ctx, selectByKind)

	switch len(ctx.subConstraints) {
	case 0:
		return selectGroupsByScore(groups, ctx)
	case 1:
		return selectGroupsByDFS(groups, ctx)
	default:
		return selectGroupsByMultiDFS(groups, ctx)
	}
}

// setDefaultConstraints set selfConstraint and subConstraints by default.
func setDefaultConstraints(ctx *ConstraintContext, selectByKind string) {
	ctx.subConstraints = make([]Constraint, 0, len(ctx.Constraints))

	for i, constraint := range ctx.Constraints {
		if constraint.Kind == selectByKind {
			ctx.selfConstraint = &ctx.Constraints[i]
		} else {
			ctx.subConstraints = append(ctx.subConstraints, constraint)
		}
	}

	// set selfConstraint unbounded constraint if not found.
	if ctx.selfConstraint == nil {
		ctx.selfConstraint = &Constraint{
			Kind:      selectByKind,
			MaxGroups: math.MaxInt64,
			MinGroups: math.MinInt64,
		}
	}
}
