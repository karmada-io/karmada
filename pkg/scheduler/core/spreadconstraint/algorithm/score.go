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

import "math"

// Scorer defines a general specification for getting scores.
type Scorer interface {
	GetScore() int64
}

// CalculateComprehensiveScore calculates the comprehensive score from multiple scores.
// We can use e.g. average, median calculation etc.
func CalculateComprehensiveScore[S Scorer](scorers []S) int64 {
	// TODO(whitewindmills): Optimize the scoring algorithm, just simply implement the function now.
	switch len(scorers) {
	case 0:
		return 0
	case 1:
		return scorers[0].GetScore()
	default:
		var sum float64
		for _, scorer := range scorers {
			sum += float64(scorer.GetScore())
		}
		return int64(math.Ceil(sum / float64(len(scorers))))
	}
}
