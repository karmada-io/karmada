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
	"strings"
)

// Group indicates the abstract group interface.
// An implementation of Group can be selected by constraints in this package.
type Group interface {
	Scorer
	GetName() string
	GetNumber(string) int64
}

// Constraint indicates how to select groups.
type Constraint struct {
	Kind      string
	Weight    int8
	MinGroups int64
	MaxGroups int64
}

// Validate checks if value is in [MinGroups, MaxGroups].
func (c *Constraint) Validate(value int64) bool {
	return c.MaxGroups >= value && c.MinGroups <= value
}

// ConstraintContext contains all necessary constraints.
type ConstraintContext struct {
	Constraints    []Constraint
	selfConstraint *Constraint
	subConstraints []Constraint
}

// Path indicates the abstract path interface.
type Path[N Group] interface {
	Scorer
	GetID() string
	GetNodes() []N
	GetNumber() int64
	MatchSubPath(Path[N]) bool
}

// BasePath is a generic implementation of Path.
type BasePath[N Group] struct {
	ID     string
	Nodes  []N
	Score  int64
	Number int64
}

func (path *BasePath[T]) Clone() *BasePath[T] {
	pathCopy := new(BasePath[T])
	*pathCopy = *path
	if path.Nodes != nil {
		pathCopy.Nodes = make([]T, len(path.Nodes))
		copy(pathCopy.Nodes, path.Nodes)
	}
	return pathCopy
}

func (path *BasePath[T]) GetID() string {
	return path.ID
}

func (path *BasePath[T]) GetNodes() []T {
	return path.Nodes
}

func (path *BasePath[T]) GetScore() int64 {
	return path.Score
}

func (path *BasePath[T]) GetNumber() int64 {
	return path.Number
}

// MatchSubPath matches path's sub-path, and returns true if its sub-path exists,
// otherwise returns false.
func (path *BasePath[T]) MatchSubPath(subPath Path[T]) bool {
	if len(subPath.GetNodes()) >= len(path.Nodes) {
		return false
	}

	var pathIndex int
	for _, spn := range subPath.GetNodes() {
		if pathIndex == len(path.Nodes) {
			return false
		}
		for i := pathIndex; i < len(path.Nodes); i++ {
			pn := path.Nodes[i]
			if pn.GetName() == spn.GetName() {
				pathIndex = i + 1
				break
			}
			if pn.GetScore() != spn.GetScore() {
				return false
			}
		}
	}

	return true
}

func (path *BasePath[T]) String() string {
	return path.ID
}

// GenerateID builds path's unique id which is concatenated by the
// sorted names of all nodes on the path. It looks like G1->G2->G3.
func (path *BasePath[T]) GenerateID() {
	sort.Slice(path.Nodes, func(i, j int) bool {
		if path.Nodes[i].GetScore() != path.Nodes[j].GetScore() {
			return path.Nodes[i].GetScore() > path.Nodes[j].GetScore()
		}

		return path.Nodes[i].GetName() < path.Nodes[j].GetName()
	})
	nodeNames := make([]string, 0, len(path.Nodes))
	for _, node := range path.Nodes {
		nodeNames = append(nodeNames, node.GetName())
	}
	path.ID = strings.Join(nodeNames, "->")
}
