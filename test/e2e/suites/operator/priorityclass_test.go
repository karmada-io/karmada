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
	"github.com/karmada-io/karmada/test/e2e/framework/resource/operator"
)

var _ = ginkgo.Describe("PriorityClass configuration testing", func() {
	var karmadaName string
	var karmadaObject *operatorv1alpha1.Karmada
	var err error

	ginkgo.Context("PriorityClass configuration testing", func() {
		ginkgo.BeforeEach(func() {
			karmadaName = KarmadaInstanceNamePrefix + rand.String(RandomStrLength)
			InitializeKarmadaInstance(operatorClient, testNamespace, karmadaName)
		})

		ginkgo.AfterEach(func() {
			err = operatorClient.OperatorV1alpha1().Karmadas(testNamespace).Delete(context.TODO(), karmadaName, metav1.DeleteOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("Custom priorityClass configuration", func() {
			ginkgo.By("Check if default value is system-node-critical", func() {
				// take etcd as a representative of StatefulSet.
				etcd, err := kubeClient.AppsV1().StatefulSets(testNamespace).Get(context.TODO(), karmadaName+"-etcd", metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(etcd.Spec.Template.Spec.PriorityClassName).Should(gomega.Equal("system-node-critical"))

				// take karmada-apiserver as a representative of Deployment.
				karmadaApiserver, err := kubeClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), karmadaName+"-apiserver", metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(karmadaApiserver.Spec.Template.Spec.PriorityClassName).Should(gomega.Equal("system-node-critical"))
			})

			ginkgo.By("Set priorityClass to system-cluster-critical", func() {
				karmadaObject, err = operatorClient.OperatorV1alpha1().Karmadas(testNamespace).Get(context.TODO(), karmadaName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				karmadaObject.Spec.Components.Etcd.Local.PriorityClassName = "system-cluster-critical"
				karmadaObject.Spec.Components.KarmadaAPIServer.PriorityClassName = "system-cluster-critical"
				operator.UpdateKarmadaInstanceWithSpec(operatorClient, testNamespace, karmadaName, karmadaObject.Spec)
				operator.WaitKarmadaReady(operatorClient, testNamespace, karmadaName, operator.GetLastTransitionTime(karmadaObject, operatorv1alpha1.Ready))
			})

			ginkgo.By("Check if the PriorityClass is applied correctly", func() {
				// take etcd as a representative of StatefulSet.
				etcd, err := kubeClient.AppsV1().StatefulSets(testNamespace).Get(context.TODO(), karmadaName+"-etcd", metav1.GetOptions{ResourceVersion: "0"})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(etcd.Spec.Template.Spec.PriorityClassName).Should(gomega.Equal("system-cluster-critical"))

				// take karmada-apiserver as a representative of Deployment.
				karmadaApiserver, err := kubeClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), karmadaName+"-apiserver", metav1.GetOptions{ResourceVersion: "0"})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(karmadaApiserver.Spec.Template.Spec.PriorityClassName).Should(gomega.Equal("system-cluster-critical"))
			})
		})
	})
})
