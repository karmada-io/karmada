package explorer

import (
	"context"
	"net/http"

	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// Request defines the input for an explorer handler.
// It contains information to identify the object in
// question (kind, name, namespace), as well as the
// operation in request(e.g. ExploreReplica, ExplorePacking,
// etc), and the object itself.
type Request struct {
	configv1alpha1.ExploreRequest
}

// Response is the output of an explorer handler.
type Response struct {
	configv1alpha1.ExploreResponse
}

// Complete populates any fields that are yet to be set in
// the underlying ExploreResponse, It mutates the response.
func (r *Response) Complete(req Request) {
	r.UID = req.UID

	// ensure that we have a valid status code
	if r.Status == nil {
		r.Status = &configv1alpha1.RequestStatus{}
	}
	if r.Status.Code == 0 {
		r.Status.Code = http.StatusOK
	}
}

// Handler can handle an ExploreRequest.
type Handler interface {
	// Handle yields a response to an ExploreRequest.
	//
	// The supplied context is extracted from the received http.Request, allowing wrapping
	// http.Handlers to inject values into and control cancellation of downstream request processing.
	Handle(context.Context, Request) Response
}

// Webhook represents each individual webhook.
type Webhook struct {
	// handler actually processes an admission request returning whether it was allowed or denied,
	// and potentially patches to apply to the handler.
	handler Handler
}

// NewWebhook return a Webhook
func NewWebhook(handler Handler, decoder *Decoder) *Webhook {
	if !InjectDecoderInto(decoder, handler) {
		klog.Errorf("Inject decoder into handler err")
	}
	return &Webhook{handler: handler}
}

// Handle processes ExploreRequest.
func (wh *Webhook) Handle(ctx context.Context, req Request) Response {
	resp := wh.handler.Handle(ctx, req)
	resp.Complete(req)
	return resp
}
