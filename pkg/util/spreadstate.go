package util

import (
	"sync"

	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// SpreadGroup stores the cluster group info for given spread constraints
type SpreadGroup struct {
	// The outer map's keys are SpreadConstraint. The values (inner map) of the outer map are maps with string
	// keys and []string values. The inner map's key should specify the cluster group name.
	GroupRecord map[v1alpha1.SpreadConstraint]map[string][]string
	sync.RWMutex
}

// NewSpreadGroup initializes a SpreadGroup
func NewSpreadGroup() *SpreadGroup {
	return &SpreadGroup{
		GroupRecord: make(map[v1alpha1.SpreadConstraint]map[string][]string),
	}
}

// InitialGroupRecord initials a spread state record
func (ss *SpreadGroup) InitialGroupRecord(constraint v1alpha1.SpreadConstraint) {
	ss.Lock()
	defer ss.Unlock()
	ss.GroupRecord[constraint] = make(map[string][]string)
}
