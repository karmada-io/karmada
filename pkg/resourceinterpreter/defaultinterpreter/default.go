package defaultinterpreter

import (
	"context"
	"fmt"

	autoscalingapi "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/scale"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// DefaultInterpreter contains all default operation interpreter factory
// for interpreting common resource.
type DefaultInterpreter struct {
	restMapper        meta.RESTMapper
	scaleClient       scale.ScalesGetter
	scaleKindResolver scale.ScaleKindResolver

	replicaHandlers         map[schema.GroupVersionKind]replicaInterpreter
	reviseReplicaHandlers   map[schema.GroupVersionKind]reviseReplicaInterpreter
	retentionHandlers       map[schema.GroupVersionKind]retentionInterpreter
	aggregateStatusHandlers map[schema.GroupVersionKind]aggregateStatusInterpreter
	dependenciesHandlers    map[schema.GroupVersionKind]dependenciesInterpreter
	reflectStatusHandlers   map[schema.GroupVersionKind]reflectStatusInterpreter
	healthHandlers          map[schema.GroupVersionKind]healthInterpreter
}

// NewDefaultInterpreter return a new DefaultInterpreter.
func NewDefaultInterpreter(restMapper meta.RESTMapper, scaleClient scale.ScalesGetter, scaleKindResolver scale.ScaleKindResolver) *DefaultInterpreter {
	return &DefaultInterpreter{
		restMapper:        restMapper,
		scaleClient:       scaleClient,
		scaleKindResolver: scaleKindResolver,

		replicaHandlers:         getAllDefaultReplicaInterpreter(),
		reviseReplicaHandlers:   getAllDefaultReviseReplicaInterpreter(),
		retentionHandlers:       getAllDefaultRetentionInterpreter(),
		aggregateStatusHandlers: getAllDefaultAggregateStatusInterpreter(),
		dependenciesHandlers:    getAllDefaultDependenciesInterpreter(),
		reflectStatusHandlers:   getAllDefaultReflectStatusInterpreter(),
		healthHandlers:          getAllDefaultHealthInterpreter(),
	}
}

// HookEnabled tells if any hook exist for specific resource type and operation type.
func (e *DefaultInterpreter) HookEnabled(kind schema.GroupVersionKind, operationType configv1alpha1.InterpreterOperation) bool {
	switch operationType {
	case configv1alpha1.InterpreterOperationInterpretReplica:
		if _, exist := e.replicaHandlers[kind]; exist {
			return true
		}
		return e.checkScaleSubResourceExist(kind)
	case configv1alpha1.InterpreterOperationReviseReplica:
		if _, exist := e.reviseReplicaHandlers[kind]; exist {
			return true
		}
	case configv1alpha1.InterpreterOperationRetain:
		if _, exist := e.retentionHandlers[kind]; exist {
			return true
		}
	case configv1alpha1.InterpreterOperationAggregateStatus:
		if _, exist := e.aggregateStatusHandlers[kind]; exist {
			return true
		}
	case configv1alpha1.InterpreterOperationInterpretDependency:
		if _, exist := e.dependenciesHandlers[kind]; exist {
			return true
		}
	case configv1alpha1.InterpreterOperationInterpretStatus:
		return true
	case configv1alpha1.InterpreterOperationInterpretHealth:
		if _, exist := e.healthHandlers[kind]; exist {
			return true
		}
		// TODO(RainbowMango): more cases should be added here
	}

	klog.V(4).Infof("Default interpreter is not enabled for kind %q with operation %q.", kind, operationType)
	return false
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (e *DefaultInterpreter) GetReplicas(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {
	handler, exist := e.replicaHandlers[object.GroupVersionKind()]
	if exist {
		return handler(object)
	}

	scaleObj, err := e.getScaleSubResource(object)
	if err != nil {
		return 0, nil, err
	}
	return scaleObj.Spec.Replicas, nil, nil
}

// ReviseReplica revises the replica of the given object.
func (e *DefaultInterpreter) ReviseReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error) {
	handler, exist := e.reviseReplicaHandlers[object.GroupVersionKind()]
	if !exist {
		return nil, fmt.Errorf("default %s interpreter for %q not found", configv1alpha1.InterpreterOperationReviseReplica, object.GroupVersionKind())
	}
	return handler(object, replica)
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (e *DefaultInterpreter) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error) {
	handler, exist := e.retentionHandlers[desired.GroupVersionKind()]
	if !exist {
		return nil, fmt.Errorf("default %s interpreter for %q not found", configv1alpha1.InterpreterOperationRetain, desired.GroupVersionKind())
	}
	return handler(desired, observed)
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (e *DefaultInterpreter) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	handler, exist := e.aggregateStatusHandlers[object.GroupVersionKind()]
	if !exist {
		return nil, fmt.Errorf("default %s interpreter for %q not found", configv1alpha1.InterpreterOperationAggregateStatus, object.GroupVersionKind())
	}
	return handler(object, aggregatedStatusItems)
}

// GetDependencies returns the dependent resources of the given object.
func (e *DefaultInterpreter) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, err error) {
	handler, exist := e.dependenciesHandlers[object.GroupVersionKind()]
	if !exist {
		return dependencies, fmt.Errorf("default interpreter for operation %s not found", configv1alpha1.InterpreterOperationInterpretDependency)
	}
	return handler(object)
}

// ReflectStatus returns the status of the object.
func (e *DefaultInterpreter) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, err error) {
	handler, exist := e.reflectStatusHandlers[object.GroupVersionKind()]
	if exist {
		return handler(object)
	}

	// for resource types that don't have a build-in handler, try to collect the whole status from '.status' filed.
	return reflectWholeStatus(object)
}

// InterpretHealth returns the health state of the object.
func (e *DefaultInterpreter) InterpretHealth(object *unstructured.Unstructured) (bool, error) {
	handler, exist := e.healthHandlers[object.GroupVersionKind()]
	if exist {
		return handler(object)
	}

	return false, fmt.Errorf("default %s interpreter for %q not found", configv1alpha1.InterpreterOperationInterpretHealth, object.GroupVersionKind())
}

func (e *DefaultInterpreter) checkScaleSubResourceExist(kind schema.GroupVersionKind) bool {
	mapping, err := e.restMapper.RESTMapping(kind.GroupKind(), kind.Version)
	if err != nil {
		klog.Errorf("Failed to get RESTMapping of resource %v: %v", kind, err)
		return false
	}

	gvr := mapping.GroupVersionKind.GroupVersion().WithResource(mapping.Resource.Resource)
	if _, err = e.scaleKindResolver.ScaleForResource(gvr); err != nil {
		klog.Warningf("Cannot get scale subresource for %v: %v", gvr, err)
		return false
	}
	return true
}

func (e *DefaultInterpreter) getScaleSubResource(object *unstructured.Unstructured) (*autoscalingapi.Scale, error) {
	mapping, err := e.restMapper.RESTMapping(object.GroupVersionKind().GroupKind(), object.GroupVersionKind().Version)
	if err != nil {
		klog.Errorf("Failed to get RESTMapping of resource %v: %v", object.GroupVersionKind(), err)
		return nil, err
	}

	return e.scaleClient.Scales(object.GetNamespace()).Get(context.TODO(), mapping.Resource.GroupResource(),
		object.GetName(), metav1.GetOptions{})
}
