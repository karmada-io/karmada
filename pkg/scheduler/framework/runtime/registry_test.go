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
	"reflect"
	"testing"

	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

func mockPluginFactory() (framework.Plugin, error) {
	return nil, nil
}

func TestRegistry_Filter(t *testing.T) {
	plugins := []string{"bar", "foo", "fuzz"}
	var r = make(Registry)
	for _, name := range plugins {
		_ = r.Register(name, mockPluginFactory)
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

func TestRegistry_Register(t *testing.T) {
	tests := []struct {
		name              string
		initialPlugins    []string
		registeringPlugin string
		wantErr           bool
		expectedPlugins   []string
	}{
		{
			name:              "Plugin registered to an empty Registry",
			initialPlugins:    nil,
			registeringPlugin: "p1",
			wantErr:           false,
			expectedPlugins:   []string{"p1"},
		},
		{
			name:              "Plugin registered to a non empty Registry",
			initialPlugins:    []string{"p1"},
			registeringPlugin: "p2",
			wantErr:           false,
			expectedPlugins:   []string{"p1", "p2"},
		},
		{
			name:              "Duplicate plugin registration",
			initialPlugins:    []string{"p1", "p2"},
			registeringPlugin: "p1",
			wantErr:           true,
			expectedPlugins:   []string{"p1", "p2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r = make(Registry)
			for _, name := range tt.initialPlugins {
				_ = r.Register(name, mockPluginFactory)
			}

			err := r.Register(tt.registeringPlugin, mockPluginFactory)

			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(r.FactoryNames(), tt.expectedPlugins) {
				t.Errorf("Filter() = %v, want %v", r.FactoryNames(), tt.expectedPlugins)
			}
		})
	}
}

func TestRegistry_Unregister(t *testing.T) {
	tests := []struct {
		name            string
		initialPlugins  []string
		removingPlugin  string
		wantErr         bool
		expectedPlugins []string
	}{
		{
			name:            "Remove not exist plugin",
			initialPlugins:  []string{"p1"},
			removingPlugin:  "p2",
			wantErr:         true,
			expectedPlugins: []string{"p1"},
		},
		{
			name:            "Remove exist plugin",
			initialPlugins:  []string{"p1", "p2", "p3"},
			removingPlugin:  "p1",
			wantErr:         false,
			expectedPlugins: []string{"p2", "p3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r = make(Registry)
			for _, name := range tt.initialPlugins {
				_ = r.Register(name, mockPluginFactory)
			}

			err := r.Unregister(tt.removingPlugin)

			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(r.FactoryNames(), tt.expectedPlugins) {
				t.Errorf("FactoryNames() = %v, want %v", r.FactoryNames(), tt.expectedPlugins)
			}
		})
	}
}

func TestRegistry_Merge(t *testing.T) {
	var r1 = make(Registry)
	_ = r1.Register("p1", mockPluginFactory)
	_ = r1.Register("p2", mockPluginFactory)
	_ = r1.Register("p3", mockPluginFactory)

	var r2 = make(Registry)
	_ = r2.Register("p4", mockPluginFactory)
	_ = r2.Register("p5", mockPluginFactory)

	expectedPlugins := []string{
		"p1", "p2", "p3", "p4", "p5",
	}

	err := r1.Merge(r2)
	if err != nil {
		t.Errorf("Merge() returned error: %v", err)
	}

	if !reflect.DeepEqual(r1.FactoryNames(), expectedPlugins) {
		t.Errorf("FactoryNames() = %v, want %v", r1.FactoryNames(), expectedPlugins)
	}
}
