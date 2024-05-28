/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package native

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/interpreter/validation"
)

// DefaultInterpreter contains all default operation interpreter factory
// for interpreting common resource.
type DefaultInterpreter struct {
	replicaHandlers         map[schema.GroupVersionKind]replicaInterpreter
	reviseReplicaHandlers   map[schema.GroupVersionKind]reviseReplicaInterpreter
	retentionHandlers       map[schema.GroupVersionKind]retentionInterpreter
	aggregateStatusHandlers map[schema.GroupVersionKind]aggregateStatusInterpreter
	dependenciesHandlers    map[schema.GroupVersionKind]dependenciesInterpreter
	reflectStatusHandlers   map[schema.GroupVersionKind]reflectStatusInterpreter
	healthHandlers          map[schema.GroupVersionKind]healthInterpreter
}

// NewDefaultInterpreter return a new DefaultInterpreter.
func NewDefaultInterpreter() *DefaultInterpreter {
	return &DefaultInterpreter{
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
	klog.V(4).Infof("Get replicas for object: %v %s/%s with build-in interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())
	handler, exist := e.replicaHandlers[object.GroupVersionKind()]
	if !exist {
		return 0, &workv1alpha2.ReplicaRequirements{}, fmt.Errorf("default %s interpreter for %q not found", configv1alpha1.InterpreterOperationInterpretReplica, object.GroupVersionKind())
	}
	return handler(object)
}

// ReviseReplica revises the replica of the given object.
func (e *DefaultInterpreter) ReviseReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error) {
	klog.V(4).Infof("Revise replicas for object: %v %s/%s with build-in interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())
	handler, exist := e.reviseReplicaHandlers[object.GroupVersionKind()]
	if !exist {
		return nil, fmt.Errorf("default %s interpreter for %q not found", configv1alpha1.InterpreterOperationReviseReplica, object.GroupVersionKind())
	}
	return handler(object, replica)
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (e *DefaultInterpreter) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error) {
	klog.V(4).Infof("Retain object: %v %s/%s with build-in interpreter.", desired.GroupVersionKind(), desired.GetNamespace(), desired.GetName())
	handler, exist := e.retentionHandlers[desired.GroupVersionKind()]
	if !exist {
		return nil, fmt.Errorf("default %s interpreter for %q not found", configv1alpha1.InterpreterOperationRetain, desired.GroupVersionKind())
	}
	return handler(desired, observed)
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (e *DefaultInterpreter) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	klog.V(4).Infof("Aggregate status of object: %v %s/%s with build-in interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())
	handler, exist := e.aggregateStatusHandlers[object.GroupVersionKind()]
	if !exist {
		return nil, fmt.Errorf("default %s interpreter for %q not found", configv1alpha1.InterpreterOperationAggregateStatus, object.GroupVersionKind())
	}
	return handler(object, aggregatedStatusItems)
}

// GetDependencies returns the dependent resources of the given object.
func (e *DefaultInterpreter) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, err error) {
	klog.V(4).Infof("Get dependencies of object: %v %s/%s with build-in interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())
	handler, exist := e.dependenciesHandlers[object.GroupVersionKind()]
	if !exist {
		return dependencies, fmt.Errorf("default interpreter for operation %s not found", configv1alpha1.InterpreterOperationInterpretDependency)
	}

	dependencies, err = handler(object)
	if err != nil {
		return
	}
	return dependencies, validation.VerifyDependencies(dependencies)
}

// ReflectStatus returns the status of the object.
func (e *DefaultInterpreter) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, err error) {
	klog.V(4).Infof("Reflect status of object: %v %s/%s with build-in interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())
	handler, exist := e.reflectStatusHandlers[object.GroupVersionKind()]
	if exist {
		return handler(object)
	}

	// for resource types that don't have a build-in handler, try to collect the whole status from '.status' filed.
	return reflectWholeStatus(object)
}

// InterpretHealth returns the health state of the object.
func (e *DefaultInterpreter) InterpretHealth(object *unstructured.Unstructured) (bool, error) {
	klog.V(4).Infof("Get health status of object: %v %s/%s with build-in interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())
	handler, exist := e.healthHandlers[object.GroupVersionKind()]
	if exist {
		return handler(object)
	}

	return false, fmt.Errorf("default %s interpreter for %q not found", configv1alpha1.InterpreterOperationInterpretHealth, object.GroupVersionKind())
}
