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

package util

const (
	// TagCommandGroup used for tag the group of the command
	TagCommandGroup = "commandGroup"
)

const (
	// GroupBasic means the command belongs to Group "Basic Commands"
	GroupBasic = "Basic Commands"

	// GroupClusterRegistration means the command belongs to Group "Cluster Registration Commands"
	GroupClusterRegistration = "Cluster Registration Commands"

	// GroupClusterManagement means the command belongs to Group "Cluster Management Commands"
	GroupClusterManagement = "Cluster Management Commands"

	// GroupClusterTroubleshootingAndDebugging means the command belongs to Group "Troubleshooting and Debugging Commands"
	GroupClusterTroubleshootingAndDebugging = "Troubleshooting and Debugging Commands"

	// GroupAdvancedCommands means the command belongs to Group "Advanced Commands"
	GroupAdvancedCommands = "Advanced Commands"

	// GroupSettingsCommands means the command belongs to Group "Settings Commands"
	GroupSettingsCommands = "Settings Commands"

	// GroupOtherCommands means the command belongs to Group "Other Commands"
	GroupOtherCommands = "Other Commands"
)
