/*
Copyright 2020 The Kubernetes Authors.

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

// Package feature implements feature functionality.
package feature

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	// Every feature gate should add method here following this template:
	//
	// // owner: @username
	// // alpha: v1.X
	// MyFeature featuregate.Feature = "MyFeature".

	// MachinePool is a feature gate for MachinePool functionality.
	//
	// alpha: v0.3
	MachinePool featuregate.Feature = "MachinePool"

	// ClusterResourceSet is a feature gate for the ClusterResourceSet functionality.
	//
	// alpha: v0.3
	// beta: v0.4
	ClusterResourceSet featuregate.Feature = "ClusterResourceSet"

	// ClusterTopology is a feature gate for the ClusterClass and managed topologies functionality.
	//
	// alpha: v0.4
	ClusterTopology featuregate.Feature = "ClusterTopology"

	// RuntimeSDK is a feature gate for the Runtime hooks and extensions functionality.
	//
	// alpha: v1.2
	RuntimeSDK featuregate.Feature = "RuntimeSDK"

	// KubeadmBootstrapFormatIgnition is a feature gate for the Ignition bootstrap format
	// functionality.
	//
	// alpha: v1.1
	KubeadmBootstrapFormatIgnition featuregate.Feature = "KubeadmBootstrapFormatIgnition"
)

func init() {
	runtime.Must(MutableGates.Add(defaultClusterAPIFeatureGates))
}

// defaultClusterAPIFeatureGates consists of all known cluster-api-specific feature keys.
// To add a new feature, define a key for it above and add it here.
var defaultClusterAPIFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	// Every feature should be initiated here:
	MachinePool:                    {Default: false, PreRelease: featuregate.Alpha},
	ClusterResourceSet:             {Default: true, PreRelease: featuregate.Beta},
	ClusterTopology:                {Default: false, PreRelease: featuregate.Alpha},
	KubeadmBootstrapFormatIgnition: {Default: false, PreRelease: featuregate.Alpha},
	RuntimeSDK:                     {Default: false, PreRelease: featuregate.Alpha},
}
