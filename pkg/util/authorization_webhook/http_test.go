/*
Copyright 2022 The Karmada Authors.

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

package authorization_webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authorizationv1 "k8s.io/api/authorization/v1"
)

var _ = Describe("Authentication Webhooks", func() {

	const (
		gvkJSONv1      = `"kind":"SubjectAccessReview","apiVersion":"authorization.k8s.io/v1"`
		gvkJSONv1beta1 = `"kind":"SubjectAccessReview","apiVersion":"authorization.k8s.io/v1beta1"`
	)

	Describe("HTTP Handler", func() {
		var respRecorder *httptest.ResponseRecorder
		webhook := &Webhook{
			Handler: nil,
		}
		BeforeEach(func() {
			respRecorder = &httptest.ResponseRecorder{
				Body: bytes.NewBuffer(nil),
			}
		})

		It("should return bad-request when given an empty body", func() {
			req := &http.Request{Body: nil}

			expected := `{"metadata":{"creationTimestamp":null},"spec":{},"status":{"allowed":false,"evaluationError":"request body is empty"}}
`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return bad-request when given the wrong content-type", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/foo"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBuffer(nil)},
			}

			expected := `{"metadata":{"creationTimestamp":null},"spec":{},"status":{"allowed":false,"evaluationError":"contentType=application/foo, expected application/json"}}
`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return bad-request when given an undecodable body", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBufferString("{")},
			}

			expected := `{"metadata":{"creationTimestamp":null},"spec":{},"status":{"allowed":false,` +
				`"evaluationError":"couldn't get version/kind; json parse error: unexpected end of JSON input"}}
`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return the response given by the handler with version defaulted to v1", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBufferString(`{"spec":{"token":"foobar"}}`)},
			}
			webhook := &Webhook{
				Handler: &fakeHandler{},
			}

			expected := fmt.Sprintf(`{%s,"metadata":{"creationTimestamp":null},"spec":{},"status":{"allowed":true}}
`, gvkJSONv1)

			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return the v1 response given by the handler", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBufferString(fmt.Sprintf(`{%s,"spec":{"user":"foobar"}}`, gvkJSONv1))},
			}
			webhook := &Webhook{
				Handler: &fakeHandler{},
			}

			expected := fmt.Sprintf(`{%s,"metadata":{"creationTimestamp":null},"spec":{},"status":{"allowed":true}}
`, gvkJSONv1)
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return the v1beta1 response given by the handler", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBufferString(fmt.Sprintf(`{%s,"spec":{"user":"foobar"}}`, gvkJSONv1beta1))},
			}
			webhook := &Webhook{
				Handler: &fakeHandler{},
			}

			expected := fmt.Sprintf(`{%s,"metadata":{"creationTimestamp":null},"spec":{},"status":{"allowed":true}}
`, gvkJSONv1beta1)
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should present the Context from the HTTP request, if any", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBufferString(`{"spec":{"token":"foobar"}}`)},
			}
			type ctxkey int
			const key ctxkey = 1
			const value = "from-ctx"
			webhook := &Webhook{
				Handler: &fakeHandler{
					fn: func(ctx context.Context, req Request) Response {
						<-ctx.Done()
						return NoOpinion(ctx.Value(key).(string))
					},
				},
			}

			expected := fmt.Sprintf(`{%s,"metadata":{"creationTimestamp":null},"spec":{},"status":{"allowed":false,"reason":%q}}
`, gvkJSONv1, value)

			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), key, value))
			cancel()
			webhook.ServeHTTP(respRecorder, req.WithContext(ctx))
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should mutate the Context from the HTTP request, if func supplied", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBufferString(`{"spec":{"user":"foobar"}}`)},
			}
			type ctxkey int
			const key ctxkey = 1
			webhook := &Webhook{
				Handler: &fakeHandler{
					fn: func(ctx context.Context, req Request) Response {
						return NoOpinion(ctx.Value(key).(string))
					},
				},
				WithContextFunc: func(ctx context.Context, r *http.Request) context.Context {
					return context.WithValue(ctx, key, r.Header["Content-Type"][0])
				},
			}

			expected := fmt.Sprintf(`{%s,"metadata":{"creationTimestamp":null},"spec":{},"status":{"allowed":false,"reason":%q}}
`, gvkJSONv1, "application/json")

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			webhook.ServeHTTP(respRecorder, req.WithContext(ctx))
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should error when given a NoBody", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   http.NoBody,
			}

			expected := `{"metadata":{"creationTimestamp":null},"spec":{},"status":{"user":{},"error":"request body is empty"}}
	`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})
	})
})

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

type fakeHandler struct {
	invoked bool
	fn      func(context.Context, Request) Response
}

func (h *fakeHandler) Handle(ctx context.Context, req Request) Response {
	h.invoked = true
	if h.fn != nil {
		return h.fn(ctx, req)
	}
	return Response{SubjectAccessReview: authorizationv1.SubjectAccessReview{
		Status: authorizationv1.SubjectAccessReviewStatus{
			Allowed: true,
		},
	}}
}
