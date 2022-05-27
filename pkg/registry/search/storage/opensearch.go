package storage

import (
	"net/http"

	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

func (r *SearchREST) newOpenSearchHandler(info *genericrequest.RequestInfo, responder rest.Responder) (http.Handler, error) {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Construct a handler and send the request to the ES.
	}), nil
}
