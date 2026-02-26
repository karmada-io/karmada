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
	"sort"

	"k8s.io/klog/v2"
)

type dfsPath[T Group] struct {
	*BasePath[T]
	kind string
}

func (path *dfsPath[T]) enqueue(node T) {
	path.Nodes = append(path.Nodes, node)
}

func (path *dfsPath[T]) popLast() {
	path.Nodes = path.Nodes[:len(path.Nodes)-1]
}

func (path *dfsPath[T]) clone() *dfsPath[T] {
	pathCopy := new(dfsPath[T])
	*pathCopy = *path
	pathCopy.BasePath = path.BasePath.Clone()
	return pathCopy
}

func (path *dfsPath[T]) next() *dfsPath[T] {
	nextPath := path.clone()
	for _, node := range path.Nodes {
		nextPath.Number += node.GetNumber(path.kind)
	}
	nextPath.Score = CalculateComprehensiveScore(nextPath.Nodes)
	nextPath.GenerateID()
	return nextPath
}

// selectGroupsByDFS selects groups using DFS algorithm to find feasible paths
// and multiple dimensions to prioritize the best path.
func selectGroupsByDFS[T Group](groups []T, ctx *ConstraintContext) []T {
	feasiblePaths := findFeasiblePaths(groups, ctx.selfConstraint, &ctx.subConstraints[0])
	if len(feasiblePaths) == 0 {
		return nil
	}

	path := prioritizePaths(feasiblePaths)
	if path != nil {
		return path.GetNodes()
	}
	return nil
}

// findFeasiblePaths finds all feasible path based on DFS.
// We give an example of a tree diagram for easy understanding, assuming: target=7, groups: [2,3,6,7].
// Note: groups is only briefly represented by the value.
//
//	                        +---------------------------------+
//	                        |            target=7             |
//	                        +---------------+-------------+---+
//	                       /                |             |    \
//	                     2/                3|            6|     \7
//	                     /                  |             |      \
//	                +---+                 +-+-+         +-+-+     +---+
//	                | 5 |                 | 4 |         | 1 |     | 0 |
//	                +-+-+                 +---+         +-+-+     +---+
//	               /  |  \               /     \          |
//	             3/  6|   \7           6/       \7       7|
//	             /    |    \           /         \        |
//	        +---+   +-+-+   +---+ +---+           +---+ +-+-+
//	        | 2 |   |-1 |   |-2 | |-2 |           |-3 | |-6 |
//	        +---+   +---+   +---+ +---+           +---+ +---+
//	       /     \
//	     6/       \7
//	     /         \
//	+---+           +---+
//	|-4 |           |-5 |
//	+---+           +---+
//
// The path from the root path to all leaf groups whose value is less than or equal to 0 is a feasible combination.
// minGroups and maxGroups are used to limit the length of the path. Therefore, we can exclude some
// combinations whose path is too long or too short during the search process.
// If we set minGroups=2 and maxGroups=2, so the feasible paths is: [2,6] [2,7] [3,6] [3,7] [6,7]
func findFeasiblePaths[T Group](groups []T, selfConstraint, constraint *Constraint) (paths []Path[T]) {
	defer klog.V(4).Infof("Find feasible paths: %v", paths)

	if len(groups) > 1 {
		sort.Slice(groups, func(i, j int) bool {
			if groups[i].GetNumber(constraint.Kind) != groups[j].GetNumber(constraint.Kind) {
				return groups[i].GetNumber(constraint.Kind) < groups[j].GetNumber(constraint.Kind)
			}

			if groups[i].GetScore() != groups[j].GetScore() {
				return groups[i].GetScore() > groups[j].GetScore()
			}

			return groups[i].GetName() < groups[j].GetName()
		})
	}

	rootPath := &dfsPath[T]{
		kind:     constraint.Kind,
		BasePath: new(BasePath[T]),
	}
	var dfsFunc func(int64, int)
	dfsFunc = func(sum int64, begin int) {
		if sum >= constraint.MinGroups && selfConstraint.Validate(int64(len(rootPath.Nodes))) {
			paths = append(paths, rootPath.next())
			return
		}

		// pruning makes DFS faster
		if int64(len(rootPath.Nodes)) >= selfConstraint.MaxGroups {
			return
		}

		for i := begin; i < len(groups); i++ {
			g := groups[i]
			sum += g.GetNumber(constraint.Kind)
			rootPath.enqueue(g)
			dfsFunc(sum, i+1)
			sum -= g.GetNumber(constraint.Kind)
			rootPath.popLast()
		}
	}
	dfsFunc(0, 0)
	return
}

// prioritizePaths selects a best path from feasible paths based on a multi-dimensional prioritization strategy.
// That's: score > number > id(just for predictable result).
// But for the existence of sub-paths, we should give priority to sub-paths.
// For example:
// G1 -> G2 -> G3
// G1 -> G2
// G1 -> G3
// The best path is [G1 -> G2 -> G3], but [G1 -> G2] is its sub-paths. So we should choose [G1 -> G2].
func prioritizePaths[T Group](paths []Path[T], compareFunctions ...func(Path[T], Path[T]) *bool) Path[T] {
	if len(paths) == 1 {
		return paths[0]
	}

	sort.Slice(paths, func(i, j int) bool {
		pi := paths[i]
		pj := paths[j]
		if pi.GetScore() != pj.GetScore() {
			return pi.GetScore() > pj.GetScore()
		}

		if pi.GetNumber() != pj.GetNumber() {
			return pi.GetNumber() > pj.GetNumber()
		}

		if len(pi.GetNodes()) != len(pj.GetNodes()) {
			return len(pi.GetNodes()) < len(pj.GetNodes())
		}

		for _, compareFunc := range compareFunctions {
			if result := compareFunc(pi, pj); result != nil {
				return *result
			}
		}

		return pi.GetID() < pj.GetID()
	})

	var recommendedPath Path[T]
	for _, path := range paths {
		if recommendedPath == nil || recommendedPath.MatchSubPath(path) {
			recommendedPath = path
		}
	}

	return recommendedPath
}
