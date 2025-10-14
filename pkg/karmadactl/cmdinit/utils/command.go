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
	"fmt"
	"regexp"
	"sort"
	"strings"

	"k8s.io/klog/v2"
)

// ValidateExtraArgs preProcessArgs and  validateArgs
func ValidateExtraArgs(extraArgs []string) ([]string, error) {
	// Parameter preprocessing
	preprocessArgs, err := preProcessArgs(extraArgs)
	if err != nil {
		return nil, err
	}
	// Verification parameters
	args, err := validateArgs(preprocessArgs)
	if err != nil {
		klog.Errorf("%v", err)
		return nil, err
	}
	return args, nil
}

// preProcessArgs
// Because it is received as user parameters using pflag.StringSliceVar,
// when a user inputs: --xxx-extra-args="--k1=v1,v2,v3",
// the received parameter list is: [--k1=v1 v2 v3].
// Therefore, it needs to be preprocessed to convert it into [--key=v1,v2,v3].
func preProcessArgs(args []string) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}

	// Pre-create a slice with capacity.
	merged := make([]string, 0, len(args))
	var last string

	for _, raw := range args {
		arg := strings.TrimSpace(raw)
		if arg != raw {
			return nil, fmt.Errorf("argument %q contains leading/trailing whitespace", raw)
		}

		if strings.HasPrefix(arg, "--") {
			if last != "" {
				merged = append(merged, last)
			}
			last = arg
		} else {
			if last == "" {
				// This indicates that this is the first argument passed in from the command line,
				// but it does not have the prefix "--".
				return nil, fmt.Errorf("argument %q is invalid: no preceding --key found", arg)
			}
			last += "," + arg
		}
	}

	if last != "" {
		merged = append(merged, last)
	}
	return merged, nil
}

// Regular expression validation of user-provided parameters.
// format: --key=value or --key (e.g., --enable-pprof)
func validateArgs(args []string) ([]string, error) {
	argPattern := regexp.MustCompile(`^--[a-zA-Z][a-zA-Z0-9_-]*(=.*)?$`)
	for _, arg := range args {
		if !argPattern.MatchString(arg) {
			return nil, fmt.Errorf("invalid argument: %s", arg)
		}
	}
	return args, nil
}

// MergeCommandArgs merges defaultArgs with extraArgs, with extraArgs overriding defaults.
// It assumes extraArgs are already pre-processed and validated.
func MergeCommandArgs(defaultArgs, extraArgs []string) []string {
	extraArgsMap := make(map[string]string, len(extraArgs))
	for _, arg := range extraArgs {
		// Assuming extraArgs are already validated to start with "--" and be in --key=value or --key format
		// SplitN with limit 2 handles cases like --key=value1=value2 correctly, taking only the first '=' as delimiter
		parts := strings.SplitN(arg, "=", 2)
		key := parts[0]
		extraArgsMap[key] = arg
	}
	finalArgs := make([]string, 0, len(defaultArgs)+len(extraArgs))

	// First, add the command name if defaultArgs is not empty.
	if len(defaultArgs) > 0 {
		finalArgs = append(finalArgs, defaultArgs[0])
	}

	// Add default arguments, skipping any that are overridden by extraArgs.
	if len(defaultArgs) > 1 {
		for _, arg := range defaultArgs[1:] {
			parts := strings.SplitN(arg, "=", 2)
			key := parts[0]
			if _, ok := extraArgsMap[key]; !ok {
				finalArgs = append(finalArgs, arg)
			}
		}
	}

	// Add all extra arguments. To ensure deterministic output for tests, sort them.
	sortedExtraArgs := make([]string, 0, len(extraArgs))
	for _, arg := range extraArgsMap {
		sortedExtraArgs = append(sortedExtraArgs, arg)
	}

	sort.Strings(sortedExtraArgs)
	finalArgs = append(finalArgs, sortedExtraArgs...)

	return finalArgs
}
