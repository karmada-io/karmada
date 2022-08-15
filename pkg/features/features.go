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
)

var (
	// FeatureGate is a shared global FeatureGate.
	FeatureGate featuregate.MutableFeatureGate = featuregate.NewFeatureGate()

	defaultFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
		Failover:                          {Default: false, PreRelease: featuregate.Alpha},
		GracefulEviction:                  {Default: false, PreRelease: featuregate.Alpha},
		PropagateDeps:                     {Default: false, PreRelease: featuregate.Alpha},
		CustomizedClusterResourceModeling: {Default: false, PreRelease: featuregate.Alpha},
	}
)

func init() {
	runtime.Must(FeatureGate.Add(defaultFeatureGates))
}
