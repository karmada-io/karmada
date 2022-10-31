package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	"github.com/karmada-io/karmada/pkg/printers"
	printersinternal "github.com/karmada-io/karmada/pkg/printers/internalversion"
	printerstorage "github.com/karmada-io/karmada/pkg/printers/storage"
	clusterregistry "github.com/karmada-io/karmada/pkg/registry/cluster"
	"github.com/karmada-io/karmada/pkg/util/proxy"
)

// ClusterStorage includes storage for Cluster and for all the subresources.
type ClusterStorage struct {
	Cluster *REST
	Status  *StatusREST
	Proxy   *ProxyREST
}

// NewStorage returns a ClusterStorage object that will work against clusters.
func NewStorage(scheme *runtime.Scheme, kubeClient kubernetes.Interface, optsGetter generic.RESTOptionsGetter) (*ClusterStorage, error) {
	strategy := clusterregistry.NewStrategy(scheme)

	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &clusterapis.Cluster{} },
		NewListFunc:              func() runtime.Object { return &clusterapis.ClusterList{} },
		PredicateFunc:            clusterregistry.MatchCluster,
		DefaultQualifiedResource: clusterapis.Resource("clusters"),

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,

		TableConvertor: printerstorage.TableConvertor{TableGenerator: printers.NewTableGenerator().With(printersinternal.AddHandlers)},
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: clusterregistry.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	statusStrategy := clusterregistry.NewStatusStrategy(strategy)
	statusStore := *store
	statusStore.UpdateStrategy = statusStrategy
	statusStore.ResetFieldsStrategy = statusStrategy

	clusterRest := &REST{store}
	return &ClusterStorage{
		Cluster: clusterRest,
		Status:  &StatusREST{&statusStore},
		Proxy: &ProxyREST{
			kubeClient:    kubeClient,
			clusterGetter: clusterRest.getCluster,
		},
	}, nil
}

// REST implements a RESTStorage for Cluster.
type REST struct {
	*genericregistry.Store
}

// Implement Redirector.
var _ = rest.Redirector(&REST{})

// ResourceLocation returns a URL to which one can send traffic for the specified cluster.
func (r *REST) ResourceLocation(ctx context.Context, name string) (*url.URL, http.RoundTripper, error) {
	cluster, err := r.getCluster(ctx, name)
	if err != nil {
		return nil, nil, err
	}

	return proxy.Location(cluster)
}

func (r *REST) getCluster(ctx context.Context, name string) (*clusterapis.Cluster, error) {
	obj, err := r.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	cluster := obj.(*clusterapis.Cluster)
	if cluster == nil {
		return nil, fmt.Errorf("unexpected object type: %#v", obj)
	}
	return cluster, nil
}

// ResourceGetter is an interface for retrieving resources by ResourceLocation.
type ResourceGetter interface {
	Get(context.Context, string, *metav1.GetOptions) (runtime.Object, error)
}

// StatusREST implements the REST endpoint for changing the status of a cluster.
type StatusREST struct {
	store *genericregistry.Store
}

// New returns empty Cluster object.
func (r *StatusREST) New() runtime.Object {
	return &clusterapis.Cluster{}
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	// We are explicitly setting forceAllowCreate to false in the call to the underlying storage because
	// subresources should never allow creating on update.
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, false, options)
}

// GetResetFields implements rest.ResetFieldsStrategy
func (r *StatusREST) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return r.store.GetResetFields()
}
