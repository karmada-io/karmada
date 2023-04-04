package resourceinterpreter

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/webhook"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/webhook/request"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/native"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/thirdparty"
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
	informer genericmanager.SingleClusterInformerManager

	configurableInterpreter *declarative.ConfigurableInterpreter
	customizedInterpreter   *webhook.CustomizedInterpreter
	thirdpartyInterpreter   *thirdparty.ConfigurableInterpreter
	defaultInterpreter      *native.DefaultInterpreter
}

// Start starts running the component and will never stop running until the context is closed or an error occurs.
func (i *customResourceInterpreterImpl) Start(ctx context.Context) (err error) {
	klog.Infof("Starting custom resource interpreter.")

	i.customizedInterpreter, err = webhook.NewCustomizedInterpreter(i.informer)
	if err != nil {
		return
	}
	i.configurableInterpreter = declarative.NewConfigurableInterpreter(i.informer)

	i.thirdpartyInterpreter = thirdparty.NewConfigurableInterpreter()
	i.defaultInterpreter = native.NewDefaultInterpreter()

	i.informer.Start()
	<-ctx.Done()
	klog.Infof("Stopped as stopCh closed.")
	return nil
}

// HookEnabled tells if any hook exist for specific resource type and operation.
func (i *customResourceInterpreterImpl) HookEnabled(objGVK schema.GroupVersionKind, operation configv1alpha1.InterpreterOperation) bool {
	return i.defaultInterpreter.HookEnabled(objGVK, operation) ||
		i.thirdpartyInterpreter.HookEnabled(objGVK, operation) ||
		i.configurableInterpreter.HookEnabled(objGVK, operation) ||
		i.customizedInterpreter.HookEnabled(objGVK, operation)
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (i *customResourceInterpreterImpl) GetReplicas(object *unstructured.Unstructured) (replica int32, requires *workv1alpha2.ReplicaRequirements, err error) {
	var hookEnabled bool

	replica, requires, hookEnabled, err = i.configurableInterpreter.GetReplicas(object)
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}

	replica, requires, hookEnabled, err = i.customizedInterpreter.GetReplicas(context.TODO(), &request.Attributes{
		Operation: configv1alpha1.InterpreterOperationInterpretReplica,
		Object:    object,
	})
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}
	replica, requires, hookEnabled, err = i.thirdpartyInterpreter.GetReplicas(object)
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}

	replica, requires, err = i.defaultInterpreter.GetReplicas(object)
	return
}

// ReviseReplica revises the replica of the given object.
func (i *customResourceInterpreterImpl) ReviseReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error) {
	obj, hookEnabled, err := i.configurableInterpreter.ReviseReplica(object, replica)
	if err != nil {
		return nil, err
	}
	if hookEnabled {
		return obj, nil
	}

	klog.V(4).Infof("Revise replicas for object: %v %s/%s with webhook interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())
	obj, hookEnabled, err = i.customizedInterpreter.Patch(context.TODO(), &request.Attributes{
		Operation:   configv1alpha1.InterpreterOperationReviseReplica,
		Object:      object,
		ReplicasSet: int32(replica),
	})
	if err != nil {
		return nil, err
	}
	if hookEnabled {
		return obj, nil
	}
	obj, hookEnabled, err = i.thirdpartyInterpreter.ReviseReplica(object, replica)
	if err != nil {
		return nil, err
	}
	if hookEnabled {
		return obj, nil
	}

	return i.defaultInterpreter.ReviseReplica(object, replica)
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (i *customResourceInterpreterImpl) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	obj, hookEnabled, err := i.configurableInterpreter.Retain(desired, observed)
	if err != nil {
		return nil, err
	}
	if hookEnabled {
		return obj, nil
	}

	klog.V(4).Infof("Retain object: %v %s/%s with webhook interpreter.", desired.GroupVersionKind(), desired.GetNamespace(), desired.GetName())
	obj, hookEnabled, err = i.customizedInterpreter.Patch(context.TODO(), &request.Attributes{
		Operation:   configv1alpha1.InterpreterOperationRetain,
		Object:      desired,
		ObservedObj: observed,
	})
	if err != nil {
		return nil, err
	}
	if hookEnabled {
		return obj, nil
	}
	obj, hookEnabled, err = i.thirdpartyInterpreter.Retain(desired, observed)
	if err != nil {
		return nil, err
	}
	if hookEnabled {
		return obj, nil
	}

	return i.defaultInterpreter.Retain(desired, observed)
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (i *customResourceInterpreterImpl) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	obj, hookEnabled, err := i.configurableInterpreter.AggregateStatus(object, aggregatedStatusItems)
	if err != nil {
		return nil, err
	}
	if hookEnabled {
		return obj, nil
	}

	klog.V(4).Infof("Aggregate status of object: %v %s/%s with webhook interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())
	obj, hookEnabled, err = i.customizedInterpreter.Patch(context.TODO(), &request.Attributes{
		Operation:        configv1alpha1.InterpreterOperationAggregateStatus,
		Object:           object.DeepCopy(),
		AggregatedStatus: aggregatedStatusItems,
	})
	if err != nil {
		return nil, err
	}
	if hookEnabled {
		return obj, nil
	}
	obj, hookEnabled, err = i.thirdpartyInterpreter.AggregateStatus(object, aggregatedStatusItems)
	if err != nil {
		return nil, err
	}
	if hookEnabled {
		return obj, nil
	}

	return i.defaultInterpreter.AggregateStatus(object, aggregatedStatusItems)
}

// GetDependencies returns the dependent resources of the given object.
func (i *customResourceInterpreterImpl) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, err error) {
	dependencies, hookEnabled, err := i.configurableInterpreter.GetDependencies(object)
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}

	dependencies, hookEnabled, err = i.customizedInterpreter.GetDependencies(context.TODO(), &request.Attributes{
		Operation: configv1alpha1.InterpreterOperationInterpretDependency,
		Object:    object,
	})
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}
	dependencies, hookEnabled, err = i.thirdpartyInterpreter.GetDependencies(object)
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}

	dependencies, err = i.defaultInterpreter.GetDependencies(object)
	return
}

// ReflectStatus returns the status of the object.
func (i *customResourceInterpreterImpl) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, err error) {
	status, hookEnabled, err := i.configurableInterpreter.ReflectStatus(object)
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}
	status, hookEnabled, err = i.customizedInterpreter.ReflectStatus(context.TODO(), &request.Attributes{
		Operation: configv1alpha1.InterpreterOperationInterpretStatus,
		Object:    object,
	})
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}
	status, hookEnabled, err = i.thirdpartyInterpreter.ReflectStatus(object)
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}
	status, err = i.defaultInterpreter.ReflectStatus(object)
	return
}

// InterpretHealth returns the health state of the object.
func (i *customResourceInterpreterImpl) InterpretHealth(object *unstructured.Unstructured) (healthy bool, err error) {
	healthy, hookEnabled, err := i.configurableInterpreter.InterpretHealth(object)
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}

	healthy, hookEnabled, err = i.customizedInterpreter.InterpretHealth(context.TODO(), &request.Attributes{
		Operation: configv1alpha1.InterpreterOperationInterpretHealth,
		Object:    object,
	})
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}
	healthy, hookEnabled, err = i.thirdpartyInterpreter.InterpretHealth(object)
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}

	healthy, err = i.defaultInterpreter.InterpretHealth(object)
	return
}
