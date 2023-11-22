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

package spreadconstraint

import (
	"reflect"
	"testing"
)

func TestSelectGroups(t *testing.T) {
	tests := []struct {
		name           string
		groups         []*GroupInfo
		minConstraints int
		maxConstraints int
		target         int
		expected       []string
	}{
		{
			name:           "empty groups",
			minConstraints: 2,
			maxConstraints: 3,
			target:         1,
		},
		{
			name: "no enough groups",
			groups: []*GroupInfo{
				{
					name:   "R1",
					value:  1,
					weight: 80,
				},
			},
			minConstraints: 2,
			maxConstraints: 3,
			target:         1,
		},
		{
			name: "no enough value",
			groups: []*GroupInfo{
				{
					name:   "R1",
					value:  1,
					weight: 80,
				},
				{
					name:   "R2",
					value:  2,
					weight: 30,
				},
			},
			minConstraints: 2,
			maxConstraints: 3,
			target:         4,
		},
		{
			name: "select one group",
			groups: []*GroupInfo{
				{
					name:   "R1",
					value:  1,
					weight: 80,
				},
			},
			minConstraints: 1,
			maxConstraints: 3,
			target:         1,
			expected:       []string{"R1"},
		},
		{
			name: "prioritize one path",
			groups: []*GroupInfo{
				{
					name:   "R1",
					value:  1,
					weight: 80,
				},
				{
					name:   "R3",
					value:  1,
					weight: 80,
				},
				{
					name:   "R2",
					value:  1,
					weight: 60,
				},
				{
					name:   "R5",
					value:  2,
					weight: 60,
				},
				{
					name:   "R4",
					value:  5,
					weight: 50,
				},
				{
					name:   "R6",
					value:  3,
					weight: 50,
				},
			},
			minConstraints: 1,
			maxConstraints: 3,
			target:         10,
			expected:       []string{"R5", "R4", "R6"},
		},
		{
			name: "select groups with less total number",
			groups: []*GroupInfo{
				{
					name:   "R1",
					value:  1,
					weight: 80,
				},
				{
					name:   "R2",
					value:  4,
					weight: 40,
				},
				{
					name:   "R3",
					value:  1,
					weight: 30,
				},
				{
					name:   "R4",
					value:  3,
					weight: 30,
				},
				{
					name:   "R5",
					value:  3,
					weight: 20,
				},
				{
					name:   "R6",
					value:  5,
					weight: 10,
				},
			},
			minConstraints: 2,
			maxConstraints: 6,
			target:         5,
			expected:       []string{"R1", "R2"},
		},
		{
			name: "select groups with the highest score",
			groups: []*GroupInfo{
				{
					name:   "R1",
					value:  1,
					weight: 60,
				},
				{
					name:   "R2",
					value:  1,
					weight: 50,
				},
				{
					name:   "R3",
					value:  1,
					weight: 40,
				},
				{
					name:   "R4",
					value:  3,
					weight: 30,
				},
				{
					name:   "R5",
					value:  3,
					weight: 20,
				},
				{
					name:   "R6",
					value:  3,
					weight: 10,
				},
			},
			minConstraints: 1,
			maxConstraints: 3,
			target:         6,
			expected:       []string{"R1", "R4", "R5"},
		},
		{
			name: "select groups that are chosen earlier",
			groups: []*GroupInfo{
				{
					name:   "R1",
					value:  1,
					weight: 60,
				},
				{
					name:   "R2",
					value:  2,
					weight: 50,
				},
				{
					name:   "R3",
					value:  3,
					weight: 40,
				},
				{
					name:   "R4",
					value:  4,
					weight: 30,
				},
			},
			minConstraints: 1,
			maxConstraints: 2,
			target:         5,
			expected:       []string{"R1", "R4"},
		},
		{
			name: "select groups with greater value",
			groups: []*GroupInfo{
				{
					name:   "R4",
					value:  1,
					weight: 60,
				},
				{
					name:   "R3",
					value:  3,
					weight: 50,
				},
				{
					name:   "R1",
					value:  3,
					weight: 40,
				},
				{
					name:   "R2",
					value:  4,
					weight: 30,
				},
			},
			minConstraints: 1,
			maxConstraints: 2,
			target:         5,
			expected:       []string{"R3", "R1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectGroups(tt.groups, tt.minConstraints, tt.maxConstraints, tt.target)
			var groupNames []string
			for _, group := range result {
				groupNames = append(groupNames, group.name)
			}
			if !reflect.DeepEqual(tt.expected, groupNames) {
				t.Errorf("expected: %v, but got %v", tt.expected, result)
			}
		})
	}
}
