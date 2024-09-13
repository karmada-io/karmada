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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/json"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// HTTPMockHandler implements the Handler and DecoderInjector interfaces for testing.
type HTTPMockHandler struct {
	response Response
	decoder  *Decoder
}

// Handle implements the Handler interface for HTTPMockHandler.
func (m *HTTPMockHandler) Handle(_ context.Context, _ Request) Response {
	return m.response
}

// InjectDecoder implements the DecoderInjector interface by setting the decoder.
func (m *HTTPMockHandler) InjectDecoder(decoder *Decoder) {
	m.decoder = decoder
}

// mockBody simulates an error when reading the request body.
type mockBody struct{}

func (m *mockBody) Read(_ []byte) (n int, err error) {
	return 0, errors.New("mock read error")
}

func (m *mockBody) Close() error {
	return nil
}

// limitedBadResponseWriter is a custom io.Writer implementation that simulates
// write errors for a specified number of attempts. After a certain number of failures,
// it allows the write operation to succeed.
type limitedBadResponseWriter struct {
	failCount   int
	maxFailures int
}

// Write simulates writing data to the writer. It forces an error response for
// a limited number of attempts, specified by maxFailures. Once failCount reaches
// maxFailures, it allows the write to succeed.
func (b *limitedBadResponseWriter) Write(p []byte) (n int, err error) {
	if b.failCount < b.maxFailures {
		b.failCount++
		return 0, errors.New("forced write error")
	}
	// After reaching maxFailures, allow the write to succeed to stop the infinite loop.
	return len(p), nil
}

func TestServeHTTP(t *testing.T) {
	tests := []struct {
		name        string
		req         *http.Request
		mockHandler *HTTPMockHandler
		contentType string
		res         configv1alpha1.ResourceInterpreterContext
		prep        func(*http.Request, string) error
		want        *configv1alpha1.ResourceInterpreterResponse
	}{
		{
			name:        "ServeHTTP_EmptyBody_RequestFailed",
			req:         httptest.NewRequest(http.MethodPost, "/", nil),
			mockHandler: &HTTPMockHandler{},
			contentType: "application/json",
			prep: func(req *http.Request, contentType string) error {
				req.Header.Set("Content-Type", contentType)
				req.Body = nil
				return nil
			},
			want: &configv1alpha1.ResourceInterpreterResponse{
				UID:        "",
				Successful: false,
				Status: &configv1alpha1.RequestStatus{
					Message: "request body is empty",
					Code:    http.StatusBadRequest,
				},
			},
		},
		{
			name:        "ServeHTTP_InvalidContentType_ContentTypeIsInvalid",
			req:         httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer([]byte(`{}`))),
			mockHandler: &HTTPMockHandler{},
			contentType: "text/plain",
			prep: func(req *http.Request, contentType string) error {
				req.Header.Set("Content-Type", contentType)
				return nil
			},
			want: &configv1alpha1.ResourceInterpreterResponse{
				UID:        "",
				Successful: false,
				Status: &configv1alpha1.RequestStatus{
					Message: "contentType=text/plain, expected application/json",
					Code:    http.StatusBadRequest,
				},
			},
		},
		{
			name:        "ServeHTTP_InvalidBodyJSON_JSONBodyIsInvalid",
			req:         httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer([]byte(`invalid-json`))),
			mockHandler: &HTTPMockHandler{},
			contentType: "application/json",
			prep: func(req *http.Request, contentType string) error {
				req.Header.Set("Content-Type", contentType)
				return nil
			},
			want: &configv1alpha1.ResourceInterpreterResponse{
				UID:        "",
				Successful: false,
				Status: &configv1alpha1.RequestStatus{
					Message: "json parse error",
					Code:    http.StatusBadRequest,
				},
			},
		},
		{
			name:        "ServeHTTP_ReadBodyError_FailedToReadBody",
			req:         httptest.NewRequest(http.MethodPost, "/", &mockBody{}),
			mockHandler: &HTTPMockHandler{},
			contentType: "application/json",
			prep: func(req *http.Request, contentType string) error {
				req.Header.Set("Content-Type", contentType)
				return nil
			},
			want: &configv1alpha1.ResourceInterpreterResponse{
				UID:        "",
				Successful: false,
				Status: &configv1alpha1.RequestStatus{
					Message: "mock read error",
					Code:    http.StatusBadRequest,
				},
			},
		},
		{
			name: "ServeHTTP_ValidRequest_RequestIsValid",
			req:  httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer([]byte(`{}`))),
			mockHandler: &HTTPMockHandler{
				response: Response{
					ResourceInterpreterResponse: configv1alpha1.ResourceInterpreterResponse{
						Successful: true,
						Status:     &configv1alpha1.RequestStatus{Code: http.StatusOK},
					},
				},
			},
			contentType: "application/json",
			prep: func(req *http.Request, contentType string) error {
				req.Header.Set("Content-Type", contentType)
				requestBody := configv1alpha1.ResourceInterpreterContext{
					Request: &configv1alpha1.ResourceInterpreterRequest{
						UID: "test-uid",
					},
				}
				body, err := json.Marshal(requestBody)
				if err != nil {
					return fmt.Errorf("failed to marshal request body: %v", err)
				}
				req.Body = io.NopCloser(bytes.NewBuffer(body))
				return nil
			},
			want: &configv1alpha1.ResourceInterpreterResponse{
				UID:        "test-uid",
				Successful: true,
				Status: &configv1alpha1.RequestStatus{
					Message: "",
					Code:    http.StatusOK,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			if err := test.prep(test.req, test.contentType); err != nil {
				t.Errorf("failed to prep serving http: %v", err)
			}
			webhook := NewWebhook(test.mockHandler, &Decoder{})
			webhook.ServeHTTP(recorder, test.req)
			if err := verifyResourceInterpreterResponse(recorder.Body.Bytes(), test.want); err != nil {
				t.Errorf("failed to verify resource interpreter response: %v", err)
			}
		})
	}
}

