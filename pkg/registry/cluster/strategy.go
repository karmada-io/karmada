package cluster

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
)

// NewStrategy creates and returns a ClusterStrategy instance.
func NewStrategy(typer runtime.ObjectTyper) Strategy {
	return Strategy{typer, names.SimpleNameGenerator}
}

// GetAttrs returns labels.Set, fields.Set, and error in case the given runtime.Object is not a Cluster.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	cluster, ok := obj.(*clusterapis.Cluster)
	if !ok {
		return nil, nil, fmt.Errorf("given object is not a Cluster")
	}
	return cluster.ObjectMeta.Labels, SelectableFields(cluster), nil
}

// MatchCluster is the filter used by the generic etcd backend to watch events
// from etcd to clients of the apiserver only interested in specific labels/fields.
func MatchCluster(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// SelectableFields returns a field set that represents the object.
func SelectableFields(obj *clusterapis.Cluster) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, true)
}

// Strategy implements behavior for Cluster.
type Strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NamespaceScoped returns if the object must be in a namespace.
func (Strategy) NamespaceScoped() bool {
	return false
}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user.
func (Strategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"cluster.karmada.io/v1alpha1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
}

// PrepareForCreate is invoked on create before validation to normalize the object.
func (Strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
}

// PrepareForUpdate is invoked on update before validation to normalize the object.
func (Strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
}

// Validate returns an ErrorList with validation errors or nil.
func (Strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	// TODO: add validation for Cluster
	return field.ErrorList{}
}

// WarningsOnCreate returns warnings for the creation of the given object.
func (Strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string { return nil }

// AllowCreateOnUpdate returns true if the object can be created by a PUT.
func (Strategy) AllowCreateOnUpdate() bool {
	return false
}

// AllowUnconditionalUpdate returns true if the object can be updated
// unconditionally (irrespective of the latest resource version), when
// there is no resource version specified in the object.
func (Strategy) AllowUnconditionalUpdate() bool {
	return true
}

// Canonicalize allows an object to be mutated into a canonical form.
func (Strategy) Canonicalize(obj runtime.Object) {
}

// ValidateUpdate is invoked after default fields in the object have been
// filled in before the object is persisted.
func (Strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return field.ErrorList{}
}

// WarningsOnUpdate returns warnings for the given update.
func (Strategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

// StatusStrategy implements behavior for ClusterStatus.
type StatusStrategy struct {
	Strategy
}

// NewStatusStrategy creates and returns a StatusStrategy instance.
func NewStatusStrategy(strategy Strategy) StatusStrategy {
	return StatusStrategy{strategy}
}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user.
func (StatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update of status
func (StatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
}

// ValidateUpdate is the default update validation for an end user updating status
func (StatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return field.ErrorList{}
}

// WarningsOnUpdate returns warnings for the given update.
func (StatusStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}
