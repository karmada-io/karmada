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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authorizationv1 "k8s.io/api/authorization/v1"
)

var _ = Describe("Response", func() {
	var (
		reason  = "reason"
		fakeErr = fmt.Errorf("fake")
	)

	Describe("#Allowed", func() {
		It("should return the expected status", func() {
			Expect(Allowed()).To(Equal(Response{
				SubjectAccessReview: authorizationv1.SubjectAccessReview{
					Status: authorizationv1.SubjectAccessReviewStatus{
						Allowed: true,
					},
				},
			}))
		})
	})

	Describe("#Denied", func() {
		It("should return the expected status", func() {
			Expect(Denied(reason)).To(Equal(Response{
				SubjectAccessReview: authorizationv1.SubjectAccessReview{
					Status: authorizationv1.SubjectAccessReviewStatus{
						Allowed: false,
						Denied:  true,
						Reason:  reason,
					},
				},
			}))
		})
	})

	Describe("#NoOpinion", func() {
		It("should return the expected status", func() {
			Expect(NoOpinion(reason)).To(Equal(Response{
				SubjectAccessReview: authorizationv1.SubjectAccessReview{
					Status: authorizationv1.SubjectAccessReviewStatus{
						Allowed: false,
						Reason:  reason,
					},
				},
			}))
		})
	})

	Describe("#Errored", func() {
		It("should return the expected status", func() {
			Expect(Errored(fakeErr)).To(Equal(Response{
				SubjectAccessReview: authorizationv1.SubjectAccessReview{
					Status: authorizationv1.SubjectAccessReviewStatus{
						EvaluationError: fakeErr.Error(),
					},
				},
			}))
		})
	})
})
