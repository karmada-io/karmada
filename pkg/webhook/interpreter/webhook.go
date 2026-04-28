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

package interpreter

import (
	"context"
	"net/http"

	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// Request defines the input for an interpreter handler.
// It contains information to identify the object in
// question (kind, name, namespace), as well as the
// operation in request(e.g. InterpreterOperationInterpretReplica, InterpreterOperationPrune,
// etc), and the object itself.
type Request struct {
	configv1alpha1.ResourceInterpreterRequest
}

// Response is the output of an interpreter handler.
type Response struct {
	configv1alpha1.ResourceInterpreterResponse
}

// Complete populates any fields that are yet to be set in
// the underlying ResourceInterpreterResponse, It mutates the response.
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

// Handler can handle an ResourceInterpreterRequest.
type Handler interface {
	// Handle yields a response to an ResourceInterpreterRequest.
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

// Handle processes ResourceInterpreterRequest.
func (wh *Webhook) Handle(ctx context.Context, req Request) Response {
	resp := wh.handler.Handle(ctx, req)
	resp.Complete(req)
	return resp
}
