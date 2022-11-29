package executor

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// Interface execute rule with given arguments. If not enabled, return enabled with false.
type Interface interface {
	// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
	GetReplicas(r *configv1alpha1.ReplicaResourceRequirement, obj *unstructured.Unstructured) (replica int32, requires *workv1alpha2.ReplicaRequirements, enabled bool, err error)

	// ReviseReplica revises the replica of the given object.
	ReviseReplica(r *configv1alpha1.ReplicaRevision, object *unstructured.Unstructured, replica int64) (revised *unstructured.Unstructured, enabled bool, err error)

	// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
	Retain(r *configv1alpha1.LocalValueRetention, desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, enabled bool, err error)

	// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
	AggregateStatus(r *configv1alpha1.StatusAggregation, obj *unstructured.Unstructured, items []workv1alpha2.AggregatedStatusItem) (aggregated *unstructured.Unstructured, enabled bool, err error)

	// InterpretHealth returns the health state of the object.
	InterpretHealth(r *configv1alpha1.HealthInterpretation, obj *unstructured.Unstructured) (healthy bool, enabled bool, err error)

	// ReflectStatus returns the status of the object.
	ReflectStatus(r *configv1alpha1.StatusReflection, obj *unstructured.Unstructured) (status *runtime.RawExtension, enabled bool, err error)

	// GetDependencies returns the dependent resources of the given object.
	GetDependencies(r *configv1alpha1.DependencyInterpretation, obj *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, enabled bool, err error)
}
