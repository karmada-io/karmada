package defaultexplorer

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

// DefaultExplorer contains all default operation explorer factory
// for exploring common resource.
type DefaultExplorer struct {
	replicaHandlers map[schema.GroupVersionKind]replicaExplorer
	packingHandlers map[schema.GroupVersionKind]packingFactory
	healthyHandlers map[schema.GroupVersionKind]healthyFactory
}

// NewDefaultExplorer return a new DefaultExplorer.
func NewDefaultExplorer() *DefaultExplorer {
	return &DefaultExplorer{
		replicaHandlers: getAllDefaultReplicaExplorer(),
		packingHandlers: getAllDefaultPackingExplorer(),
		healthyHandlers: getAllDefaultHealthyExplorer(),
	}
}

// HookEnabled tells if any hook exist for specific resource type and operation type.
func (e *DefaultExplorer) HookEnabled(kind schema.GroupVersionKind, operationType configv1alpha1.OperationType) bool {
	switch operationType {
	case configv1alpha1.ExploreReplica:
		if _, exist := e.replicaHandlers[kind]; exist {
			return true
		}
		// TODO(RainbowMango): more cases should be added here
	}
	return false
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (e *DefaultExplorer) GetReplicas(object runtime.Object) (int32, *workv1alpha2.ReplicaRequirements, error) {
	_, exist := e.replicaHandlers[object.GetObjectKind().GroupVersionKind()]
	if !exist {
		return 0, &workv1alpha2.ReplicaRequirements{}, fmt.Errorf("defalut explorer for operation %s not found", configv1alpha1.ExploreReplica)
	}
	return e.replicaHandlers[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)](object)
}

// GetHealthy tells if the object in healthy state.
func (e *DefaultExplorer) GetHealthy(object runtime.Object) (bool, error) {
	// judge object type, and then get correct kind.
	_, exist := e.healthyHandlers[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)]
	if !exist {
		return false, fmt.Errorf("defalut explorer for operation %s not found", configv1alpha1.ExploreHealthy)
	}
	return e.healthyHandlers[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)](object)
}
