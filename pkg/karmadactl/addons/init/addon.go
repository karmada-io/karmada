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

package init

const (
	// AddonDisabledStatus describe a karmada addon is not installed
	AddonDisabledStatus = "disabled"

	// AddonEnabledStatus describe a karmada addon is installed
	AddonEnabledStatus = "enabled"

	// AddonUnhealthyStatus describe a karmada addon is unhealthy
	AddonUnhealthyStatus = "unhealthy"

	// AddonUnknownStatus describe a karmada addon is unknown
	AddonUnknownStatus = "unknown"
)

// Addons hosts the optional components that support by karmada
var Addons = map[string]*Addon{}

// Addon describe how to enable or disable an optional component that support by karmada
type Addon struct {
	Name string

	// Status return current addon install status
	Status func(opts *CommandAddonsListOption) (string, error)

	// Enable install current addon in host cluster and Karmada control plane
	Enable func(opts *CommandAddonsEnableOption) error

	// Disable uninstall current addon in host cluster and Karmada control plane
	Disable func(opts *CommandAddonsDisableOption) error
}
