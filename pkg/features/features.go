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
	}
)

func init() {
	runtime.Must(FeatureGate.Add(DefaultFeatureGates))
}
