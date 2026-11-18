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

package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
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
func NewStorage(scheme *runtime.Scheme, restConfig *restclient.Config, secretLister listcorev1.SecretLister, optsGetter generic.RESTOptionsGetter) (*ClusterStorage, error) {
	strategy := clusterregistry.NewStrategy(scheme)

	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &clusterapis.Cluster{} },
		NewListFunc:               func() runtime.Object { return &clusterapis.ClusterList{} },
		PredicateFunc:             clusterregistry.MatchCluster,
		DefaultQualifiedResource:  clusterapis.Resource("clusters"),
		SingularQualifiedResource: clusterapis.Resource("cluster"),
		CreateStrategy:            strategy,
		UpdateStrategy:            strategy,
		DeleteStrategy:            strategy,
		ResetFieldsStrategy:       strategy,

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

	kubeClientSet := kubernetes.NewForConfigOrDie(restConfig)
	karmadaLocation, karmadaTransport, err := karmadaResourceLocation(restConfig)
	if err != nil {
		return nil, err
	}
	clusterRest := &REST{secretLister, store}
	return &ClusterStorage{
		Cluster: clusterRest,
		Status:  &StatusREST{&statusStore},
		Proxy: &ProxyREST{
			restConfig:       restConfig,
			kubeClient:       kubeClientSet,
			secretLister:     clusterRest.secretLister,
			clusterGetter:    clusterRest.getCluster,
			clusterLister:    clusterRest.listClusters,
			karmadaLocation:  karmadaLocation,
			karmadaTransPort: karmadaTransport,
		},
	}, nil
}

// REST implements a RESTStorage for Cluster.
type REST struct {
	secretLister listcorev1.SecretLister
	*genericregistry.Store
}

const deletionProtectionErrorMessage = "This resource is protected, please make sure to remove the label: %s"

var _ rest.GracefulDeleter = &REST{}
var _ rest.Redirector = &REST{}

// Delete deletes a Cluster after verifying that it is not protected from deletion.
func (r *REST) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return r.Store.Delete(ctx, name, composeDeleteValidations(validateDeletionProtection, deleteValidation), options)
}

// validateDeletionProtection checks whether a Cluster is protected from deletion.
func validateDeletionProtection(_ context.Context, obj runtime.Object) error {
	cluster, ok := obj.(*clusterapis.Cluster)
	if !ok || cluster == nil {
		return fmt.Errorf("expected non-nil *cluster.Cluster, got %T", obj)
	}

	if cluster.ObjectMeta.Labels[workv1alpha2.DeletionProtectionLabelKey] != workv1alpha2.DeletionProtectionAlways {
		return nil
	}

	return apierrors.NewForbidden(
		schema.GroupResource{Group: clusterapis.GroupName, Resource: "clusters"},
		cluster.ObjectMeta.Name,
		//nolint:staticcheck // Preserve the established webhook error wording for a consistent user-facing response.
		fmt.Errorf(deletionProtectionErrorMessage, workv1alpha2.DeletionProtectionLabelKey),
	)
}

// composeDeleteValidations combines multiple validators into one validator.
func composeDeleteValidations(validations ...rest.ValidateObjectFunc) rest.ValidateObjectFunc {
	return func(ctx context.Context, obj runtime.Object) error {
		for _, validation := range validations {
			if validation == nil {
				continue
			}
			if err := validation(ctx, obj); err != nil {
				return err
			}
		}
		return nil
	}
}

// ResourceLocation returns a URL to which one can send traffic for the specified cluster.
func (r *REST) ResourceLocation(ctx context.Context, name string) (*url.URL, http.RoundTripper, error) {
	cluster, err := r.getCluster(ctx, name)
	if err != nil {
		return nil, nil, err
	}

	secretGetter := func(_ context.Context, namespace, name string) (*corev1.Secret, error) {
		return r.secretLister.Secrets(namespace).Get(name)
	}
	tlsConfig, err := proxy.GetTLSConfigForCluster(ctx, cluster, secretGetter)
	if err != nil {
		return nil, nil, err
	}

	return proxy.Location(cluster, tlsConfig)
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
func (r *REST) listClusters(ctx context.Context) (*clusterapis.ClusterList, error) {
	obj, err := r.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	clusterList := obj.(*clusterapis.ClusterList)
	if clusterList == nil {
		return nil, fmt.Errorf("unexpected object type: %#v", obj)
	}
	return clusterList, nil
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
func (r *StatusREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, _ bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	// We are explicitly setting forceAllowCreate to false in the call to the underlying storage because
	// subresources should never allow creating on update.
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
