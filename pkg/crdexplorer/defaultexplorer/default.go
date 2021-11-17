package defaultexplorer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// DefaultExplorer contains all default operation explorer factory
// for exploring common resource.
type DefaultExplorer struct {
	replicaHandlers   map[schema.GroupVersionKind]replicaExplorer
	retentionHandlers map[schema.GroupVersionKind]retentionExplorer
}

// NewDefaultExplorer return a new DefaultExplorer.
func NewDefaultExplorer() *DefaultExplorer {
	return &DefaultExplorer{
		replicaHandlers:   getAllDefaultReplicaExplorer(),
		retentionHandlers: getAllDefaultRetentionExplorer(),
	}
}

// HookEnabled tells if any hook exist for specific resource type and operation type.
func (e *DefaultExplorer) HookEnabled(kind schema.GroupVersionKind, operationType configv1alpha1.OperationType) bool {
	switch operationType {
	case configv1alpha1.ExploreReplica:
		if _, exist := e.replicaHandlers[kind]; exist {
			return true
		}
	case configv1alpha1.ExploreRetaining:
		if _, exist := e.retentionHandlers[kind]; exist {
			return true
		}

		// TODO(RainbowMango): more cases should be added here
	}

	klog.V(4).Infof("Hook is not enabled: %q in %q is not supported.", kind, operationType)
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

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (e *DefaultExplorer) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error) {
	handler, exist := e.retentionHandlers[desired.GroupVersionKind()]
	if !exist {
		return nil, fmt.Errorf("default retain explorer for %q not found", desired.GroupVersionKind())
	}

	return handler(desired, observed)
}
