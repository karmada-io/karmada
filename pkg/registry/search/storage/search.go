package storage

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	searchapis "github.com/karmada-io/karmada/pkg/apis/search"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

// SearchREST implements a RESTStorage for search resource.
type SearchREST struct {
	multiClusterInformerManager genericmanager.MultiClusterInformerManager
	clusterLister               clusterlister.ClusterLister

	// add needed parameters here
}

var _ rest.Scoper = &SearchREST{}
var _ rest.Storage = &SearchREST{}
var _ rest.Connecter = &SearchREST{}

// NewSearchREST returns a RESTStorage object that will work against search.
func NewSearchREST(
	multiClusterInformerManager genericmanager.MultiClusterInformerManager,
	clusterLister clusterlister.ClusterLister) *SearchREST {
	return &SearchREST{
		multiClusterInformerManager: multiClusterInformerManager,
		clusterLister:               clusterLister,
	}
}

// New return empty Search object.
func (r *SearchREST) New() runtime.Object {
	return &searchapis.Search{}
}

// NamespaceScoped returns false because Search is not namespaced.
func (r *SearchREST) NamespaceScoped() bool {
	return false
}

// ConnectMethods returns the list of HTTP methods handled by Connect.
func (r *SearchREST) ConnectMethods() []string {
	return []string{"GET"}
}

// NewConnectOptions returns an empty options object that will be used to pass options to the Connect method.
func (r *SearchREST) NewConnectOptions() (runtime.Object, bool, string) {
	return nil, true, ""
}

// Connect returns a handler for search.
func (r *SearchREST) Connect(ctx context.Context, id string, _ runtime.Object, responder rest.Responder) (http.Handler, error) {
	info, ok := genericrequest.RequestInfoFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("no RequestInfo found in the context")
	}

	if len(info.Parts) < 3 {
		return nil, fmt.Errorf("invalid requestInfo parts: %v", info.Parts)
	}

	// reqParts are slices split by the k8s-native URL that include
	// APIPrefix, APIGroup, APIVersion, Namespace and Resource.
	// For example, the whole request URL is /apis/search.karmada.io/v1alpha1/search/cache/api/v1/nodes
	// info.Parts is [search cache api v1 nodes], so reParts is [api v1 nodes]
	reqParts := info.Parts[2:]
	nativeResourceInfo, err := parseK8sNativeResourceInfo(reqParts)
	if err != nil {
		klog.Errorf("Failed to parse k8s native RequestInfo, err: %v", err)
		return nil, err
	}
	if !nativeResourceInfo.IsResourceRequest {
		return nil, fmt.Errorf("k8s native Request URL(%s) is not a resource request", strings.Join(reqParts, "/"))
	}

	switch id {
	case "cache":
		return r.newCacheHandler(nativeResourceInfo, responder)
	case "opensearch":
		return r.newOpenSearchHandler(nativeResourceInfo, responder)
	default:
		return nil, fmt.Errorf("connect with unrecognized search category %s", id)
	}
}

// Destroy cleans up its resources on shutdown.
func (r *SearchREST) Destroy() {
	// Given no underlying store, so we don't
	// need to destroy anything.
}
