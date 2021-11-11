package defaultexplorer

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/karmada-io/karmada/pkg/util"
)

// packingFactory return default packing factory that can be used to
// retain necessary field and packing from the input object.
type packingFactory func(object runtime.Object) ([]byte, error)

func getAllDefaultPackingExplorer() map[schema.GroupVersionKind]packingFactory {
	explorers := make(map[schema.GroupVersionKind]packingFactory)
	explorers[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = deployPackingExplorer
	explorers[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = jobPackingExplorer
	return explorers
}

func deployPackingExplorer(object runtime.Object) ([]byte, error) {
	return nil, nil
}

func jobPackingExplorer(object runtime.Object) ([]byte, error) {
	return nil, nil
}
