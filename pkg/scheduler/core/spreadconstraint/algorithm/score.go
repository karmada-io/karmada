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
