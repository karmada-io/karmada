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
	"strings"
)

// DiffKey compares keys of two map with same key type,
// as the name of return values told, it will find out added keys
// and removed keys.
func DiffKey[K comparable, V1, V2 any](previous map[K]V1, current map[K]V2) (added, removed []K) {
	if previous == nil && current == nil {
		return
	}
	if previous == nil {
		for key := range current {
			added = append(added, key)
		}
		return
	}
	if current == nil {
		for key := range previous {
			removed = append(removed, key)
		}
		return
	}
	for key := range previous {
		if _, exist := current[key]; !exist {
			removed = append(removed, key)
		}
	}
	for key := range current {
		if _, exist := previous[key]; !exist {
			added = append(added, key)
		}
	}
	return
}

// StringerJoin acts the same with `strings.Join`, except that
// it consumes a slice of `fmt.Stringer`.
// This mainly used for debug purpose, to log some slice of complex
// object as human-readable string.
func StringerJoin[T fmt.Stringer](st []T, sep string) string {
	ss := make([]string, 0, len(st))
	for _, s := range st {
		ss = append(ss, s.String())
	}
	return strings.Join(ss, sep)
}

// Keys return slice of keys of the given map
func Keys[K comparable, V any](m map[K]V) []K {
	if m == nil {
		return nil
	}
	keys := make([]K, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
