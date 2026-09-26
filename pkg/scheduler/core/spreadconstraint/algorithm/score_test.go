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
	"testing"
)

type fakeScorer int64

func (f fakeScorer) GetScore() int64 {
	return int64(f)
}

func TestCalculateComprehensiveScore(t *testing.T) {
	cases := []struct {
		name          string
		scorers       []fakeScorer
		expectedScore int64
	}{
		{
			name:          "no scorer",
			scorers:       []fakeScorer{},
			expectedScore: 0,
		},
		{
			name:          "one scorer",
			scorers:       []fakeScorer{1},
			expectedScore: 1,
		},
		{
			name:          "multiple scorers",
			scorers:       []fakeScorer{1, 5, 3},
			expectedScore: 3,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			score := CalculateComprehensiveScore(tt.scorers)
			if score != tt.expectedScore {
				t.Errorf("expected score: %v, but got %v", tt.expectedScore, score)
			}
		})
	}
}
