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
	"reflect"
	"testing"
)

type group struct {
	kind   string
	weight int8
}

var (
	cluster  = group{kind: "cluster", weight: 41}
	zone     = group{kind: "zone", weight: 20}
	region   = group{kind: "region"}
	provider = group{kind: "provider"}
)

type fakeProvider struct {
	regions  int64
	zones    int64
	clusters int64
	score    int64
	name     string
}

func (c *fakeProvider) GetName() string {
	return c.name
}

func (c *fakeProvider) GetScore() int64 {
	return c.score
}

func (c *fakeProvider) GetNumber(kind string) int64 {
	switch kind {
	case region.kind:
		return c.regions
	case zone.kind:
		return c.zones
	case cluster.kind:
		return c.clusters
	default:
		return 1
	}
}

func TestSelectGroups(t *testing.T) {
	cases := []struct {
		name           string
		groups         []*fakeProvider
		ctx            *ConstraintContext
		expectedGroups []string
	}{
		{
			name:   "empty providers",
			groups: []*fakeProvider{},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      cluster.kind,
						MinGroups: 3,
						MaxGroups: 5,
					},
				},
			},
			expectedGroups: nil,
		},
		{
			name: "empty constraints",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 10,
					score:    30,
				},
			},
			ctx:            &ConstraintContext{},
			expectedGroups: []string{"P1"},
		},
		{
			name: "select providers by score:the number of providers less than max groups",
			groups: []*fakeProvider{
				{
					name:  "P1",
					score: 30,
				},
				{
					name:  "P2",
					score: 70,
				},
				{
					name:  "P3",
					score: 90,
				},
				{
					name:  "P4",
					score: 10,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 2,
						MaxGroups: 5,
					},
				},
			},
			expectedGroups: []string{"P1", "P2", "P3", "P4"},
		},
		{
			name: "select providers by score:the number of providers more than max groups",
			groups: []*fakeProvider{
				{
					name:  "P1",
					score: 30,
				},
				{
					name:  "P2",
					score: 70,
				},
				{
					name:  "P3",
					score: 90,
				},
				{
					name:  "P4",
					score: 10,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 2,
						MaxGroups: 2,
					},
				},
			},
			expectedGroups: []string{"P3", "P2"},
		},
		{
			name: "select providers by score:no enough number",
			groups: []*fakeProvider{
				{
					name:  "P1",
					score: 30,
				},
				{
					name:  "P2",
					score: 70,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 5,
						MaxGroups: 5,
					},
				},
			},
			expectedGroups: nil,
		},
		{
			name: "select providers by dfs:select the highest score",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 10,
					score:    30,
				},
				{
					name:     "P2",
					clusters: 20,
					score:    70,
				},
				{
					name:     "P3",
					clusters: 20,
					score:    90,
				},
				{
					name:     "P4",
					clusters: 30,
					score:    10,
				},
				{
					name:     "P5",
					clusters: 50,
					score:    10,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 2,
						MaxGroups: 6,
					},
					{
						Kind:      cluster.kind,
						MinGroups: 50,
						MaxGroups: 100,
					},
				},
			},
			expectedGroups: []string{"P3", "P2", "P1"},
		},
		{
			name: "select providers by dfs:no enough number for itself",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 10,
					score:    30,
				},
				{
					name:     "P2",
					clusters: 20,
					score:    70,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 3,
						MaxGroups: 6,
					},
					{
						Kind:      cluster.kind,
						MinGroups: 50,
						MaxGroups: 60,
					},
				},
			},
			expectedGroups: nil,
		},
		{
			name: "select providers by dfs:no enough number for cluster",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 10,
					score:    30,
				},
				{
					name:     "P2",
					clusters: 20,
					score:    70,
				},
				{
					name:     "P3",
					clusters: 20,
					score:    90,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 1,
						MaxGroups: 3,
					},
					{
						Kind:      cluster.kind,
						MinGroups: 60,
						MaxGroups: 60,
					},
				},
			},
			expectedGroups: nil,
		},
		{
			name: "select providers by dfs:select one provider",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 20,
					score:    90,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 1,
						MaxGroups: 1,
					},
					{
						Kind:      cluster.kind,
						MinGroups: 20,
						MaxGroups: 30,
					},
				},
			},
			expectedGroups: []string{"P1"},
		},
		{
			name: "select providers by dfs:prioritize one path",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 1,
					score:    80,
				},
				{
					name:     "P3",
					clusters: 1,
					score:    80,
				},
				{
					name:     "P2",
					clusters: 1,
					score:    60,
				},
				{
					name:     "P5",
					clusters: 2,
					score:    60,
				},
				{
					name:     "P4",
					clusters: 5,
					score:    50,
				},
				{
					name:     "P6",
					clusters: 3,
					score:    50,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 1,
						MaxGroups: 3,
					},
					{
						Kind:      cluster.kind,
						MinGroups: 10,
						MaxGroups: 30,
					},
				},
			},
			expectedGroups: []string{"P5", "P4", "P6"},
		},
		{
			name: "select providers by dfs:select sub-path",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 1,
					score:    60,
				},
				{
					name:     "P2",
					clusters: 4,
					score:    60,
				},
				{
					name:     "P3",
					clusters: 1,
					score:    30,
				},
				{
					name:     "P4",
					clusters: 3,
					score:    60,
				},
				{
					name:     "P5",
					clusters: 3,
					score:    20,
				},
				{
					name:     "P6",
					clusters: 5,
					score:    10,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 2,
						MaxGroups: 6,
					},
					{
						Kind:      cluster.kind,
						MinGroups: 5,
						MaxGroups: 10,
					},
				},
			},
			expectedGroups: []string{"P2", "P4"},
		},
		{
			name: "select providers by dfs:select the min id",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 1,
					score:    60,
				},
				{
					name:     "P2",
					clusters: 2,
					score:    50,
				},
				{
					name:     "P3",
					clusters: 3,
					score:    40,
				},
				{
					name:     "P4",
					clusters: 4,
					score:    30,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 1,
						MaxGroups: 2,
					},
					{
						Kind:      cluster.kind,
						MinGroups: 5,
						MaxGroups: 10,
					},
				},
			},
			expectedGroups: []string{"P1", "P4"},
		},
		{
			name: "select providers by dfs:select the max number",
			groups: []*fakeProvider{
				{
					name:     "P4",
					clusters: 1,
					score:    60,
				},
				{
					name:     "P3",
					clusters: 3,
					score:    50,
				},
				{
					name:     "P1",
					clusters: 3,
					score:    40,
				},
				{
					name:     "P2",
					clusters: 4,
					score:    30,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 1,
						MaxGroups: 2,
					},
					{
						Kind:      cluster.kind,
						MinGroups: 5,
						MaxGroups: 10,
					},
				},
			},
			expectedGroups: []string{"P3", "P1"},
		},
		{
			name: "select providers by multi-dfs:no enough number for itself",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 10,
					zones:    2,
					regions:  1,
					score:    30,
				},
				{
					name:     "P2",
					clusters: 20,
					zones:    2,
					regions:  1,
					score:    70,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 3,
						MaxGroups: 4,
					},
					{
						Kind:      region.kind,
						MinGroups: 1,
						MaxGroups: 3,
					},
					{
						Kind:      zone.kind,
						Weight:    zone.weight,
						MinGroups: 2,
						MaxGroups: 2,
					},
					{
						Kind:      cluster.kind,
						MinGroups: 50,
						MaxGroups: 60,
					},
				},
			},
			expectedGroups: nil,
		},
		{
			name: "select providers by multi-dfs:no enough number for any sub-constraint",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 10,
					zones:    1,
					regions:  1,
					score:    30,
				},
				{
					name:     "P2",
					clusters: 20,
					zones:    1,
					regions:  1,
					score:    70,
				},
				{
					name:     "P3",
					clusters: 20,
					zones:    2,
					regions:  1,
					score:    80,
				},
				{
					name:     "P4",
					clusters: 20,
					zones:    1,
					regions:  1,
					score:    50,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 1,
						MaxGroups: 3,
					},
					{
						Kind:      region.kind,
						MinGroups: 4,
						MaxGroups: 6,
					},
					{
						Kind:      zone.kind,
						Weight:    zone.weight,
						MinGroups: 2,
						MaxGroups: 5,
					},
					{
						Kind:      cluster.kind,
						Weight:    cluster.weight,
						MinGroups: 20,
						MaxGroups: 60,
					},
				},
			},
			expectedGroups: nil,
		},
		{
			name: "select providers by multi-dfs:select the max number",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 30,
					zones:    2,
					regions:  2,
					score:    80,
				},
				{
					name:     "P2",
					clusters: 15,
					zones:    4,
					regions:  3,
					score:    80,
				},
				{
					name:     "P3",
					clusters: 20,
					zones:    4,
					regions:  1,
					score:    80,
				},
				{
					name:     "P4",
					clusters: 5,
					zones:    10,
					regions:  4,
					score:    80,
				},
				{
					name:     "P5",
					clusters: 5,
					zones:    6,
					regions:  5,
					score:    80,
				},
				{
					name:     "P6",
					clusters: 30,
					zones:    7,
					regions:  6,
					score:    80,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 5,
						MaxGroups: 5,
					},
					{
						Kind:      region.kind,
						MinGroups: 6,
						MaxGroups: 10,
					},
					{
						Kind:      zone.kind,
						Weight:    zone.weight,
						MinGroups: 10,
						MaxGroups: 15,
					},
					{
						Kind:      cluster.kind,
						Weight:    cluster.weight,
						MinGroups: 50,
						MaxGroups: 90,
					},
				},
			},
			expectedGroups: []string{"P1", "P2", "P3", "P4", "P6"},
		},
		{
			name: "select providers by multi-dfs:exceed the max provider groups",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 5,
					zones:    2,
					regions:  1,
					score:    30,
				},
				{
					name:     "P2",
					clusters: 5,
					zones:    3,
					regions:  2,
					score:    70,
				},
				{
					name:     "P3",
					clusters: 20,
					zones:    2,
					regions:  1,
					score:    80,
				},
				{
					name:     "P4",
					clusters: 10,
					zones:    1,
					regions:  1,
					score:    50,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 1,
						MaxGroups: 3,
					},
					{
						Kind:      region.kind,
						MinGroups: 4,
						MaxGroups: 6,
					},
					{
						Kind:      zone.kind,
						Weight:    zone.weight,
						MinGroups: 7,
						MaxGroups: 9,
					},
					{
						Kind:      cluster.kind,
						Weight:    cluster.weight,
						MinGroups: 35,
						MaxGroups: 60,
					},
				},
			},
			expectedGroups: nil,
		},
		{
			name: "select providers by multi-dfs:prioritize one path",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 5,
					zones:    2,
					regions:  1,
					score:    30,
				},
				{
					name:     "P2",
					clusters: 5,
					zones:    3,
					regions:  2,
					score:    70,
				},
				{
					name:     "P3",
					clusters: 20,
					zones:    2,
					regions:  1,
					score:    80,
				},
				{
					name:     "P4",
					clusters: 10,
					zones:    1,
					regions:  1,
					score:    50,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 1,
						MaxGroups: 4,
					},
					{
						Kind:      region.kind,
						MinGroups: 4,
						MaxGroups: 6,
					},
					{
						Kind:      zone.kind,
						Weight:    zone.weight,
						MinGroups: 7,
						MaxGroups: 9,
					},
					{
						Kind:      cluster.kind,
						Weight:    cluster.weight,
						MinGroups: 35,
						MaxGroups: 60,
					},
				},
			},
			expectedGroups: []string{"P3", "P2", "P4", "P1"},
		},
		{
			name: "select providers by multi-dfs:select the highest score",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 5,
					zones:    1,
					regions:  1,
					score:    30,
				},
				{
					name:     "P2",
					clusters: 15,
					zones:    3,
					regions:  2,
					score:    70,
				},
				{
					name:     "P3",
					clusters: 15,
					zones:    3,
					regions:  2,
					score:    80,
				},
				{
					name:     "P4",
					clusters: 5,
					zones:    1,
					regions:  1,
					score:    50,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 2,
						MaxGroups: 3,
					},
					{
						Kind:      region.kind,
						MinGroups: 5,
						MaxGroups: 6,
					},
					{
						Kind:      zone.kind,
						Weight:    zone.weight,
						MinGroups: 7,
						MaxGroups: 9,
					},
					{
						Kind:      cluster.kind,
						Weight:    cluster.weight,
						MinGroups: 35,
						MaxGroups: 60,
					},
				},
			},
			expectedGroups: []string{"P3", "P2", "P4"},
		},
		{
			name: "select providers by multi-dfs:select one provider",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 20,
					zones:    4,
					score:    90,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 1,
						MaxGroups: 1,
					},
					{
						Kind:      zone.kind,
						Weight:    zone.weight,
						MinGroups: 3,
						MaxGroups: 9,
					},
					{
						Kind:      cluster.kind,
						Weight:    cluster.weight,
						MinGroups: 20,
						MaxGroups: 60,
					},
				},
			},
			expectedGroups: []string{"P1"},
		},
		{
			name: "select providers by multi-dfs:select sub-path",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 10,
					zones:    3,
					regions:  3,
					score:    30,
				},
				{
					name:     "P2",
					clusters: 5,
					zones:    4,
					regions:  3,
					score:    30,
				},
				{
					name:     "P3",
					clusters: 15,
					zones:    3,
					regions:  2,
					score:    30,
				},
				{
					name:     "P4",
					clusters: 5,
					zones:    3,
					regions:  3,
					score:    30,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 2,
						MaxGroups: 4,
					},
					{
						Kind:      region.kind,
						MinGroups: 5,
						MaxGroups: 6,
					},
					{
						Kind:      zone.kind,
						Weight:    zone.weight,
						MinGroups: 6,
						MaxGroups: 9,
					},
					{
						Kind:      cluster.kind,
						Weight:    cluster.weight,
						MinGroups: 20,
						MaxGroups: 60,
					},
				},
			},
			expectedGroups: []string{"P1", "P3"},
		},
		{
			name: "select providers by multi-dfs:select the min id",
			groups: []*fakeProvider{
				{
					name:     "P1",
					clusters: 5,
					zones:    1,
					regions:  1,
					score:    70,
				},
				{
					name:     "P2",
					clusters: 15,
					zones:    3,
					regions:  2,
					score:    70,
				},
				{
					name:     "P3",
					clusters: 15,
					zones:    3,
					regions:  2,
					score:    70,
				},
				{
					name:     "P4",
					clusters: 5,
					zones:    1,
					regions:  1,
					score:    70,
				},
			},
			ctx: &ConstraintContext{
				Constraints: []Constraint{
					{
						Kind:      provider.kind,
						MinGroups: 2,
						MaxGroups: 3,
					},
					{
						Kind:      region.kind,
						MinGroups: 5,
						MaxGroups: 6,
					},
					{
						Kind:      zone.kind,
						Weight:    zone.weight,
						MinGroups: 7,
						MaxGroups: 9,
					},
					{
						Kind:      cluster.kind,
						Weight:    cluster.weight,
						MinGroups: 35,
						MaxGroups: 60,
					},
				},
			},
			expectedGroups: []string{"P1", "P2", "P3"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result := SelectGroups(provider.kind, tt.groups, tt.ctx)
			var groupNames []string
			for _, group := range result {
				groupNames = append(groupNames, group.GetName())
			}
			if len(groupNames) != len(tt.expectedGroups) || (len(groupNames) > 0 && !reflect.DeepEqual(tt.expectedGroups, groupNames)) {
				t.Errorf("expected: %v, but got %v", tt.expectedGroups, groupNames)
			}
		})
	}
}
