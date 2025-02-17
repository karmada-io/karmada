/*
Copyright 2025 The Karmada Authors.

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

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	operatorutil "github.com/karmada-io/karmada/operator/pkg/util"
)

var _ = ginkgo.Describe("Status testing", func() {
	var karmadaName string
	var karmadaObject *operatorv1alpha1.Karmada
	var err error

	ginkgo.Context("Karmada instance status testing", func() {
		ginkgo.BeforeEach(func() {
			karmadaName = KarmadaInstanceNamePrefix + rand.String(RandomStrLength)
			InitializeKarmadaInstance(operatorClient, testNamespace, karmadaName)
		})

		ginkgo.AfterEach(func() {
			err = operatorClient.OperatorV1alpha1().Karmadas(testNamespace).Delete(context.TODO(), karmadaName, metav1.DeleteOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("Check if the karmada status meets the expectations", func() {
			ginkgo.By("Get the latest karmada instance", func() {
				karmadaObject, err = operatorClient.OperatorV1alpha1().Karmadas(testNamespace).Get(context.TODO(), karmadaName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("Check if status.conditions meets the expectations", func() {
				conditions := karmadaObject.Status.Conditions
				gomega.Expect(len(conditions)).Should(gomega.BeNumerically(">", 0))
				// check if the Ready condition is true
				hasReadyCondition := false
				for i := range karmadaObject.Status.Conditions {
					switch karmadaObject.Status.Conditions[i].Type {
					case string(operatorv1alpha1.Ready):
						gomega.Expect(karmadaObject.Status.Conditions[i].Status).Should(gomega.Equal(metav1.ConditionTrue))
						hasReadyCondition = true
					}
				}
				gomega.Expect(hasReadyCondition).Should(gomega.BeTrue())
			})

			ginkgo.By("Check if the status.SecretRef can ref to the right secret", func() {
				secretRef := karmadaObject.Status.SecretRef
				gomega.Expect(secretRef).ShouldNot(gomega.BeNil())
				gomega.Expect(secretRef.Namespace).Should(gomega.Equal(karmadaObject.GetNamespace()))
				gomega.Expect(secretRef.Name).Should(gomega.Equal(operatorutil.AdminKarmadaConfigSecretName(karmadaObject.GetName())))
				_, err := kubeClient.CoreV1().Secrets(secretRef.Namespace).Get(context.TODO(), secretRef.Name, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("Check if the status.apiServerService can ref to the right service", func() {
				apiServerService := karmadaObject.Status.APIServerService
				gomega.Expect(apiServerService).ShouldNot(gomega.BeNil())
				gomega.Expect(apiServerService.Name).Should(gomega.Equal(operatorutil.KarmadaAPIServerName(karmadaObject.GetName())))
				_, err := kubeClient.CoreV1().Services(karmadaObject.GetNamespace()).Get(context.TODO(), apiServerService.Name, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})
})
