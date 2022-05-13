package context

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

type args struct {
	controllerName               string
	disabledByDefaultControllers []string
}

func TestContext_IsControllerEnabled(t *testing.T) {
	tests := []struct {
		name     string
		opts     Options
		args     args
		expected bool
	}{
		{
			name: "on by name",
			args: args{
				controllerName:               "bravo",
				disabledByDefaultControllers: []string{"delta", "echo"},
			},
			opts: Options{
				Controllers: []string{"alpha", "bravo", "-charlie"},
			},
			expected: true,
		},
		{
			name: "off by name",
			args: args{
				controllerName:               "charlie",
				disabledByDefaultControllers: []string{"delta", "echo"},
			},
			opts: Options{
				Controllers: []string{"alpha", "bravo", "-charlie"},
			},
			expected: false,
		},
		{
			name: "on by default",
			args: args{
				controllerName:               "alpha",
				disabledByDefaultControllers: []string{"delta", "echo"},
			},
			opts: Options{
				Controllers: []string{"*"},
			},
			expected: true,
		},
		{
			name: "off by default",
			args: args{
				controllerName:               "delta",
				disabledByDefaultControllers: []string{"delta", "echo"},
			},
			opts: Options{
				Controllers: []string{"*"},
			},
			expected: false,
		},
		{
			name: "on by star, not off by name",
			args: args{
				controllerName:               "alpha",
				disabledByDefaultControllers: []string{"delta", "echo"},
			},
			opts: Options{
				Controllers: []string{"*", "-charlie"},
			},
			expected: true,
		},
		{
			name: "off by name with star",
			args: args{
				controllerName:               "charlie",
				disabledByDefaultControllers: []string{"delta", "echo"},
			},
			opts: Options{
				Controllers: []string{"*", "-charlie"},
			},
			expected: false,
		},
		{
			name: "off by default implicit, no star",
			args: args{
				controllerName:               "foxtrot",
				disabledByDefaultControllers: []string{"delta", "echo"},
			},
			opts: Options{
				Controllers: []string{"alpha", "bravo", "-charlie"},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Context{
				Opts: tt.opts,
			}
			if got := c.IsControllerEnabled(tt.args.controllerName, sets.NewString(tt.args.disabledByDefaultControllers...)); got != tt.expected {
				t.Errorf("IsControllerEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}
