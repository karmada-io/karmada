package features

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	// Failover indicates if scheduler should reschedule on cluster failure.
	Failover featuregate.Feature = "Failover"
)

var (
	// FeatureGate is a shared global FeatureGate.
	FeatureGate featuregate.MutableFeatureGate = featuregate.NewFeatureGate()

	defaultFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
		Failover: {Default: false, PreRelease: featuregate.Alpha},
	}
)

func init() {
	runtime.Must(FeatureGate.Add(defaultFeatureGates))
}
