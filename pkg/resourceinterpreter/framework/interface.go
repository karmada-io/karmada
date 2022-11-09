package framework

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// Interpreter describe the interpretation rules
type Interpreter interface {
	// HookEnabled tells if any hook exist for specific resource type and operation.
	HookEnabled(objGVK schema.GroupVersionKind, operationType configv1alpha1.InterpreterOperation) bool

	// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
	GetReplicas(object *unstructured.Unstructured) (replica int32, replicaRequires *workv1alpha2.ReplicaRequirements, enabled bool, err error)

	// ReviseReplica revises the replica of the given object.
	ReviseReplica(object *unstructured.Unstructured, replica int64) (obj *unstructured.Unstructured, enabled bool, err error)

	// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
	Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, enabled bool, err error)

	// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
	AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (obj *unstructured.Unstructured, enabled bool, err error)

	// GetDependencies returns the dependent resources of the given object.
	GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, enabled bool, err error)

	// ReflectStatus returns the status of the object.
	ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, enabled bool, err error)

	// InterpretHealth returns the health state of the object.
	InterpretHealth(object *unstructured.Unstructured) (healthy bool, enabled bool, err error)

	// other common method
}
