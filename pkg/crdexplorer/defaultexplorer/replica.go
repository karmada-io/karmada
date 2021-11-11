package defaultexplorer

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

// replicaFactory return default replica factory that can be used to obtain replica
// and requirements by each replica from the input object.
type replicaFactory func(object runtime.Object) (int32, *workv1alpha2.ReplicaRequirements, error)

func getAllDefaultReplicaExplorer() map[schema.GroupVersionKind]replicaFactory {
	explorers := make(map[schema.GroupVersionKind]replicaFactory)
	explorers[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = deployReplicaExplorer
	explorers[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = jobReplicaExplorer
	return explorers
}

func deployReplicaExplorer(object runtime.Object) (int32, *workv1alpha2.ReplicaRequirements, error) {
	return 0, &workv1alpha2.ReplicaRequirements{}, nil
}

func jobReplicaExplorer(object runtime.Object) (int32, *workv1alpha2.ReplicaRequirements, error) {
	return 0, &workv1alpha2.ReplicaRequirements{}, nil
}
