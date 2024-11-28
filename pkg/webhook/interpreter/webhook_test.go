/*
Copyright 2024 The Karmada Authors.

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
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// WebhookMockHandler implements the Handler and DecoderInjector interfaces for testing.
type WebhookMockHandler struct {
	response Response
	decoder  *Decoder
}

// Handle implements the Handler interface for WebhookMockHandler.
func (m *WebhookMockHandler) Handle(_ context.Context, _ Request) Response {
	return m.response
}

// InjectDecoder implements the DecoderInjector interface by setting the decoder.
func (m *WebhookMockHandler) InjectDecoder(decoder *Decoder) {
	m.decoder = decoder
}

func TestNewWebhook(t *testing.T) {
	mockHandler := &WebhookMockHandler{}
	decoder := &Decoder{}

	webhook := NewWebhook(mockHandler, decoder)
	if webhook == nil {
		t.Fatalf("webhook returned by NewWebhook() is nil")
	}
	if webhook.handler != mockHandler {
		t.Errorf("webhook has incorrect handler: expected %v, got %v", mockHandler, webhook.handler)
	}
}

func TestHandle(t *testing.T) {
	var uid types.UID = "test-uid"

	expectedResponse := Response{
		ResourceInterpreterResponse: configv1alpha1.ResourceInterpreterResponse{
			Successful: true,
			Status: &configv1alpha1.RequestStatus{
				Code: http.StatusOK,
			},
			UID: uid,
		},
	}

	mockHandler := &WebhookMockHandler{response: expectedResponse}
	webhook := NewWebhook(mockHandler, &Decoder{})
	req := Request{
		ResourceInterpreterRequest: configv1alpha1.ResourceInterpreterRequest{
			UID: uid,
		},
	}

	resp := webhook.Handle(context.TODO(), req)
	if !reflect.DeepEqual(resp, expectedResponse) {
		t.Errorf("response mismatch in Handle(): expected %v, got %v", expectedResponse, resp)
	}
	if resp.UID != req.UID {
		t.Errorf("uid was not set as expected: expected %v, got %v", req.UID, resp.UID)
	}
}

func TestComplete(t *testing.T) {
	tests := []struct {
		name   string
		req    Request
		res    Response
		verify func(*Response, *Request) error
	}{
		{
			name: "TestComplete_StatusAndStatusCodeAreUnset_FieldsArePopulated",
			req: Request{
				ResourceInterpreterRequest: configv1alpha1.ResourceInterpreterRequest{
					UID: "test-uid",
				},
			},
			res:    Response{},
			verify: verifyResourceInterpreterCompleteResponse,
		},
		{
			name: "TestComplete_OverrideResponseUIDAndStatusCode_ResponseUIDAndStatusCodeAreOverridden",
			req: Request{
				ResourceInterpreterRequest: configv1alpha1.ResourceInterpreterRequest{
					UID: "test-uid",
				},
			},
			res: Response{
				ResourceInterpreterResponse: configv1alpha1.ResourceInterpreterResponse{
					UID: "existing-uid",
					Status: &configv1alpha1.RequestStatus{
						Code: http.StatusForbidden,
					},
				},
			},
			verify: func(resp *Response, req *Request) error {
				if resp.UID != req.UID {
					return fmt.Errorf("uid should be overridden if it's already set in the request: expected %v, got %v", req.UID, resp.UID)
				}
				if resp.Status.Code != http.StatusForbidden {
					return fmt.Errorf("status code should not be overridden if it's already set: expected %v, got %v", http.StatusForbidden, resp.Status.Code)
				}
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.res.Complete(test.req)
			if err := test.verify(&test.res, &test.req); err != nil {
				t.Errorf("failed to verify complete resource interpreter response: %v", err)
			}
		})
	}
}

// verifyResourceInterpreterCompleteResponse checks if the response from
// the resource interpreter's Complete method is valid.
// It ensures the response UID matches the request UID, the Status is initialized,
// and the Status code is set to http.StatusOK. Returns an error if any check fails.
func verifyResourceInterpreterCompleteResponse(res *Response, req *Request) error {
	if res.UID != req.UID {
		return fmt.Errorf("uid was not set as expected: expected %v, got %v", req.UID, res.UID)
	}
	if res.Status == nil {
		return fmt.Errorf("status should be initialized if it's nil")
	}
	if res.Status.Code != http.StatusOK {
		return fmt.Errorf("status code should be set to %v if it was 0, got %v", http.StatusOK, res.Status.Code)
	}
	return nil
}
