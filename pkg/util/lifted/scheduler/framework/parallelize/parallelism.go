/*
Copyright 2020 The Kubernetes Authors.

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

// This code is directly lifted from the Kubernetes codebase.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/scheduler/framework/parallelize/parallelism.go

package parallelize

import (
	"context"
	"math"

	"k8s.io/client-go/util/workqueue"
)

// DefaultParallelism is the default parallelism used in scheduler.
const DefaultParallelism int = 16

// Parallelizer holds the parallelism for scheduler.
type Parallelizer struct {
	parallelism int
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/scheduler/framework/parallelize/parallelism.go#L35-L38
// +lifted:changed

// NewParallelizer returns an object holding the parallelism.
func NewParallelizer(p int) Parallelizer {
	if p <= 0 {
		p = DefaultParallelism
	}
	return Parallelizer{parallelism: p}
}

// chunkSizeFor returns a chunk size for the given number of items to use for
// parallel work. The size aims to produce good CPU utilization.
// returns max(1, min(sqrt(n), n/Parallelism))
func chunkSizeFor(n, parallelism int) int {
	s := int(math.Sqrt(float64(n)))

	if r := n/parallelism + 1; s > r {
		s = r
	} else if s < 1 {
		s = 1
	}
	return s
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/scheduler/framework/parallelize/parallelism.go#L54-L65
// +lifted:changed

// Until is a wrapper around workqueue.ParallelizeUntil to use in scheduling algorithms.
// A given operation will be a label that is recorded in the goroutine metric.
func (p Parallelizer) Until(ctx context.Context, pieces int, doWorkPiece workqueue.DoWorkPieceFunc) {
	workqueue.ParallelizeUntil(ctx, p.parallelism, pieces, doWorkPiece, workqueue.WithChunkSize(chunkSizeFor(pieces, p.parallelism)))
}
