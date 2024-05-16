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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machinerytypes "k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Authorization Webhooks", func() {
	allowHandler := func() *Webhook {
		handler := &fakeHandler{
			fn: func(ctx context.Context, req Request) Response {
				return Response{
					SubjectAccessReview: authorizationv1.SubjectAccessReview{
						Status: authorizationv1.SubjectAccessReviewStatus{
							Allowed: true,
						},
					},
				}
			},
		}
		webhook := &Webhook{
			Handler: handler,
		}

		return webhook
	}

	It("should invoke the handler to get a response", func() {
		By("setting up a webhook with an allow handler")
		webhook := allowHandler()

		By("invoking the webhook")
		resp := webhook.Handle(context.Background(), Request{})

		By("checking that it allowed the request")
		Expect(resp.Status.Allowed).To(BeTrue())
	})

	It("should ensure that the response's UID is set to the request's UID", func() {
		By("setting up a webhook")
		webhook := allowHandler()

		By("invoking the webhook")
		resp := webhook.Handle(context.Background(), Request{SubjectAccessReview: authorizationv1.SubjectAccessReview{ObjectMeta: metav1.ObjectMeta{UID: "foobar"}}})

		By("checking that the response share's the request's UID")
		Expect(resp.UID).To(Equal(machinerytypes.UID("foobar")))
	})

	It("should populate the status on a response if one is not provided", func() {
		By("setting up a webhook")
		webhook := allowHandler()

		By("invoking the webhook")
		resp := webhook.Handle(context.Background(), Request{})

		By("checking that the response share's the request's UID")
		Expect(resp.Status).To(Equal(authorizationv1.SubjectAccessReviewStatus{Allowed: true}))
	})

	It("shouldn't overwrite the status on a response", func() {
		By("setting up a webhook that sets a status")
		webhook := &Webhook{
			Handler: HandlerFunc(func(ctx context.Context, req Request) Response {
				return Response{
					SubjectAccessReview: authorizationv1.SubjectAccessReview{
						Status: authorizationv1.SubjectAccessReviewStatus{
							Allowed:         true,
							EvaluationError: "Ground Control to Major Tom",
						},
					},
				}
			}),
		}

		By("invoking the webhook")
		resp := webhook.Handle(context.Background(), Request{})

		By("checking that the message is intact")
		Expect(resp.Status).NotTo(BeNil())
		Expect(resp.Status.Allowed).To(BeTrue())
		Expect(resp.Status.EvaluationError).To(Equal("Ground Control to Major Tom"))
	})
})
