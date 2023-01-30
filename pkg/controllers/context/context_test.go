package context

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

func TestContext_IsControllerEnabled(t *testing.T) {
	tests := []struct {
		name                         string
		controllerName               string
		controllers                  []string
		disabledByDefaultControllers []string
		expected                     bool
	}{
		{
			name:                         "on by name",
			controllerName:               "bravo",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"alpha", "bravo", "-charlie"}, // --controllers=alpha,bravo,-charlie
			expected:                     true,
		},
		{
			name:                         "off by name",
			controllerName:               "charlie",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"alpha", "bravo", "-charlie"}, // --controllers=alpha,bravo,-charlie
			expected:                     false,
		},
		{
			name:                         "on by default",
			controllerName:               "alpha",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"*"}, // --controllers=*
			expected:                     true,
		},
		{
			name:                         "off by default",
			controllerName:               "delta",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"*"}, // --controllers=*
			expected:                     false,
		},
		{
			name:                         "on by star, not off by name",
			controllerName:               "alpha",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"*", "-charlie"}, // --controllers=*,-charlie
			expected:                     true,
		},
		{
			name:                         "off by name with star",
			controllerName:               "charlie",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"*", "-charlie"}, // --controllers=*,-charlie
			expected:                     false,
		},
		{
			name:                         "off by default implicit, no star",
			controllerName:               "foxtrot",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"alpha", "bravo", "-charlie"}, // --controllers=alpha,bravo,-charlie
			expected:                     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Context{
				Opts: Options{
					Controllers: tt.controllers,
				},
			}
			if got := c.IsControllerEnabled(tt.controllerName, sets.New(tt.disabledByDefaultControllers...)); got != tt.expected {
				t.Errorf("expected %v, but got %v", tt.expected, got)
			}
		})
	}
}
