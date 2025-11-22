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
	"container/heap"
	"sort"
)

// Party represents a party in the Webster apportionment method.
type Party struct {
	// Name is the name of the party.
	Name string
	// Votes is the number of votes the party received.
	Votes int64
	// Seats is the number of seats assigned to the party.
	Seats int32
}

// WebsterPriorityQueue implements heap.Interface using the Webster (Sainte-Laguë) method.
// The party with the highest Webster priority (Votes/(2*Seats+1)) is at the top.
type WebsterPriorityQueue struct {
	// Parties contains all the parties participating in the Webster apportionment method.
	// Each party has a name, vote count, and current seat allocation.
	Parties []Party

	// TieBreaker is an optional function used to break ties when two parties have
	// the same Webster priority. If nil, ties are broken by vote count, then seat count,
	// then party name to ensure a deterministic result.
	TieBreaker func(a Party, b Party) bool
}

// Check if our WebsterPriorityQueue implements necessary interfaces
var _ heap.Interface = &WebsterPriorityQueue{}

// Len returns the number of Parties in the queue.
func (pq *WebsterPriorityQueue) Len() int {
	return len(pq.Parties)
}

// Less compares two Parties by their Webster priority.
// The party with the higher priority comes first.
func (pq *WebsterPriorityQueue) Less(i, j int) bool {
	// In the Webster method, compare the priority of two Parties:
	// the one with the higher value of Votes/(2*Seats+1) gets the next seat.
	iPriority := float64(pq.Parties[i].Votes) / (float64(2*pq.Parties[i].Seats + 1))
	jPriority := float64(pq.Parties[j].Votes) / (float64(2*pq.Parties[j].Seats + 1))
	if iPriority == jPriority {
		if pq.TieBreaker != nil {
			return pq.TieBreaker(pq.Parties[i], pq.Parties[j])
		}

		// The party with fewer seats wins the tie.
		// This is because the Webster method inherently favors smaller parties (or, in Karmada's context, smaller clusters).
		// In Karmada, this method is often used to assign replicas based on static or dynamic weights.
		// The rationale is that assigning to smaller clusters earlier helps establish High Availability (HA) and enables
		// Horizontal Pod Autoscaling (HPA) more effectively. By preferring parties (clusters) with fewer seats (replicas),
		// we promote a more balanced and resilient distribution, which is desirable for HA scenarios.
		if pq.Parties[i].Seats != pq.Parties[j].Seats {
			return pq.Parties[i].Seats < pq.Parties[j].Seats
		}

		// If both parties have the same priority and the same number of seats,
		// that means they also have the same number of votes. So we don't have to compare the votes now.
		// The party with the lexicographically smaller name wins the tie.
		// For example, party 'a' wins over party 'b' because 'a' < 'b' lexicographically.
		return pq.Parties[i].Name < pq.Parties[j].Name
	}

	return iPriority > jPriority
}

// Swap swaps two Parties in the queue.
func (pq *WebsterPriorityQueue) Swap(i, j int) {
	pq.Parties[i], pq.Parties[j] = pq.Parties[j], pq.Parties[i]
}

// Push adds a new party to the queue.
func (pq *WebsterPriorityQueue) Push(x interface{}) {
	pq.Parties = append(pq.Parties, x.(Party))
}

// Pop removes and returns the party with the highest priority.
func (pq *WebsterPriorityQueue) Pop() interface{} {
	old := pq.Parties
	n := len(old)
	item := old[n-1]
	pq.Parties = old[0 : n-1]
	return item
}

// AllocateWebsterSeats allocates new seats using the Webster method.
// newSeats: number of new seats to allocate across all Parties.
// partyVotes: map of party name to number of votes.
// initialAssignments: map of party name to initial seat assignments.
// tieBreaker: function to break ties between two Parties.
// Returns: slice of Party with updated Seats, sorted by Name in ascending order.
func AllocateWebsterSeats(newSeats int32, partyVotes map[string]int64, initialAssignments map[string]int32, tieBreaker func(a, b Party) bool) []Party {
	partiesMap := make(map[string]Party, len(partyVotes)+len(initialAssignments))

	// set initial party assignments, the votes default to 0
	// If a party is present in initialAssignments but not in partyVotes,
	// it will be initialized with 0 votes and its initial seat count.
	// Such a party will still be included in the allocation process, but
	// the priority of such a party is 0, which is the lowest priority.
	for n, s := range initialAssignments {
		partiesMap[n] = Party{Name: n, Votes: 0, Seats: s}
	}

	for n, v := range partyVotes {
		if p, ok := partiesMap[n]; ok {
			p.Votes = v
			partiesMap[n] = p
		} else {
			partiesMap[n] = Party{Name: n, Votes: v, Seats: 0}
		}
	}

	pq := WebsterPriorityQueue{
		Parties:    make([]Party, 0, len(partiesMap)),
		TieBreaker: tieBreaker,
	}

	for _, party := range partiesMap {
		pq.Parties = append(pq.Parties, party)
	}

	if len(pq.Parties) == 0 {
		return nil
	}

	heap.Init(&pq)

	for remaining := newSeats; remaining > 0; remaining-- {
		nextParty := heap.Pop(&pq).(Party)
		nextParty.Seats++
		heap.Push(&pq, nextParty)
	}

	// sort the parties by name in ascending order, to ensure the result is deterministic.
	// This will break the heap structure, but it is ok because we will not use the heap structure again.
	sort.Slice(pq.Parties, func(i, j int) bool {
		return pq.Parties[i].Name < pq.Parties[j].Name
	})

	return pq.Parties
}
