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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
)

var _ = ginkgo.Describe("API server sidecar configuration testing", func() {
	var karmadaName string
	var apiserver *appsv1.Deployment
	var sidecarContainer corev1.Container
	var err error

	ginkgo.Context("API server sidecar configuration testing", func() {
		ginkgo.AfterEach(func() {
			err = operatorClient.OperatorV1alpha1().Karmadas(testNamespace).Delete(context.TODO(), karmadaName, metav1.DeleteOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("Custom API server sidecar configuration", func() {
			ginkgo.By("Deploy a Karmada instance with API server sidecar configuration", func() {
				karmadaName = KarmadaInstanceNamePrefix + rand.String(RandomStrLength)
				mutateFn := func(karmada *operatorv1alpha1.Karmada) {
					karmada.Spec.Components.KarmadaAPIServer = &operatorv1alpha1.KarmadaAPIServer{
						SidecarContainers: []corev1.Container{
							{
								Name:  "foo",
								Image: "busybox:latest",
								Command: []string{
									"sh",
									"-c",
									"while true; do echo 'Sidecar container is running'; sleep 10; done",
								},
							},
						},
						ExtraArgs: map[string]string{
							"authorization-mode": "RBAC",
						},
					}
				}
				InitializeKarmadaInstance(operatorClient, testNamespace, karmadaName, mutateFn)
			})

			ginkgo.By("Check if API server sidecar configuration works", func() {
				apiserver, err = kubeClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), karmadaName+"-apiserver", metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				containers := apiserver.Spec.Template.Spec.Containers
				gomega.Expect(len(containers)).Should(gomega.Equal(2))
				sidecarContainer = getContainer(containers, "foo")
				gomega.Expect(sidecarContainer).ShouldNot(gomega.Equal(corev1.Container{}))
			})

			ginkgo.By("Check if the configuration of the main container will not be applied to the sidecar container", func() {
				targetCommand := "--authorization-mode=RBAC"
				containers := apiserver.Spec.Template.Spec.Containers
				mainContainer := getContainer(containers, "kube-apiserver")
				gomega.Expect(mainContainer).ShouldNot(gomega.Equal(corev1.Container{}))
				gomega.Expect(isCommandExist(mainContainer, targetCommand)).Should(gomega.Equal(true))
				gomega.Expect(isCommandExist(sidecarContainer, targetCommand)).Should(gomega.Equal(false), "the configuration of the main container should not be applied to the sidecar container")
			})
		})
	})
})

func getContainer(containers []corev1.Container, name string) corev1.Container {
	for _, container := range containers {
		if container.Name == name {
			return container
		}
	}
	return corev1.Container{}
}

func isCommandExist(container corev1.Container, command string) bool {
	for _, c := range container.Command {
		if c == command {
			return true
		}
	}
	return false
}
