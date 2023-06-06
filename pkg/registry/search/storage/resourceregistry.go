package storage

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"

	searchapis "github.com/karmada-io/karmada/pkg/apis/search"
	searchregistry "github.com/karmada-io/karmada/pkg/registry/search"
)

// ResourceRegistryStorage includes storage for ResourceRegistry and for all the subresources.
type ResourceRegistryStorage struct {
	ResourceRegistry *REST
	Status           *StatusREST
}

// NewResourceRegistryStorage returns a ResourceRegistryStorage object that will work against resourceRegistries.
func NewResourceRegistryStorage(scheme *runtime.Scheme, optsGetter generic.RESTOptionsGetter) (*ResourceRegistryStorage, error) {
	strategy := searchregistry.NewStrategy(scheme)

	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &searchapis.ResourceRegistry{} },
		NewListFunc:              func() runtime.Object { return &searchapis.ResourceRegistryList{} },
		PredicateFunc:            searchregistry.MatchResourceRegistry,
		DefaultQualifiedResource: searchapis.Resource("resourceRegistries"),

		CreateStrategy:      strategy,
		UpdateStrategy:      strategy,
		DeleteStrategy:      strategy,
		ResetFieldsStrategy: strategy,

		// TODO: define table converter that exposes more than name/creation timestamp
		TableConvertor: rest.NewDefaultTableConvertor(searchapis.Resource("resourceRegistries")),
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: searchregistry.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	statusStrategy := searchregistry.NewStatusStrategy(strategy)
	statusStore := *store
	statusStore.UpdateStrategy = statusStrategy
	statusStore.ResetFieldsStrategy = statusStrategy

	resourceRegistryRest := &REST{store}
	return &ResourceRegistryStorage{
		ResourceRegistry: resourceRegistryRest,
		Status:           &StatusREST{&statusStore},
	}, nil
}

// REST implements a RESTStorage for ResourceRegistry.
type REST struct {
	*genericregistry.Store
}

// StatusREST implements the REST endpoint for changing the status of a ResourceRegistry.
type StatusREST struct {
	store *genericregistry.Store
}

// New returns empty ResourceRegistry.
func (r *StatusREST) New() runtime.Object {
	return &searchapis.ResourceRegistry{}
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, _ bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	// We are explicitly setting forceAllowCreate to false in the call to the underlying storage because
	// subresources should never allow create on update.
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, false, options)
}

// GetResetFields implements rest.ResetFieldsStrategy
func (r *StatusREST) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return r.store.GetResetFields()
}

// Destroy cleans up its resources on shutdown.
func (r *StatusREST) Destroy() {
	// Given that underlying 'store' is shared with REST,
	// we don't have anything else need to be destroyed.
}
