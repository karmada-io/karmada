/*
Copyright 2026 The Karmada Authors.

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

package e2e

import (
	"context"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	karmadacontroller "github.com/karmada-io/karmada/operator/pkg/controller/karmada"
	operatorresource "github.com/karmada-io/karmada/test/e2e/framework/resource/operator"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("CRDTarball configuration testing", func() {
	var karmadaName string

	ginkgo.BeforeEach(func() {
		karmadaName = KarmadaInstanceNamePrefix + rand.String(RandomStrLength)
	})

	ginkgo.AfterEach(func() {
		operatorresource.DeleteKarmadaInstance(operatorClient, testNamespace, karmadaName)
	})

	ginkgo.Context("CRDTarball validation", func() {
		ginkgo.It("should update Ready condition and emit Warning event when HTTP URL is invalid", func() {
			ginkgo.By("create a Karmada instance with an invalid CRD tarball HTTP URL", func() {
				karmada := helper.NewKarmada(testNamespace, karmadaName)
				karmada.Spec.CRDTarball.HTTPSource.URL = "invalid-url"

				err := operatorresource.CreateKarmadaInstance(operatorClient, karmada)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("assert Ready condition becomes False with ValidationError reason", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					updated, err := operatorClient.OperatorV1alpha1().Karmadas(testNamespace).Get(context.TODO(), karmadaName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					var readyCond *metav1.Condition
					for i := range updated.Status.Conditions {
						if updated.Status.Conditions[i].Type == string(operatorv1alpha1.Ready) {
							readyCond = &updated.Status.Conditions[i]
							break
						}
					}

					g.Expect(readyCond).ShouldNot(gomega.BeNil())
					g.Expect(readyCond.Status).Should(gomega.Equal(metav1.ConditionFalse))
					g.Expect(readyCond.Reason).Should(gomega.Equal(karmadacontroller.ValidationErrorReason))
					g.Expect(readyCond.Message).Should(gomega.ContainSubstring(karmadacontroller.ErrInvalidCRDsRemoteURL.Error()))
				}, pollTimeout, pollInterval).Should(gomega.Succeed())
			})

			ginkgo.By("assert a Warning ValidationError event is emitted", func() {
				gomega.Eventually(func(g gomega.Gomega) bool {
					events, err := hostClient.CoreV1().Events(testNamespace).List(context.TODO(), metav1.ListOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					for _, event := range events.Items {
						if event.InvolvedObject.Name == karmadaName &&
							event.Type == corev1.EventTypeWarning &&
							event.Reason == karmadacontroller.ValidationErrorReason &&
							strings.Contains(event.Message, karmadacontroller.ErrInvalidCRDsRemoteURL.Error()) {
							return true
						}
					}

					return false
				}, pollTimeout, pollInterval).Should(gomega.BeTrue())
			})
		})
	})

	ginkgo.Context("CRDTarball functionality", func() {
		ginkgo.It("should deploy successfully when CRDTarball is not set (operator applies defaults)", func() {
			ginkgo.By("create a Karmada instance with nil CRDTarball", func() {
				karmada := helper.NewKarmada(testNamespace, karmadaName)
				karmada.Spec.CRDTarball = nil

				err := operatorresource.CreateKarmadaInstance(operatorClient, karmada)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("assert Karmada becomes Ready with defaulted CRDTarball", func() {
				operatorresource.WaitKarmadaReady(operatorClient, testNamespace, karmadaName, time.Now())
			})
		})
	})
})
