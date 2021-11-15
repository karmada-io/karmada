package defaultexplorer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
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
func (e *DefaultExplorer) GetReplicas(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {
	handler, exist := e.replicaHandlers[object.GroupVersionKind()]
	if !exist {
		return 0, &workv1alpha2.ReplicaRequirements{}, fmt.Errorf("defalut explorer for operation %s not found", configv1alpha1.ExploreReplica)
	}
	return handler(object)
}

// GetHealthy tells if the object in healthy state.
func (e *DefaultExplorer) GetHealthy(object *unstructured.Unstructured) (bool, error) {
	handler, exist := e.healthyHandlers[object.GroupVersionKind()]
	if !exist {
		return false, fmt.Errorf("defalut explorer for operation %s not found", configv1alpha1.ExploreHealthy)
	}
	return handler(object)
}
