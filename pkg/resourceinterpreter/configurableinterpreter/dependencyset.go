package configurableinterpreter

import (
	"fmt"
	"sort"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

type dependencySet map[configv1alpha1.DependentObjectReference]struct{}

func newDependencySet(items ...configv1alpha1.DependentObjectReference) dependencySet {
	s := make(dependencySet, len(items))
	s.insert(items...)
	return s
}

func (s dependencySet) insert(items ...configv1alpha1.DependentObjectReference) dependencySet {
	for _, item := range items {
		s[item] = struct{}{}
	}
	return s
}

func (s dependencySet) list() []configv1alpha1.DependentObjectReference {
	keys := make([]configv1alpha1.DependentObjectReference, len(s))
	index := 0
	for key := range s {
		keys[index] = key
		index++
	}
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprintf("%s", keys[i]) < fmt.Sprintf("%s", keys[j])
	})
	return keys
}
