package util

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

// GetDifferenceSet will get difference set from includeItems and excludeItems.
func GetDifferenceSet(includeItems, excludeItems []string) []string {
	if includeItems == nil {
		includeItems = []string{}
	}
	if excludeItems == nil {
		excludeItems = []string{}
	}
	includeSet := sets.NewString()
	excludeSet := sets.NewString()
	for _, targetItem := range excludeItems {
		excludeSet.Insert(targetItem)
	}
	for _, targetItem := range includeItems {
		includeSet.Insert(targetItem)
	}

	matchItems := includeSet.Difference(excludeSet)

	return matchItems.List()
}

// GetUniqueElements will delete duplicate element in list.
func GetUniqueElements(list []string) []string {
	if list == nil {
		return []string{}
	}
	result := sets.String{}
	for _, item := range list {
		result.Insert(item)
	}
	return result.List()
}
