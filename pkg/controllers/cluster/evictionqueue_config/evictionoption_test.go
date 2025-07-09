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

package config

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestEvictionControllerOptions_AddFlags(t *testing.T) {
	config := &EvictionQueueOptions{}
	fs := pflag.NewFlagSet("test", pflag.ExitOnError)
	config.AddFlags(fs)

	expectedFlags := []string{
		"resource-eviction-rate",
		"secondary-resource-eviction-rate",
		"unhealthy-cluster-threshold",
		"large-cluster-num-threshold",
	}

	for _, flagName := range expectedFlags {
		if fs.Lookup(flagName) == nil {
			t.Errorf("Expected flag %s not found", flagName)
		}
	}
}

func TestEvictionControllerOptions_AddFlags_NilReceiver(t *testing.T) {
	var config *EvictionQueueOptions
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	config.AddFlags(fs)

	if fs.HasFlags() {
		t.Error("Expected no flags to be added when receiver is nil, but flags were added")
	}
}
