package storage

import (
	"context"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	searchapis "github.com/karmada-io/karmada/pkg/apis/search"
)

// SearchREST implements a RESTStorage for search resource.
type SearchREST struct {
	kubeClient kubernetes.Interface

	// add needed parameters here
}

var _ rest.Scoper = &SearchREST{}
var _ rest.Storage = &SearchREST{}
var _ rest.Connecter = &SearchREST{}

// NewSearchREST returns a RESTStorage object that will work against search.
func NewSearchREST(client kubernetes.Interface) *SearchREST {
	return &SearchREST{
		kubeClient: client,
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

// Connect returns a handler for the ES search.
func (r *SearchREST) Connect(ctx context.Context, id string, _ runtime.Object, responder rest.Responder) (http.Handler, error) {
	klog.Infof("Prepare for construct handler to connect ES.")

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

		// Construct a handler and send the request to the ES.
	}), nil
}
