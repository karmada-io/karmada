/*
Copyright 2022 The Karmada Authors.

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

package runtime

import (
	"testing"

	"github.com/karmada-io/karmada/pkg/search/proxy/framework"
)

func emptyPluginFactory(_ PluginDependency) (framework.Plugin, error) {
	return nil, nil
}

func TestRegistry_Register(t *testing.T) {
	// test nil slice
	var nilSlice Registry

	t.Logf("nilSlice: %v, len: %v, cap: %v\n", nilSlice, len(nilSlice), cap(nilSlice))

	nilSlice.Register(emptyPluginFactory)

	// no panic

	t.Logf("nilSlice: %v, len: %v, cap: %v\n", nilSlice, len(nilSlice), cap(nilSlice))

	if len(nilSlice) != 1 {
		t.Fatalf("slice len = %v, expected = 1", len(nilSlice))
	}
}
