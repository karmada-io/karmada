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
	"sync"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/util/sets"
)

// multiDFSPath extends Path based on multi-DFS and merges
// paths of different dimensions into one path.
type multiDFSPath[T Group] struct {
	*BasePath[T]
	nodeSet sets.Set[string]
}

func (path *multiDFSPath[T]) clone() *multiDFSPath[T] {
	pathCopy := new(multiDFSPath[T])
	pathCopy.BasePath = path.BasePath.Clone()
	if path.nodeSet != nil {
		pathCopy.nodeSet = path.nodeSet.Clone()
	}

	return pathCopy
}

// merge merges Path's nodes into multiDFSPath.
func (path *multiDFSPath[T]) merge(p Path[T]) {
	for _, node := range p.GetNodes() {
		if !path.nodeSet.Has(node.GetName()) {
			path.nodeSet.Insert(node.GetName())
			path.Nodes = append(path.Nodes, node)
		}
	}

	path.GenerateID()
}

// multiDFSPathIndexer is the store implementation of multiDFSPath
// based on the Path's id index.
type multiDFSPathIndexer[T Group] map[string]*multiDFSPath[T]

// calculatePaths calculates a comprehensive number and score of all multiDFSPath in multiDFSPathIndexer.
func (indexer multiDFSPathIndexer[T]) calculatePaths(ctx *ConstraintContext) (paths []Path[T]) {
	for _, mp := range indexer {
		if !ctx.selfConstraint.Validate(int64(len(mp.Nodes))) {
			continue
		}

		for _, constraint := range ctx.subConstraints {
			var number int64
			for _, node := range mp.Nodes {
				number += node.GetNumber(constraint.Kind)
			}

			// The following algorithm variants are derived from the Snowflake Algorithm.
			// More info: https://github.com/twitter-archive/snowflake/tree/snowflake-2010
			//
			// We introduce the concept of weight to measure the importance of different kinds(zone, cluster...).
			// Weight is up to 64 bits in size, it can be defined as the following structure:
			//  0  1111111111111111111111 111111111111111111111 11111111111111111111
			//  1           22                     21                  20
			// sign        cluster                zone                region
			// For example:
			// Group A has 3 clusters and 2 zones, and Group B has 2 clusters and 3 zones.
			// We will realize that Group A is more important than Group B with the help of the following algorithm because
			// Group A has more clusters even if it's zones are less.
			mp.Number |= number << constraint.Weight
		}
		mp.Score = CalculateComprehensiveScore(mp.Nodes)
		paths = append(paths, mp)
	}
	return
}

// mergePaths merges multiple paths into multiDFSPathIndexer.
func mergePaths[T Group](indexer multiDFSPathIndexer[T], paths []Path[T]) multiDFSPathIndexer[T] {
	if len(indexer) == 0 {
		for _, path := range paths {
			mp := &multiDFSPath[T]{
				nodeSet:  sets.Set[string]{},
				BasePath: new(BasePath[T]),
			}
			mp.merge(path)
			indexer[mp.ID] = mp
		}
		return indexer
	}

	newIndexer := multiDFSPathIndexer[T]{}
	for _, mp := range indexer {
		for _, path := range paths {
			mpCopy := mp.clone()
			mpCopy.merge(path)
			newIndexer[mpCopy.ID] = mpCopy
		}
	}
	return newIndexer
}

// selectGroupsByMultiDFS splits multi-dimensional calculations(region, zone and cluster etc)
// into multiple DFS parallel calculations, then merges their results and prioritizes the best path.
func selectGroupsByMultiDFS[T Group](groups []T, ctx *ConstraintContext) []T {
	var pathsWithKind sync.Map
	var counter atomic.Int32

	wg := new(sync.WaitGroup)
	wg.Add(len(ctx.subConstraints))

	for i := 0; i < len(ctx.subConstraints); i++ {
		go func(constraint *Constraint) {
			defer wg.Done()
			paths := findFeasiblePaths(append(make([]T, 0, len(groups)), groups...), ctx.selfConstraint, constraint)
			if len(paths) > 0 {
				pathsWithKind.Store(constraint.Kind, paths)
				counter.Add(1)
			}
		}(&ctx.subConstraints[i])
	}
	wg.Wait()

	// if any one of the calculation results is empty,
	// then the overall result is empty.
	if int(counter.Load()) != len(ctx.subConstraints) {
		return nil
	}

	indexer := multiDFSPathIndexer[T]{}
	pathsWithKind.Range(func(_, value any) bool {
		paths := value.([]Path[T])
		indexer = mergePaths(indexer, paths)
		return true
	})
	paths := indexer.calculatePaths(ctx)

	recommendedPath := prioritizePaths(paths)
	if recommendedPath == nil {
		return nil
	}
	return recommendedPath.GetNodes()
}
