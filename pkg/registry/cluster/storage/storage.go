package storage

import (
	"context"
	"crypto/tls"
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
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	clusterregistry "github.com/karmada-io/karmada/pkg/registry/cluster"
)

// ClusterStorage includes storage for Cluster and for Status subresource.
type ClusterStorage struct {
	Cluster *REST
	Status  *StatusREST
	Proxy   *ProxyREST
}

// NewStorage returns new instance of ClusterStorage.
func NewStorage(scheme *runtime.Scheme, kubeClient kubernetes.Interface, karmadaClient karmadaclientset.Interface,
	optsGetter generic.RESTOptionsGetter) (ClusterStorage, error) {
	clusterRest, clusterStatusRest, err := NewREST(scheme, optsGetter)
	if err != nil {
		return ClusterStorage{}, err
	}

	return ClusterStorage{
		Cluster: clusterRest,
		Status:  clusterStatusRest,
		Proxy: &ProxyREST{
			Redirector:    clusterRest,
			kubeClient:    kubeClient,
			karmadaClient: karmadaClient,
		},
	}, nil
}

// NewREST returns a RESTStorage object that will work against API services.
func NewREST(scheme *runtime.Scheme, optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST, error) {
	strategy := clusterregistry.NewStrategy(scheme)

	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &clusterapis.Cluster{} },
		NewListFunc:              func() runtime.Object { return &clusterapis.ClusterList{} },
		PredicateFunc:            clusterregistry.MatchCluster,
		DefaultQualifiedResource: clusterapis.Resource("clusters"),

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,

		// TODO: define table converter that exposes more than name/creation timestamp
		TableConvertor: rest.NewDefaultTableConvertor(clusterapis.Resource("clusters")),
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: clusterregistry.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, err
	}

	statusStrategy := clusterregistry.NewStatusStrategy(strategy)
	statusStore := *store
	statusStore.UpdateStrategy = statusStrategy
	statusStore.ResetFieldsStrategy = statusStrategy

	return &REST{store}, &StatusREST{store: &statusStore}, nil
}

// REST implements a RESTStorage for Cluster.
type REST struct {
	*genericregistry.Store
}

// Implement Redirector.
var _ = rest.Redirector(&REST{})

// ResourceLocation returns a URL to which one can send traffic for the specified cluster.
func (r *REST) ResourceLocation(ctx context.Context, name string) (*url.URL, http.RoundTripper, error) {
	cluster, err := getCluster(ctx, r, name)
	if err != nil {
		return nil, nil, err
	}

	location, err := constructLocation(cluster)
	if err != nil {
		return nil, nil, err
	}

	transport, err := createProxyTransport(cluster)
	if err != nil {
		return nil, nil, err
	}

	return location, transport, nil
}

// ResourceGetter is an interface for retrieving resources by ResourceLocation.
type ResourceGetter interface {
	Get(context.Context, string, *metav1.GetOptions) (runtime.Object, error)
}

func getCluster(ctx context.Context, getter ResourceGetter, name string) (*clusterapis.Cluster, error) {
	obj, err := getter.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	cluster := obj.(*clusterapis.Cluster)
	if cluster == nil {
		return nil, fmt.Errorf("unexpected object type: %#v", obj)
	}
	return cluster, nil
}

func constructLocation(cluster *clusterapis.Cluster) (*url.URL, error) {
	if cluster.Spec.APIEndpoint == "" {
		return nil, fmt.Errorf("API endpoint of cluster %s should not be empty", cluster.Name)
	}

	uri, err := url.Parse(cluster.Spec.APIEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse api endpoint %s: %v", cluster.Spec.APIEndpoint, err)
	}
	return uri, nil
}

func createProxyTransport(cluster *clusterapis.Cluster) (*http.Transport, error) {
	trans := &http.Transport{}
	// #nosec
	trans.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	if cluster.Spec.ProxyURL != "" {
		proxy, err := url.Parse(cluster.Spec.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse url of proxy url %s: %v", cluster.Spec.ProxyURL, err)
		}
		trans.Proxy = http.ProxyURL(proxy)
	}
	return trans, nil
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
	// subresources should never allow create on update.
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, false, options)
}

// GetResetFields implements rest.ResetFieldsStrategy
func (r *StatusREST) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return r.store.GetResetFields()
}
