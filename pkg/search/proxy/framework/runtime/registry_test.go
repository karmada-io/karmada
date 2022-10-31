package runtime

import (
	"testing"

	"github.com/karmada-io/karmada/pkg/search/proxy/framework"
)

func emptyPluginFactory(dep PluginDependency) (framework.Plugin, error) {
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
