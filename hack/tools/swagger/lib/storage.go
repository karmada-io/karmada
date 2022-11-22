package lib

import (
	"context"
	"net/http"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/rest"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

// StandardREST define CRUD api for resources.
type StandardREST struct {
	cfg ResourceInfo
}

// StatusREST define status endpoint for resources.
type StatusREST struct {
	cfg StatusInfo
}

// ProxyREST define proxy endpoint for resources.
type ProxyREST struct{}

// Implement below interfaces for StandardREST.
var _ rest.GroupVersionKindProvider = &StandardREST{}
var _ rest.Scoper = &StandardREST{}
var _ rest.StandardStorage = &StandardREST{}

// Implement below interfaces for StatusREST.
var _ rest.Patcher = &StatusREST{}

// Implement below interfaces for ProxyREST.
var _ rest.Connecter = &ProxyREST{}

// GroupVersionKind implement GroupVersionKind interface.
func (r *StandardREST) GroupVersionKind(containingGV schema.GroupVersion) schema.GroupVersionKind {
	return r.cfg.gvk
}

// NamespaceScoped implement NamespaceScoped interface.
func (r *StandardREST) NamespaceScoped() bool {
	return r.cfg.namespaceScoped
}

// New implement New interface.
func (r *StandardREST) New() runtime.Object {
	return r.cfg.obj
}

// Create implement Create interface.
func (r *StandardREST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	return r.New(), nil
}

// Get implement Get interface.
func (r *StandardREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.New(), nil
}

// NewList implement NewList interface.
func (r *StandardREST) NewList() runtime.Object {
	return r.cfg.list
}

// List implement List interface.
func (r *StandardREST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	return r.NewList(), nil
}

// ConvertToTable implement ConvertToTable interface.
func (r *StandardREST) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return nil, nil
}

// Update implement Update interface.
func (r *StandardREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return r.New(), true, nil
}

// Delete implement Delete interface.
func (r *StandardREST) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return r.New(), true, nil
}

// DeleteCollection implement DeleteCollection interface.
func (r *StandardREST) DeleteCollection(ctx context.Context, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions, listOptions *metainternalversion.ListOptions) (runtime.Object, error) {
	return r.NewList(), nil
}

// Watch implement Watch interface.
func (r *StandardREST) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return nil, nil
}

// Destroy cleans up its resources on shutdown.
func (r *StandardREST) Destroy() {
	// Given no underlying store, so we don't
	// need to destroy anything.
}

// GroupVersionKind implement GroupVersionKind interface.
func (r *StatusREST) GroupVersionKind(containingGV schema.GroupVersion) schema.GroupVersionKind {
	return r.cfg.gvk
}

// New returns Cluster object.
func (r *StatusREST) New() runtime.Object {
	return r.cfg.obj
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return r.New(), true, nil
}

// Get retrieves the status object.
func (r *StatusREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.New(), nil
}

// Destroy cleans up its resources on shutdown.
func (r *StatusREST) Destroy() {
	// Given no underlying store, so we don't
	// need to destroy anything.
}

// New returns an empty cluster proxy subresource.
func (r *ProxyREST) New() runtime.Object {
	return &clusterv1alpha1.ClusterProxyOptions{}
}

// ConnectMethods returns the list of HTTP methods handled by Connect.
func (r *ProxyREST) ConnectMethods() []string {
	return []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
}

// NewConnectOptions returns versioned resource that represents proxy parameters.
func (r *ProxyREST) NewConnectOptions() (runtime.Object, bool, string) {
	return &clusterv1alpha1.ClusterProxyOptions{}, true, "path"
}

// Connect implement Connect interface.
func (r *ProxyREST) Connect(ctx context.Context, id string, options runtime.Object, responder rest.Responder) (http.Handler, error) {
	return nil, nil
}

// Destroy cleans up its resources on shutdown.
func (r *ProxyREST) Destroy() {
	// Given no underlying store, so we don't
	// need to destroy anything.
}

// ResourceInfo is content of StandardREST.
type ResourceInfo struct {
	gvk             schema.GroupVersionKind
	obj             runtime.Object
	list            runtime.Object
	namespaceScoped bool
}

// StatusInfo is content of StatusREST.
type StatusInfo struct {
	gvk schema.GroupVersionKind
	obj runtime.Object
}
