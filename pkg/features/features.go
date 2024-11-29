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
	// Failover controls whether the scheduler should reschedule
	// workloads on cluster failure.
	// When enabled, Karmada will automatically migrate workloads
	// from a failed cluster to other available clusters.
	//
	// Note: This feature does not control application failover,
	// which is managed separately via the PropagationPolicy or
	// ClusterPropagationPolicy.
	Failover featuregate.Feature = "Failover"

	// GracefulEviction controls whether to perform graceful evictions
	// during both cluster failover and application failover.
	// When used for cluster failover, it takes effect only when the
	// Failover feature is enabled.
	// Graceful eviction ensures that workloads are migrated in a
	// controlled manner, minimizing disruption to applications.
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
		// Failover(cluster failover) is disabled by default because it involves migrating
		// all resources in the cluster, which can have significant impacts, it should be
		// explicitly enabled by administrators after fully evaluation to avoid unexpected
		// incidents.
		Failover:                          {Default: false, PreRelease: featuregate.Beta},
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
