/*
Copyright The Karmada Authors.

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
	"sync"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// SpreadGroup stores the cluster group info for given spread constraints
type SpreadGroup struct {
	// The outer map's keys are SpreadConstraint. The values (inner map) of the outer map are maps with string
	// keys and []string values. The inner map's key should specify the cluster group name.
	GroupRecord map[policyv1alpha1.SpreadConstraint]map[string][]*clusterv1alpha1.Cluster
	sync.RWMutex
}

// NewSpreadGroup initializes a SpreadGroup
func NewSpreadGroup() *SpreadGroup {
	return &SpreadGroup{
		GroupRecord: make(map[policyv1alpha1.SpreadConstraint]map[string][]*clusterv1alpha1.Cluster),
	}
}

// InitialGroupRecord initials a spread state record
func (ss *SpreadGroup) InitialGroupRecord(constraint policyv1alpha1.SpreadConstraint) {
	ss.Lock()
	defer ss.Unlock()
	ss.GroupRecord[constraint] = make(map[string][]*clusterv1alpha1.Cluster)
}
