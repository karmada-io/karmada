package runtime

import (
	"reflect"
	"testing"

	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

func TestRegistry_Filter(t *testing.T) {
	plugins := []string{"bar", "foo", "fuzz"}
	var r = make(Registry)
	for _, name := range plugins {
		_ = r.Register(name, func() (framework.Plugin, error) {
			return nil, nil
		})
	}

	tests := []struct {
		name            string
		curPlugins      []string
		r               Registry
		expectedPlugins []string
	}{
		{
			name:            "enable foo",
			curPlugins:      []string{"foo"}, // --plugins=foo
			r:               r,
			expectedPlugins: []string{"foo"},
		},
		{
			name:            "enable all",
			curPlugins:      []string{"*"}, // --plugins=*
			r:               r,
			expectedPlugins: []string{"bar", "foo", "fuzz"},
		},
		{
			name:            "disable foo",
			curPlugins:      []string{"*", "-foo"}, // --plugins=*,-foo
			r:               r,
			expectedPlugins: []string{"bar", "fuzz"},
		},
		{
			name:            "disable foo",
			curPlugins:      []string{"-foo", "*"}, // --plugins=-foo,*
			r:               r,
			expectedPlugins: []string{"bar", "fuzz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Filter(tt.curPlugins); !reflect.DeepEqual(got.FactoryNames(), tt.expectedPlugins) {
				t.Errorf("Filter() = %v, want %v", got.FactoryNames(), tt.expectedPlugins)
			}
		})
	}
}
