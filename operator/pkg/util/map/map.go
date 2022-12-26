package util

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
)

// MergeBoolMaps merges two bool maps.
func MergeBoolMaps(maps ...map[string]bool) map[string]bool {
	result := map[string]bool{}
	for _, m := range maps {
		for key, value := range m {
			result[key] = value
		}
	}
	return result
}

// MergeStringMaps merges two string maps.
func MergeStringMaps(maps ...map[string]string) map[string]string {
	result := map[string]string{}
	for _, m := range maps {
		for key, value := range m {
			result[key] = value
		}
	}
	return result
}

// ConvertToCommandOrArgs converts a string map to a command or args array.
func ConvertToCommandOrArgs(m map[string]string) []string {
	keys := sets.NewString()
	for k := range m {
		keys.Insert(k)
	}
	array := make([]string, 0, len(keys))
	for _, v := range keys.List() {
		array = append(array, fmt.Sprintf("--%s=%s", v, m[v]))
	}
	return array
}
