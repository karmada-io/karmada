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

package search

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

	searchapis "github.com/karmada-io/karmada/pkg/apis/search"
)

// NewStrategy creates and returns a ResourceRegistry Strategy instance.
func NewStrategy(typer runtime.ObjectTyper) Strategy {
	return Strategy{typer, names.SimpleNameGenerator}
}

// GetAttrs returns labels.Set, fields.Set, and error in case the given runtime.Object is not a ResourceRegistry.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	resourceRegistry, ok := obj.(*searchapis.ResourceRegistry)
	if !ok {
		return nil, nil, fmt.Errorf("given object is not a ResourceRegistry")
	}
	return resourceRegistry.ObjectMeta.Labels, SelectableFields(resourceRegistry), nil
}

// MatchResourceRegistry is the filter used by the generic etcd backend to watch events
// from etcd to clients of the apiserver only interested in specific labels/fields.
func MatchResourceRegistry(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// SelectableFields returns a field set that represents the object.
func SelectableFields(obj *searchapis.ResourceRegistry) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

// Strategy implements behavior for ResourceRegistry.
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
		"search.karmada.io/v1alpha1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
}

// PrepareForCreate is invoked on create before validation to normalize the object.
func (Strategy) PrepareForCreate(_ context.Context, _ runtime.Object) {
}

// PrepareForUpdate is invoked on update before validation to normalize the object.
func (Strategy) PrepareForUpdate(_ context.Context, _, _ runtime.Object) {
}

// Validate returns an ErrorList with validation errors or nil.
func (Strategy) Validate(_ context.Context, _ runtime.Object) field.ErrorList {
	// TODO: add validation for ResourceRegistry
	return field.ErrorList{}
}

// WarningsOnCreate returns warnings for the creation of the given object.
func (Strategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string { return nil }

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
func (Strategy) Canonicalize(_ runtime.Object) {
}

// ValidateUpdate is invoked after default fields in the object have been
// filled in before the object is persisted.
func (Strategy) ValidateUpdate(_ context.Context, _, _ runtime.Object) field.ErrorList {
	// TODO: add validation for ResourceRegistry
	return field.ErrorList{}
}

// WarningsOnUpdate returns warnings for the given update.
func (Strategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

// StatusStrategy implements behavior for ResourceRegistryStatus.
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
func (StatusStrategy) PrepareForUpdate(_ context.Context, _, _ runtime.Object) {
}

// ValidateUpdate is the default update validation for an end user updating status
func (StatusStrategy) ValidateUpdate(_ context.Context, _, _ runtime.Object) field.ErrorList {
	return field.ErrorList{}
}

// WarningsOnUpdate returns warnings for the given update.
func (StatusStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}
