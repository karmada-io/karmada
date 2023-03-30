package spreadconstraint

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/klog/v2"
)

// GroupInfo indicate the group info.
type GroupInfo struct {
	name   string
	value  int
	weight int64
}

type dfsPath struct {
	id     int
	groups []*GroupInfo
	weight int64
	value  int
}

func (path *dfsPath) next() *dfsPath {
	path.id++
	result := new(dfsPath)
	*result = *path
	if path.groups != nil {
		result.groups = make([]*GroupInfo, len(path.groups))
		copy(result.groups, path.groups)
	}
	for _, group := range result.groups {
		result.weight += group.weight
		result.value += group.value
	}
	result.sortGroups()
	return result
}

func (path *dfsPath) enqueue(group *GroupInfo) {
	path.groups = append(path.groups, group)
}

func (path *dfsPath) popLast() {
	path.groups = path.groups[:path.length()-1]
}

func (path *dfsPath) length() int {
	return len(path.groups)
}

func (path *dfsPath) sortGroups() {
	sort.Slice(path.groups, func(i, j int) bool {
		if path.groups[i].weight != path.groups[j].weight {
			return path.groups[i].weight > path.groups[j].weight
		}

		return path.groups[i].name < path.groups[j].name
	})
}

func (path *dfsPath) matchSubPath(subPath *dfsPath) bool {
	if subPath.length() >= path.length() {
		return false
	}

	for i, group := range subPath.groups {
		if path.groups[i].name != group.name {
			return false
		}
	}

	return true
}

func (path *dfsPath) String() string {
	var groupNames []string
	for _, group := range path.groups {
		groupNames = append(groupNames, group.name)
	}
	return fmt.Sprintf("[%s]", strings.Join(groupNames, "->"))
}

// selectGroups select best groups in all groups.
func selectGroups(groups []*GroupInfo, minConstraint, maxConstraint, target int) []*GroupInfo {
	if len(groups) == 0 {
		return nil
	}

	feasiblePaths := findFeasiblePaths(groups, minConstraint, maxConstraint, target)
	if len(feasiblePaths) == 0 {
		return nil
	}
	klog.V(4).Infof("find feasible paths: %v", feasiblePaths)

	return prioritizePaths(feasiblePaths).groups
}

// findFeasiblePaths find all feasible path based on DFS.
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
// The path from the root path to all leaf nodes whose value is less than or equal to 0 is a feasible combination.
// minConstraint and maxConstraint are used to limit the length of the path. Therefore, we can exclude some
// combinations whose path is too long or too short during the search process.
// If we set minConstraint=2 and maxConstraint=2, so the feasible paths is: [2,6] [2,7] [3,6] [3,7] [6,7]
func findFeasiblePaths(groups []*GroupInfo, minConstraint, maxConstraint, target int) (paths []*dfsPath) {
	if len(groups) > 1 {
		sort.Slice(groups, func(i, j int) bool {
			if groups[i].value != groups[j].value {
				return groups[i].value < groups[j].value
			}

			if groups[i].weight != groups[j].weight {
				return groups[i].weight > groups[j].weight
			}

			return groups[i].name < groups[j].name
		})
	}

	rootPath := new(dfsPath)
	var dfsFunc func(int, int)
	dfsFunc = func(sum, begin int) {
		if sum >= target && rootPath.length() >= minConstraint && rootPath.length() <= maxConstraint {
			paths = append(paths, rootPath.next())
			return
		}

		// pruning makes DFS faster
		if rootPath.length() >= maxConstraint {
			return
		}

		for i := begin; i < len(groups); i++ {
			sum += groups[i].value
			rootPath.enqueue(groups[i])
			dfsFunc(sum, i+1)
			sum -= groups[i].value
			rootPath.popLast()
		}
	}
	dfsFunc(0, 0)
	return
}

// prioritizePaths select a best path from feasible paths based on a multi-dimensional prioritization strategy.
// That's: weight > value > id(just for predictable result).
// But for the existence of subpaths, we should give priority to subpaths.
// For example:
// G1 -> G2 -> G3
// G1 -> G2
// G1 -> G3
// The best path is [G1 -> G2 -> G3], but [G1 -> G2] is its subpath. So we should choose [G1 -> G2].
func prioritizePaths(feasiblePaths []*dfsPath) *dfsPath {
	if len(feasiblePaths) == 1 {
		return feasiblePaths[0]
	}

	sort.Slice(feasiblePaths, func(i, j int) bool {
		if feasiblePaths[i].weight != feasiblePaths[j].weight {
			return feasiblePaths[i].weight > feasiblePaths[j].weight
		}

		if feasiblePaths[i].value != feasiblePaths[j].value {
			return feasiblePaths[i].value > feasiblePaths[j].value
		}

		return feasiblePaths[i].id < feasiblePaths[j].id
	})

	finalPath := feasiblePaths[0]
	for i := 1; i < len(feasiblePaths); i++ {
		if finalPath.matchSubPath(feasiblePaths[i]) {
			finalPath = feasiblePaths[i]
		}
	}
	return finalPath
}
