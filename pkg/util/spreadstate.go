package util

import (
	"sync"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// SpreadGroup stores the cluster group info for given spread constraints
type SpreadGroup struct {
	// The outer map's keys are SpreadConstraint. The values (inner map) of the outer map are maps with string
	// keys and []*clusterv1alpha1.Cluster values. The inner map's key should specify the cluster group name.
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
