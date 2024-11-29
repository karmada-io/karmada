/*
Copyright 2021 The Karmada Authors.

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

package features

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	// Failover indicates if scheduler should reschedule on cluster failure.
	Failover featuregate.Feature = "Failover"

	// GracefulEviction indicates if enable grace eviction.
	// Takes effect only when the Failover feature is enabled.
	GracefulEviction featuregate.Feature = "GracefulEviction"

	// PropagateDeps indicates if relevant resources should be propagated automatically
	PropagateDeps featuregate.Feature = "PropagateDeps"

	// CustomizedClusterResourceModeling indicates if enable cluster resource custom modeling.
	CustomizedClusterResourceModeling featuregate.Feature = "CustomizedClusterResourceModeling"

	// PolicyPreemption indicates if high-priority PropagationPolicy/ClusterPropagationPolicy could preempt resource templates which are matched by low-priority PropagationPolicy/ClusterPropagationPolicy.
	PolicyPreemption featuregate.Feature = "PropagationPolicyPreemption"

	// MultiClusterService indicates if enable multi-cluster service function.
	MultiClusterService featuregate.Feature = "MultiClusterService"

	// ResourceQuotaEstimate indicates if enable resource quota check in estimator
	ResourceQuotaEstimate featuregate.Feature = "ResourceQuotaEstimate"

	// StatefulFailoverInjection controls whether Karmada collects state information
	// from the source cluster during a failover event for stateful applications and
	// injects this information into the application configuration when it is moved
	// to the target cluster.
	//
	// owner: @mszacillo, @XiShanYongYe-Chang
	// alpha: v1.12
	StatefulFailoverInjection featuregate.Feature = "StatefulFailoverInjection"
)

var (
	// FeatureGate is a shared global FeatureGate.
	FeatureGate featuregate.MutableFeatureGate = featuregate.NewFeatureGate()

	// DefaultFeatureGates is the default feature gates of Karmada.
	DefaultFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
		Failover:                          {Default: true, PreRelease: featuregate.Beta},
		GracefulEviction:                  {Default: true, PreRelease: featuregate.Beta},
		PropagateDeps:                     {Default: true, PreRelease: featuregate.Beta},
		CustomizedClusterResourceModeling: {Default: true, PreRelease: featuregate.Beta},
		PolicyPreemption:                  {Default: false, PreRelease: featuregate.Alpha},
		MultiClusterService:               {Default: false, PreRelease: featuregate.Alpha},
		ResourceQuotaEstimate:             {Default: false, PreRelease: featuregate.Alpha},
		StatefulFailoverInjection:         {Default: false, PreRelease: featuregate.Alpha},
	}
)

func init() {
	runtime.Must(FeatureGate.Add(DefaultFeatureGates))
}
