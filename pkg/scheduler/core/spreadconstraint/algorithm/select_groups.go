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
