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

package util

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

type sigMultiCluster struct {
	Name          string
	Status        string
	LatestVersion string
	Comment       string
}

func (smc sigMultiCluster) String() string {
	return fmt.Sprintf("%s/%s/%s", smc.Name, smc.Status, smc.LatestVersion)
}

var (
	kubefed = sigMultiCluster{
		Name:          "kubefed",
		Status:        "archived",
		LatestVersion: "v0.9.2",
	}
	karmada = sigMultiCluster{
		Name:          "karmada",
		Status:        "active",
		LatestVersion: "v1.6.0",
	}
	virtualKubelet = sigMultiCluster{
		Name:          "virtual-kubelet",
		Status:        "active",
		LatestVersion: "v1.10.0",
		Comment:       "However, it should be noted that VK is explicitly not intended to be an alternative to Kubernetes federation.",
	}
)

func TestDiffKey(t *testing.T) {
	tests := []struct {
		name            string
		previous        map[string]sigMultiCluster
		current         map[string]string
		expectedAdded   []string
		expectedRemoved []string
	}{
		{
			name: "all old remove, all new added",
			previous: map[string]sigMultiCluster{
				"kubefed": kubefed,
			},
			current: map[string]string{
				"karmada": "v1.6.0",
			},
			expectedAdded:   []string{"karmada"},
			expectedRemoved: []string{"kubefed"},
		},
		{
			name:            "no old, no new",
			previous:        nil,
			current:         nil,
			expectedAdded:   nil,
			expectedRemoved: nil,
		},
		{
			name:     "no old, all new added",
			previous: nil,
			current: map[string]string{
				"karmada": "v1.6.0",
			},
			expectedAdded:   []string{"karmada"},
			expectedRemoved: nil,
		},
		{
			name: "all old removed, no new",
			previous: map[string]sigMultiCluster{
				"kubefed": kubefed,
			},
			current:         nil,
			expectedAdded:   nil,
			expectedRemoved: []string{"kubefed"},
		},
		{
			name: "removed, added and remained",
			previous: map[string]sigMultiCluster{
				"kubefed":         kubefed,
				"virtual-kubelet": virtualKubelet,
			},
			current: map[string]string{
				"karmada":         "v1.6.0",
				"virtual-kubelet": virtualKubelet.Comment,
			},
			expectedAdded:   []string{"karmada"},
			expectedRemoved: []string{"kubefed"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added, removed := DiffKey(tt.previous, tt.current)
			if !reflect.DeepEqual(added, tt.expectedAdded) {
				t.Errorf("added = %v, want %v", added, tt.expectedAdded)
			}
			if !reflect.DeepEqual(removed, tt.expectedRemoved) {
				t.Errorf("removed = %v want %v", removed, tt.expectedRemoved)
			}
		})
	}
}

func TestStringerJoin(t *testing.T) {
	mcs := []sigMultiCluster{kubefed, karmada}
	got := StringerJoin(mcs, "; ")
	expect := "kubefed/archived/v0.9.2; karmada/active/v1.6.0"
	if got != expect {
		t.Errorf("got = %s, want %s", got, expect)
	}
}

func TestKeys(t *testing.T) {
	tests := []struct {
		name   string
		mcs    map[string]sigMultiCluster
		expect []string
	}{
		{
			name: "keys exist",
			mcs: map[string]sigMultiCluster{
				kubefed.Name: kubefed,
				karmada.Name: karmada,
			},
			expect: []string{kubefed.Name, karmada.Name},
		},
		{
			name:   "empty keys",
			mcs:    nil,
			expect: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Keys(tt.mcs)
			sort.Strings(got)
			sort.Strings(tt.expect)
			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("got = %v, want %v", got, tt.expect)
			}
		})
	}
}
