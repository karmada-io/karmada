/*
Copyright 2025 The Karmada Authors.

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

package helper

import (
	"fmt"
	"reflect"
	"testing"
)

func ExampleAllocateWebsterSeats() {
	partiesVotes := map[string]int64{
		"Alpha": 1200,
		"Beta":  900,
		"Gamma": 400,
	}
	totalSeats := int32(4)
	result := AllocateWebsterSeats(totalSeats, partiesVotes, nil, nil)
	for _, p := range result {
		fmt.Printf("%s: %d seats\n", p.Name, p.Seats)
	}
	// Output:
	// Alpha: 2 seats
	// Beta: 1 seats
	// Gamma: 1 seats
}

func TestAllocateWebsterSeats(t *testing.T) {
	// This is used for test the classic example from https://en.wikipedia.org/wiki/Sainte-Lagu%C3%AB_method,
	// where 230,000 voters allocate 8 seats among 4 Parties.
	// The expected seat distribution after each round matches the step-by-step allocation shown in the Wikipedia.
	partiesVotes := map[string]int64{
		"PartyA": 100000,
		"PartyB": 80000,
		"PartyC": 30000,
		"PartyD": 20000,
	}

	tests := []struct {
		name               string
		newSeats           int32
		partyVotes         map[string]int64
		initialAssignments map[string]int32
		tieBreaker         func(a, b Party) bool
		expected           []Party
	}{
		{
			name:               "classic example, round 1",
			newSeats:           1,
			partyVotes:         partiesVotes,
			initialAssignments: nil,
			tieBreaker:         nil,
			expected: []Party{
				{Name: "PartyA", Votes: 100000, Seats: 1},
				{Name: "PartyB", Votes: 80000, Seats: 0},
				{Name: "PartyC", Votes: 30000, Seats: 0},
				{Name: "PartyD", Votes: 20000, Seats: 0},
			},
		},
		{
			name:               "classic example, round 2",
			newSeats:           2,
			partyVotes:         partiesVotes,
			initialAssignments: nil,
			tieBreaker:         nil,
			expected: []Party{
				{Name: "PartyA", Votes: 100000, Seats: 1},
				{Name: "PartyB", Votes: 80000, Seats: 1},
				{Name: "PartyC", Votes: 30000, Seats: 0},
				{Name: "PartyD", Votes: 20000, Seats: 0},
			},
		},
		{
			name:               "classic example, round 3",
			newSeats:           3,
			partyVotes:         partiesVotes,
			initialAssignments: nil,
			tieBreaker:         nil,
			expected: []Party{
				{Name: "PartyA", Votes: 100000, Seats: 2},
				{Name: "PartyB", Votes: 80000, Seats: 1},
				{Name: "PartyC", Votes: 30000, Seats: 0},
				{Name: "PartyD", Votes: 20000, Seats: 0},
			},
		},
		{
			name:               "classic example, round 4",
			newSeats:           4,
			partyVotes:         partiesVotes,
			initialAssignments: nil,
			tieBreaker:         nil,
			expected: []Party{
				{Name: "PartyA", Votes: 100000, Seats: 2},
				{Name: "PartyB", Votes: 80000, Seats: 1},
				{Name: "PartyC", Votes: 30000, Seats: 1},
				{Name: "PartyD", Votes: 20000, Seats: 0},
			},
		},
		{
			name:               "classic example, round 5",
			newSeats:           5,
			partyVotes:         partiesVotes,
			initialAssignments: nil,
			tieBreaker:         nil,
			expected: []Party{
				{Name: "PartyA", Votes: 100000, Seats: 2},
				{Name: "PartyB", Votes: 80000, Seats: 2},
				{Name: "PartyC", Votes: 30000, Seats: 1},
				{Name: "PartyD", Votes: 20000, Seats: 0},
			},
		},
		{
			name:               "classic example, round 6",
			newSeats:           6,
			partyVotes:         partiesVotes,
			initialAssignments: nil,
			tieBreaker:         nil,
			expected: []Party{
				{Name: "PartyA", Votes: 100000, Seats: 2},
				{Name: "PartyB", Votes: 80000, Seats: 2},
				{Name: "PartyC", Votes: 30000, Seats: 1},
				{Name: "PartyD", Votes: 20000, Seats: 1},
			},
		},
		{
			name:               "classic example, round 7",
			newSeats:           7,
			partyVotes:         partiesVotes,
			initialAssignments: nil,
			tieBreaker:         nil,
			expected: []Party{
				{Name: "PartyA", Votes: 100000, Seats: 3},
				{Name: "PartyB", Votes: 80000, Seats: 2},
				{Name: "PartyC", Votes: 30000, Seats: 1},
				{Name: "PartyD", Votes: 20000, Seats: 1},
			},
		},
		{
			name:               "classic example, round 8",
			newSeats:           8,
			partyVotes:         partiesVotes,
			initialAssignments: nil,
			tieBreaker:         nil,
			expected: []Party{
				{Name: "PartyA", Votes: 100000, Seats: 3},
				{Name: "PartyB", Votes: 80000, Seats: 3},
				{Name: "PartyC", Votes: 30000, Seats: 1},
				{Name: "PartyD", Votes: 20000, Seats: 1},
			},
		},
		{
			name:               "empty party votes, expect nil result",
			newSeats:           1,
			partyVotes:         map[string]int64{},
			initialAssignments: nil,
			tieBreaker:         nil,
			expected:           nil,
		},
		{
			name:               "party votes not empty, newSeats is 0, expect all seats 0",
			newSeats:           0,
			partyVotes:         map[string]int64{"PartyA": 100, "PartyB": 200},
			initialAssignments: nil,
			tieBreaker:         nil,
			expected: []Party{
				{Name: "PartyA", Votes: 100, Seats: 0},
				{Name: "PartyB", Votes: 200, Seats: 0},
			},
		},
		{
			name:               "both party votes and newSeats are 0, expect nil result",
			newSeats:           0,
			partyVotes:         map[string]int64{},
			initialAssignments: nil,
			tieBreaker:         nil,
			expected:           nil,
		},
		{
			name:     "tie-breaker is nil, expect break tie by seats",
			newSeats: 1,
			partyVotes: map[string]int64{
				"PartyA": 3,
				"PartyB": 1,
			},
			initialAssignments: map[string]int32{
				"PartyA": 1,
				"PartyB": 0,
			},
			tieBreaker: nil,
			expected: []Party{
				{Name: "PartyA", Votes: 3, Seats: 1},
				{Name: "PartyB", Votes: 1, Seats: 1},
			},
		},
		{
			name:     "tie-breaker is nil, expect break tie by name",
			newSeats: 1,
			partyVotes: map[string]int64{
				"PartyA": 1,
				"PartyB": 1,
			},
			initialAssignments: nil,
			tieBreaker:         nil,
			expected: []Party{
				{Name: "PartyA", Votes: 1, Seats: 1},
				{Name: "PartyB", Votes: 1, Seats: 0},
			},
		},
		{
			name:     "custom tie-breaker returns true, expect custom tie-breaker controls seat",
			newSeats: 1,
			partyVotes: map[string]int64{
				"PartyA": 1,
				"PartyB": 1,
			},
			initialAssignments: nil,
			tieBreaker: func(a, b Party) bool {
				return a.Name < b.Name
			},
			expected: []Party{
				{Name: "PartyA", Votes: 1, Seats: 1},
				{Name: "PartyB", Votes: 1, Seats: 0},
			},
		},
		{
			name:     "custom tie-breaker returns false, expect custom tie-breaker controls seat",
			newSeats: 1,
			partyVotes: map[string]int64{
				"PartyA": 1,
				"PartyB": 1,
			},
			initialAssignments: nil,
			tieBreaker: func(a, b Party) bool {
				return a.Name > b.Name
			},
			expected: []Party{
				{Name: "PartyA", Votes: 1, Seats: 0},
				{Name: "PartyB", Votes: 1, Seats: 1},
			},
		},
		{
			name:     "non-initial allocation, new party joins, only new party gets new seats",
			newSeats: 2,
			partyVotes: map[string]int64{
				"NewParty1": 1,
				"NewParty2": 1,
			},
			initialAssignments: map[string]int32{
				"OldParty": 0,
			},
			tieBreaker: nil,
			expected: []Party{
				{Name: "NewParty1", Votes: 1, Seats: 1},
				{Name: "NewParty2", Votes: 1, Seats: 1},
				{Name: "OldParty", Votes: 0, Seats: 0},
			},
		},
		{
			name:     "non-initial allocation, both new and old parties compete for new seats",
			newSeats: 2,
			partyVotes: map[string]int64{
				"OldParty": 1,
				"NewParty": 1,
			},
			initialAssignments: map[string]int32{
				"OldParty": 1,
			},
			tieBreaker: nil,
			expected: []Party{
				{Name: "NewParty", Votes: 1, Seats: 2},
				{Name: "OldParty", Votes: 1, Seats: 1},
			},
		},
		{
			name:     "non-initial allocation, initialAssignments party not in partyVotes, expect no new seats for that party",
			newSeats: 2,
			partyVotes: map[string]int64{
				"PartyA": 100,
			},
			initialAssignments: map[string]int32{
				"PartyB": 3,
			},
			tieBreaker: nil,
			expected: []Party{
				{Name: "PartyA", Votes: 100, Seats: 2},
				{Name: "PartyB", Votes: 0, Seats: 3},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := AllocateWebsterSeats(test.newSeats, test.partyVotes, test.initialAssignments, test.tieBreaker)
			if !reflect.DeepEqual(got, test.expected) {
				t.Errorf("expected %v, got %v", test.expected, got)
			}
		})
	}
}
