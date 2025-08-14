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

package utils

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"k8s.io/klog/v2"
)

// KarmadaComponentCommand merges default parameters with user-provided parameters.
func KarmadaComponentCommand(defaultArgs, extraArgs []string) []string {
	// Return directly without parameters.
	if len(extraArgs) == 0 {
		return defaultArgs
	}

	// Parameter preprocessing
	preprocessArgs := preProcessArgs(extraArgs)

	// Verification parameters
	args, err := validateArgs(preprocessArgs)
	if err != nil {
		klog.Errorf("%v", err)
		return nil
	}

	// merge Parameters
	return mergeCommandArgs(defaultArgs, args)
}

// Parameter preprocessing  // has --key=v1,v2,v3.
func preProcessArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	// Pre-create a slice with capacity.
	merged := make([]string, 0, len(args))
	var last string

	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			if last != "" {
				merged = append(merged, last)
			}
			last = arg
		} else {
			if last == "" {
				// The explanation is the first parameter, at this time skip and warn.
				klog.Warningf("argument %q ignored: no preceding --key found", arg)
				continue
			}
			last += "," + arg
		}
	}

	if last != "" {
		merged = append(merged, last)
	}
	return merged
}

// Regular expression validation of user-provided parameters.
// format: --key=value or --key
func validateArgs(args []string) ([]string, error) {
	// Modified regex to allow flags without values (e.g., --enable-pprof)
	argPattern := regexp.MustCompile(`^--[a-zA-Z0-9\-]+(=.*)?$`)
	for _, arg := range args {
		if !argPattern.MatchString(arg) {
			return nil, fmt.Errorf("invalid argument: %s", arg)
		}
	}
	return args, nil
}

// mergeCommandArgs merges defaultArgs with extraArgs, with extraArgs overriding defaults.
// It assumes extraArgs are already pre-processed and validated.
func mergeCommandArgs(defaultArgs, extraArgs []string) []string {
	extraArgsMap := make(map[string]string)
	for _, arg := range extraArgs {
		// Assuming extraArgs are already validated to start with "--" and be in --key=value or --key format
		// SplitN with limit 2 handles cases like --key=value1=value2 correctly, taking only the first '=' as delimiter
		parts := strings.SplitN(arg, "=", 2)
		key := parts[0]
		extraArgsMap[key] = arg // Store the full argument string
	}
	var finalArgs []string

	// First, add the command name if defaultArgs is not empty.
	if len(defaultArgs) > 0 {
		finalArgs = append(finalArgs, defaultArgs[0])
	}

	// Add default arguments, skipping any that are overridden by extraArgs.
	for _, arg := range defaultArgs[1:] {
		parts := strings.SplitN(arg, "=", 2)
		key := parts[0]
		if _, ok := extraArgsMap[key]; !ok {
			finalArgs = append(finalArgs, arg)
		}
	}

	// Add all extra arguments. To ensure deterministic output for tests, sort them.
	var sortedExtraArgs []string
	for _, arg := range extraArgsMap {
		sortedExtraArgs = append(sortedExtraArgs, arg)
	}

	sort.Strings(sortedExtraArgs)
	finalArgs = append(finalArgs, sortedExtraArgs...)

	return finalArgs
}
