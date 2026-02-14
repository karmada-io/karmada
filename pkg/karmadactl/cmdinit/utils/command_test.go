/*
Copyright 2025 The Karmada Authors.

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

package utils

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

var (
	defaultArgs = []string{
		"test",
		"--v=6",                          // --key=value
		"--runtime-config=",              // --key=
		"--authorization-mode=Node,RBAC", // --key=v1,v2
	}
)

type componentCommandCase struct {
	name      string
	extraArgs []string
	finalArgs []string
	err       error
}

// User normal input
var commonCases = []componentCommandCase{
	// 1. --key=value format (replace existing parameters)
	{
		name:      "replace existing key=value",
		extraArgs: []string{"--v=2"},
		finalArgs: []string{
			"test",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
			"--v=2",
		},
		err: nil,
	},

	// 2. --key= format (replace existing parameters with empty values)
	{
		name:      "replace with empty value",
		extraArgs: []string{"--runtime-config="},
		finalArgs: []string{
			"test",
			"--v=6",
			"--authorization-mode=Node,RBAC",
			"--runtime-config=",
		},
		err: nil,
	},

	// 3. --key format (Boolean flag, new parameter)
	{
		name:      "add boolean flag",
		extraArgs: []string{"--enable-pprof"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
			"--enable-pprof",
		},
		err: nil,
	},

	// 4. --key=v1,v2 format (merging multiple values through preprocessing)
	{
		name:      "merge multiple values",
		extraArgs: []string{"--new-option=value1", "value2", "value3"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
			"--new-option=value1,value2,value3",
		},
		err: nil,
	},

	// 5. Mixing multiple formats
	{
		name: "mixed formats",
		extraArgs: []string{
			"--v=4",               // Replace
			"--enable-audit",      // Add a Boolean flag.
			"--ports=8080", "443", // Add multiple values.
		},
		finalArgs: []string{
			"test",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
			"--enable-audit",
			"--ports=8080,443",
			"--v=4",
		},
		err: nil,
	},

	// 6. Replace multiple existing parameters
	{
		name: "replace multiple existing",
		extraArgs: []string{
			"--v=8",
			"--authorization-mode=RBAC",
		},
		finalArgs: []string{
			"test",
			"--runtime-config=",
			"--authorization-mode=RBAC",
			"--v=8",
		},
		err: nil,
	},

	// 7. Empty input
	{
		name:      "empty extra args",
		extraArgs: []string{},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
		},
		err: nil,
	},
	// 8. Includes equal sign
	{
		name:      "equals in key name",
		extraArgs: []string{"--key=name=value"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
			"--key=name=value", // This is legal because SplitN(arg, "=", 2) only splits at the first "=".
		},
		err: nil,
	},
	{
		name:      "multiple equals signs",
		extraArgs: []string{"--config=key1=value1=extra"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
			"--config=key1=value1=extra", // It's valid because it's only split at the first = sign.
		},
		err: nil,
	},
	// 9. Underscore
	{
		name:      "key with numbers and special chars",
		extraArgs: []string{"--key123_test_value=123"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
			"--key123_test_value=123",
		},
		err: nil,
	},
}

// User input error.
var errCase = []componentCommandCase{
	// 1. Missing the "--" prefix.
	{
		name:      "missing -- prefix",
		extraArgs: []string{"v=2"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
		},
		err: errors.New("want error"),
	},
	{
		name:      "single dash prefix",
		extraArgs: []string{"-v=2"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
		},
		err: errors.New("want error"),
	},

	// 2. Chinese comma issue
	{
		name:      "chinese comma in value",
		extraArgs: []string{"--ports=8080，443"}, // Chinese comma
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
			"--ports=8080，443",
		},
		err: nil,
	},
	{
		// --test-extra-args="--modes=read,write，execute"
		name:      "mixed comma types",
		extraArgs: []string{"--modes=read", "write，execute"}, // Mixed Chinese and English comma
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
			"--modes=read,write，execute",
		},
		err: nil,
	},

	// 3. Special characters and format errors
	{
		name:      "space in key name", // In this situation, it will return the default parameters directly.
		extraArgs: []string{"--my key=value"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
		},
		err: errors.New("want error"),
	},

	// 4. Null values and special cases
	{
		name:      "empty argument",
		extraArgs: []string{""},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
		},
		err: errors.New("want error"), // This is because the first parameter is an empty string, which will return an error when checking the prefix.
	},
	{
		name:      "only dashes",
		extraArgs: []string{"--"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
		},
		err: errors.New("want error"),
	},

	// 5. Illegal key naming
	{
		name:      "key starts with number",
		extraArgs: []string{"--123key=value"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
		},
		err: errors.New("want error"),
	},
	{
		name:      "key with special characters",
		extraArgs: []string{"--key@host=value"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
		}, // The "@" symbol is not allowed.
		err: errors.New("want error"),
	},

	// 6. Unicode and encoding issues
	{
		name:      "unicode characters in key",
		extraArgs: []string{"--配置=value"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
		},
		err: errors.New("want error"),
	},
	{
		name:      "emoji in value",
		extraArgs: []string{"--status=running😊"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
			"--status=running😊", // Special characters in values are usually allowed.
		},
		err: nil,
	},

	// 7. Isolated value (without a corresponding key)
	{
		name:      "orphaned value at start",
		extraArgs: []string{"orphaned", "--key=value"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC", // preProcessArgs will return default args.
		},
		err: errors.New("want error"),
	},
	{
		name:      "multiple orphaned values",
		extraArgs: []string{"value1", "value2", "--key=value"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC", // preProcessArgs will return default args.
		},
		err: errors.New("want error"),
	},

	// 8. Space and tab issues
	{
		name:      "leading/trailing spaces",
		extraArgs: []string{" --key=value "},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
		},
		err: errors.New("want error"),
	},
	{
		name:      "tab characters",
		extraArgs: []string{"--key=value\t"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
		},
		err: errors.New("want error"),
	},

	// 9. Repeated equal signs
	{
		name:      "double equals",
		extraArgs: []string{"--key==value"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
			"--key==value", // The first letter of this value is "=".
		},
		err: nil,
	},

	// 10. Extreme Length Test
	{
		name:      "very long key name",
		extraArgs: []string{"--" + strings.Repeat("a", 1000) + "=value"},
		finalArgs: []string{
			"test",
			"--v=6",
			"--runtime-config=",
			"--authorization-mode=Node,RBAC",
			"--" + strings.Repeat("a", 1000) + "=value",
		},
		err: nil,
	},
}

// KarmadaComponentCommand Integration test ValidateExtraArgs and MergeCommandArgs
// Note: this function is only used in tests
func KarmadaComponentCommand(defaultArgs, extraArgs []string) ([]string, error) {
	// Return directly without parameters.
	if len(extraArgs) == 0 {
		return defaultArgs, nil
	}

	// Validate Parameters
	args, err := ValidateExtraArgs(extraArgs)
	if err != nil {
		return defaultArgs, err
	}

	// merge Parameters
	return MergeCommandArgs(defaultArgs, args), nil
}

// Abstract a function that runs multiple test cases.
func testCase(cases []componentCommandCase, t *testing.T) {
	for _, c := range cases {
		got, err := KarmadaComponentCommand(defaultArgs, c.extraArgs)
		if err != nil {
			if c.err == nil {
				t.Errorf("test case [%s] failed, want: %v, got: %v", c.name, c.err, err)
			}
		} else {
			if c.err != nil {
				t.Errorf("test case [%s] failed, want: %v, got: %v", c.name, c.err, err)
			}
		}
		if !reflect.DeepEqual(got, c.finalArgs) {
			t.Errorf("test case [%s] failed, want: %v, got: %v", c.name, c.finalArgs, got)
		}
	}
}

// Define a test function for each group.
func testCommonCase(t *testing.T) {
	testCase(commonCases, t)
}

func testErrCase(t *testing.T) {
	testCase(errCase, t)
}

// Test function
func TestKarmadaComponentCommand(t *testing.T) {
	t.Run("CommonCase", testCommonCase)
	t.Run("ErrCase", testErrCase)
}

// Benchmark test
func BenchmarkKarmadaComponentCommand(b *testing.B) {
	for b.Loop() {
		_, _ = KarmadaComponentCommand(defaultArgs, commonCases[0].extraArgs)
	}
}
