package defaultexplorer

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/karmada-io/karmada/pkg/util"
)

// healthyFactory return default healthy factory that tells if the object in healthy state.
type healthyFactory func(object runtime.Object) (bool, error)

func getAllDefaultHealthyExplorer() map[schema.GroupVersionKind]healthyFactory {
	explorers := make(map[schema.GroupVersionKind]healthyFactory)
	explorers[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = deployHealthyExplorer
	explorers[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = jobHealthyExplorer
	return explorers
}

func deployHealthyExplorer(object runtime.Object) (bool, error) {
	return false, nil
}

func jobHealthyExplorer(object runtime.Object) (bool, error) {
	return false, nil
}
