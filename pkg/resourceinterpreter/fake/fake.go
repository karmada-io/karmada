package fake

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
)

var _ resourceinterpreter.ResourceInterpreter = &FakeInterpreter{}

type (
	GetReplicasFunc     = func(*unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error)
	ReviseReplicaFunc   = func(*unstructured.Unstructured, int64) (*unstructured.Unstructured, error)
	RetainFunc          = func(*unstructured.Unstructured, *unstructured.Unstructured) (*unstructured.Unstructured, error)
	AggregateStatusFunc = func(*unstructured.Unstructured, []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error)
	GetDependenciesFunc = func(*unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error)
	ReflectStatusFunc   = func(*unstructured.Unstructured) (*runtime.RawExtension, error)
	InterpretHealthFunc = func(*unstructured.Unstructured) (bool, error)
)

type FakeInterpreter struct {
	getReplicasFunc     map[schema.GroupVersionKind]GetReplicasFunc
	reviseReplicaFunc   map[schema.GroupVersionKind]ReviseReplicaFunc
	retainFunc          map[schema.GroupVersionKind]RetainFunc
	aggregateStatusFunc map[schema.GroupVersionKind]AggregateStatusFunc
	getDependenciesFunc map[schema.GroupVersionKind]GetDependenciesFunc
	reflectStatusFunc   map[schema.GroupVersionKind]ReflectStatusFunc
	interpretHealthFunc map[schema.GroupVersionKind]InterpretHealthFunc
}

func (f *FakeInterpreter) Start(ctx context.Context) (err error) {
	return nil
}

func (f *FakeInterpreter) HookEnabled(objGVK schema.GroupVersionKind, operationType configv1alpha1.InterpreterOperation) bool {
	var exist bool
	switch operationType {
	case configv1alpha1.InterpreterOperationInterpretReplica:
		_, exist = f.getReplicasFunc[objGVK]
	case configv1alpha1.InterpreterOperationReviseReplica:
		_, exist = f.reviseReplicaFunc[objGVK]
	case configv1alpha1.InterpreterOperationRetain:
		_, exist = f.retainFunc[objGVK]
	case configv1alpha1.InterpreterOperationAggregateStatus:
		_, exist = f.aggregateStatusFunc[objGVK]
	case configv1alpha1.InterpreterOperationInterpretDependency:
		_, exist = f.getDependenciesFunc[objGVK]
	case configv1alpha1.InterpreterOperationInterpretStatus:
		_, exist = f.reflectStatusFunc[objGVK]
	case configv1alpha1.InterpreterOperationInterpretHealth:
		_, exist = f.interpretHealthFunc[objGVK]
	default:
		exist = false
	}

	return exist
}

func (f *FakeInterpreter) GetReplicas(object *unstructured.Unstructured) (replica int32, requires *workv1alpha2.ReplicaRequirements, err error) {
	return f.getReplicasFunc[object.GetObjectKind().GroupVersionKind()](object)
}

func (f *FakeInterpreter) ReviseReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error) {
	return f.reviseReplicaFunc[object.GetObjectKind().GroupVersionKind()](object, replica)
}

func (f *FakeInterpreter) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return f.retainFunc[observed.GetObjectKind().GroupVersionKind()](desired, observed)
}

func (f *FakeInterpreter) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	return f.aggregateStatusFunc[object.GetObjectKind().GroupVersionKind()](object, aggregatedStatusItems)
}

func (f *FakeInterpreter) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, err error) {
	return f.getDependenciesFunc[object.GetObjectKind().GroupVersionKind()](object)
}

func (f *FakeInterpreter) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, err error) {
	return f.reflectStatusFunc[object.GetObjectKind().GroupVersionKind()](object)
}

func (f *FakeInterpreter) InterpretHealth(object *unstructured.Unstructured) (healthy bool, err error) {
	return f.interpretHealthFunc[object.GetObjectKind().GroupVersionKind()](object)
}

func NewFakeInterpreter() *FakeInterpreter {
	return &FakeInterpreter{}
}

func (f *FakeInterpreter) WithGetReplicas(objGVK schema.GroupVersionKind, iFunc GetReplicasFunc) *FakeInterpreter {
	if f.getReplicasFunc == nil {
		f.getReplicasFunc = make(map[schema.GroupVersionKind]GetReplicasFunc)
	}
	f.getReplicasFunc[objGVK] = iFunc

	return f
}

func (f *FakeInterpreter) WithReviseReplica(objGVK schema.GroupVersionKind, iFunc ReviseReplicaFunc) *FakeInterpreter {
	if f.reviseReplicaFunc == nil {
		f.reviseReplicaFunc = make(map[schema.GroupVersionKind]ReviseReplicaFunc)
	}
	f.reviseReplicaFunc[objGVK] = iFunc

	return f
}

func (f *FakeInterpreter) WithRetain(objGVK schema.GroupVersionKind, iFunc RetainFunc) *FakeInterpreter {
	if f.retainFunc == nil {
		f.retainFunc = make(map[schema.GroupVersionKind]RetainFunc)
	}
	f.retainFunc[objGVK] = iFunc

	return f
}

func (f *FakeInterpreter) WithAggregateStatus(objGVK schema.GroupVersionKind, iFunc AggregateStatusFunc) *FakeInterpreter {
	if f.aggregateStatusFunc == nil {
		f.aggregateStatusFunc = make(map[schema.GroupVersionKind]AggregateStatusFunc)
	}
	f.aggregateStatusFunc[objGVK] = iFunc

	return f
}

func (f *FakeInterpreter) WithGetDependencies(objGVK schema.GroupVersionKind, iFunc GetDependenciesFunc) *FakeInterpreter {
	if f.getDependenciesFunc == nil {
		f.getDependenciesFunc = make(map[schema.GroupVersionKind]GetDependenciesFunc)
	}
	f.getDependenciesFunc[objGVK] = iFunc

	return f
}

func (f *FakeInterpreter) WithReflectStatus(objGVK schema.GroupVersionKind, iFunc ReflectStatusFunc) *FakeInterpreter {
	if f.reflectStatusFunc == nil {
		f.reflectStatusFunc = make(map[schema.GroupVersionKind]ReflectStatusFunc)
	}
	f.reflectStatusFunc[objGVK] = iFunc

	return f
}

func (f *FakeInterpreter) WithInterpretHealth(objGVK schema.GroupVersionKind, iFunc InterpretHealthFunc) *FakeInterpreter {
	if f.interpretHealthFunc == nil {
		f.interpretHealthFunc = make(map[schema.GroupVersionKind]InterpretHealthFunc)
	}
	f.interpretHealthFunc[objGVK] = iFunc

	return f
}