func TestWriteResponse(t *testing.T) {
	tests := []struct {
		name        string
		res         Response
		rec         *httptest.ResponseRecorder
		mockHandler *HTTPMockHandler
		decoder     *Decoder
		verify      func([]byte, *configv1alpha1.ResourceInterpreterResponse) error
		want        *configv1alpha1.ResourceInterpreterResponse
	}{
		{
			name: "WriteResponse_ValidValues_IsSucceeded",
			res: Response{
				ResourceInterpreterResponse: configv1alpha1.ResourceInterpreterResponse{
					UID:        "test-uid",
					Successful: true,
					Status:     &configv1alpha1.RequestStatus{Code: http.StatusOK},
				},
			},
			rec:         httptest.NewRecorder(),
			mockHandler: &HTTPMockHandler{},
			decoder:     &Decoder{},
			verify:      verifyResourceInterpreterResponse,
			want: &configv1alpha1.ResourceInterpreterResponse{
				UID:        "test-uid",
				Successful: true,
				Status: &configv1alpha1.RequestStatus{
					Message: "",
					Code:    http.StatusOK,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			webhook := NewWebhook(test.mockHandler, test.decoder)
			webhook.writeResponse(test.rec, test.res)
			if err := test.verify(test.rec.Body.Bytes(), test.want); err != nil {
				t.Errorf("failed to verify resource interpreter response: %v", err)
			}
		})
	}
}

func TestWriteResourceInterpreterResponse(t *testing.T) {
	tests := []struct {
		name        string
		mockHandler *HTTPMockHandler
		rec         io.Writer
		res         configv1alpha1.ResourceInterpreterContext
		verify      func(io.Writer, *configv1alpha1.ResourceInterpreterResponse) error
		want        *configv1alpha1.ResourceInterpreterResponse
	}{
		{
			name:        "WriteResourceInterpreterResponse_ValidValues_WriteIsSuccessful",
			mockHandler: &HTTPMockHandler{},
			rec:         httptest.NewRecorder(),
			res: configv1alpha1.ResourceInterpreterContext{
				Response: &configv1alpha1.ResourceInterpreterResponse{
					UID:        "test-uid",
					Successful: true,
					Status:     &configv1alpha1.RequestStatus{Code: http.StatusOK},
				},
			},
			verify: func(writer io.Writer, rir *configv1alpha1.ResourceInterpreterResponse) error {
				data, ok := writer.(*httptest.ResponseRecorder)
				if !ok {
					return fmt.Errorf("expected writer of type httptest.ResponseRecorder but got %T", writer)
				}
				return verifyResourceInterpreterResponse(data.Body.Bytes(), rir)
			},
			want: &configv1alpha1.ResourceInterpreterResponse{
				UID:        "test-uid",
				Successful: true,
				Status: &configv1alpha1.RequestStatus{
					Message: "",
					Code:    http.StatusOK,
				},
			},
		},
		{
			name:        "WriteResourceInterpreterResponse_FailedToWrite_WriterReachedMaxFailures",
			mockHandler: &HTTPMockHandler{},
			res: configv1alpha1.ResourceInterpreterContext{
				Response: &configv1alpha1.ResourceInterpreterResponse{},
			},
			rec: &limitedBadResponseWriter{maxFailures: 3},
			verify: func(writer io.Writer, _ *configv1alpha1.ResourceInterpreterResponse) error {
				data, ok := writer.(*limitedBadResponseWriter)
				if !ok {
					return fmt.Errorf("expected writer of type limitedBadResponseWriter but got %T", writer)
				}
				if data.failCount != data.maxFailures {
					return fmt.Errorf("expected %d write failures, got %d", data.maxFailures, data.failCount)
				}
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			webhook := NewWebhook(test.mockHandler, &Decoder{})
			webhook.writeResourceInterpreterResponse(test.rec, test.res)
			if err := test.verify(test.rec, test.want); err != nil {
				t.Errorf("failed to verify resource interpreter response: %v", err)
			}
		})
	}
}

// verifyResourceInterpreterResponse unmarshals the provided body into a
// ResourceInterpreterContext and verifies it matches the expected values in res2.
func verifyResourceInterpreterResponse(body []byte, res2 *configv1alpha1.ResourceInterpreterResponse) error {
	var resContext configv1alpha1.ResourceInterpreterContext
	if err := json.Unmarshal(body, &resContext); err != nil {
		return fmt.Errorf("failed to unmarshal body: %v", err)
	}
	if resContext.Response.UID != res2.UID {
		return fmt.Errorf("expected UID %s, but got %s", res2.UID, resContext.Response.UID)
	}
	if resContext.Response.Successful != res2.Successful {
		return fmt.Errorf("expected success status %t, but got %t", res2.Successful, resContext.Response.Successful)
	}
	if !strings.Contains(resContext.Response.Status.Message, res2.Status.Message) {
		return fmt.Errorf("expected message %s to be subset, but got %s", res2.Status.Message, resContext.Response.Status.Message)
	}
	if resContext.Response.Status.Code != res2.Status.Code {
		return fmt.Errorf("expected status code %d, but got %d", res2.Status.Code, resContext.Response.Status.Code)
	}
	return nil
}
