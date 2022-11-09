package resourceinterpreter

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/configurableinterpreter"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customizedinterpreter"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/defaultinterpreter"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/framework"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

// ResourceInterpreter manages both default and customized webhooks to interpret custom resource structure.
type ResourceInterpreter interface {
	// Start starts running the component and will never stop running until the context is closed or an error occurs.
	Start(ctx context.Context) (err error)

	// HookEnabled tells if any hook exist for specific resource type and operation.
	HookEnabled(objGVK schema.GroupVersionKind, operationType configv1alpha1.InterpreterOperation) bool

	// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
	GetReplicas(object *unstructured.Unstructured) (replica int32, replicaRequires *workv1alpha2.ReplicaRequirements, err error)

	// ReviseReplica revises the replica of the given object.
	ReviseReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error)

	// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
	Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error)

	// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
	AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error)

	// GetDependencies returns the dependent resources of the given object.
	GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, err error)

	// ReflectStatus returns the status of the object.
	ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, err error)

	// InterpretHealth returns the health state of the object.
	InterpretHealth(object *unstructured.Unstructured) (healthy bool, err error)

	// other common method
}

// NewResourceInterpreter builds a new ResourceInterpreter object.
func NewResourceInterpreter(informer genericmanager.SingleClusterInformerManager) ResourceInterpreter {
	return &customResourceInterpreterImpl{
		informer: informer,
	}
}

type customResourceInterpreterImpl struct {
	informer     genericmanager.SingleClusterInformerManager
	interpreters []framework.Interpreter
}

// Start starts running the component and will never stop running until the context is closed or an error occurs.
func (i *customResourceInterpreterImpl) Start(ctx context.Context) (err error) {
	klog.Infof("Starting resource interpreter.")
	var interpreter framework.Interpreter
	interpreterFactories := []func(genericmanager.SingleClusterInformerManager) (framework.Interpreter, error){
		func(manager genericmanager.SingleClusterInformerManager) (framework.Interpreter, error) {
			interpreter, err = configurableinterpreter.NewConfigurableInterpreter(manager)
			return interpreter, err
		},
		customizedinterpreter.NewCustomizedInterpreter,
		defaultinterpreter.NewDefaultInterpreter,
	}
	for _, factory := range interpreterFactories {
		interpreter, err = factory(i.informer)
		if err != nil {
			return err
		}
		i.interpreters = append(i.interpreters, interpreter)
	}
	i.informer.Start()
	<-ctx.Done()
	klog.Infof("Stopped as stopCh closed.")
	return nil
}

// HookEnabled tells if any hook exist for specific resource type and operation.
func (i *customResourceInterpreterImpl) HookEnabled(objGVK schema.GroupVersionKind, operation configv1alpha1.InterpreterOperation) bool {
	for _, interpreter := range i.interpreters {
		if interpreter.HookEnabled(objGVK, operation) {
			return true
		}
	}
	return false
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (i *customResourceInterpreterImpl) GetReplicas(object *unstructured.Unstructured) (replica int32, requires *workv1alpha2.ReplicaRequirements, err error) {
	for _, interpreter := range i.interpreters {
		replica, requires, enabled, err := interpreter.GetReplicas(object)
		if err != nil || enabled {
			return replica, requires, err
		}
	}
	return 0, &workv1alpha2.ReplicaRequirements{}, fmt.Errorf("resource interpreter %s for %q not found", configv1alpha1.InterpreterOperationInterpretReplica, object.GroupVersionKind())
}

// ReviseReplica revises the replica of the given object.
func (i *customResourceInterpreterImpl) ReviseReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error) {
	for _, interpreter := range i.interpreters {
		obj, enabled, err := interpreter.ReviseReplica(object, replica)
		if err != nil || enabled {
			return obj, err
		}
	}
	return nil, fmt.Errorf("resource interpreter %s for %q not found", configv1alpha1.InterpreterOperationReviseReplica, object.GroupVersionKind())
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (i *customResourceInterpreterImpl) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	for _, interpreter := range i.interpreters {
		retained, enabled, err := interpreter.Retain(desired, observed)
		if err != nil || enabled {
			return retained, err
		}
	}
	return nil, fmt.Errorf("resource interpreter %s for %q not found", configv1alpha1.InterpreterOperationRetain, desired.GroupVersionKind())
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (i *customResourceInterpreterImpl) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	for _, interpreter := range i.interpreters {
		obj, enabled, err := interpreter.AggregateStatus(object, aggregatedStatusItems)
		if err != nil || enabled {
			return obj, err
		}
	}
	return nil, fmt.Errorf("resource interpreter %s for %q not found", configv1alpha1.InterpreterOperationAggregateStatus, object.GroupVersionKind())
}

// GetDependencies returns the dependent resources of the given object.
func (i *customResourceInterpreterImpl) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, err error) {
	for _, interpreter := range i.interpreters {
		getDependencies, enabled, err := interpreter.GetDependencies(object)
		if err != nil || enabled {
			return getDependencies, err
		}
	}
	return nil, fmt.Errorf("resource interpreter %s operation %s not found", configv1alpha1.InterpreterOperationInterpretDependency, object.GroupVersionKind())
}

// ReflectStatus returns the status of the object.
func (i *customResourceInterpreterImpl) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, err error) {
	for _, interpreter := range i.interpreters {
		reflectStatus, enabled, err := interpreter.ReflectStatus(object)
		if err != nil || enabled {
			return reflectStatus, err
		}
	}
	return nil, fmt.Errorf("resource interpreter %s for operation %s not found", configv1alpha1.InterpreterOperationInterpretStatus, object.GroupVersionKind())
}

// InterpretHealth returns the health state of the object.
func (i *customResourceInterpreterImpl) InterpretHealth(object *unstructured.Unstructured) (healthy bool, err error) {
	for _, interpreter := range i.interpreters {
		health, enabled, err := interpreter.InterpretHealth(object)
		if err != nil || enabled {
			return health, err
		}
	}
	return false, fmt.Errorf("resource interpreter %s for %q not found", configv1alpha1.InterpreterOperationInterpretHealth, object.GroupVersionKind())
}
