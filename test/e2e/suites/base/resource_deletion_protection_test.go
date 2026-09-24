/*
Copyright 2023 The Karmada Authors.

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

package base

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/retry"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[resource deletion protection] deletion protection testing", func() {
	var deploymentName, namespaceName string
	var namespace *corev1.Namespace
	var deployment *appsv1.Deployment

	// update resource's label like this will protect the resource.
	protectedLabelValues := map[string]string{
		workv1alpha2.DeletionProtectionLabelKey: workv1alpha2.DeletionProtectionAlways,
	}
	noProtectedLabelValues := map[string]string{
		workv1alpha2.DeletionProtectionLabelKey: "",
	}
	deletionProtectionErrorSubStr := "This resource is protected"

	// create the deployment and namespaces for test.
	ginkgo.BeforeEach(func() {
		deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
		namespaceName = "karmada-e2e-" + rand.String(RandomStrLength)
		namespace = helper.NewNamespace(namespaceName)
		deployment = helper.NewDeployment(namespaceName, deploymentName)
		framework.CreateNamespace(kubeClient, namespace)
		framework.CreateDeployment(kubeClient, deployment)
	})

	ginkgo.It("Delete the protected deployment", func() {
		framework.UpdateDeploymentLabels(kubeClient, deployment, protectedLabelValues)

		// the deletion operation should return an error like:
		// `admission webhook "resourcedeletionprotection.karmada.io" denied the request:
		// This resource is protected, please make sure to remove the label:
		// resourcetemplate.karmada.io/deletion-protected`
		err := kubeClient.AppsV1().Deployments(namespaceName).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
		gomega.Expect(err).Should(gomega.MatchError(gomega.ContainSubstring(deletionProtectionErrorSubStr)))
	})

	ginkgo.It("Delete the protected namespace", func() {
		framework.UpdateNamespaceLabels(kubeClient, namespace, protectedLabelValues)

		// the deletion operation should return an error too.
		err := kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespaceName, metav1.DeleteOptions{})
		gomega.Expect(err).Should(gomega.MatchError(gomega.ContainSubstring(deletionProtectionErrorSubStr)))
	})

	ginkgo.It("Delete the no protected namespace, the deployment is protected", func() {
		framework.UpdateDeploymentLabels(kubeClient, deployment, protectedLabelValues)

		// the deletion operation should not return an error, and the namespace
		// should not be deleted because the deployment was protected.
		err := kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespaceName, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		_, err = kubeClient.CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.It("Delete the namespace after the protection has been removed", func() {
		framework.UpdateNamespaceLabels(kubeClient, namespace, protectedLabelValues)
		err := kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespaceName, metav1.DeleteOptions{})
		gomega.Expect(err).Should(gomega.MatchError(gomega.ContainSubstring(deletionProtectionErrorSubStr)))

		framework.UpdateNamespaceLabels(kubeClient, namespace, noProtectedLabelValues)
		err = kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespaceName, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.It("Delete the protected Cluster", func() {
		clusterName := "cluster-deletion-protection-" + rand.String(RandomStrLength)
		cluster := &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName},
			Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Push},
		}
		_, err := karmadaClient.ClusterV1alpha1().Clusters().Create(context.TODO(), cluster, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		ginkgo.DeferCleanup(func() {
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				current, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
				if apierrors.IsNotFound(err) {
					return nil
				}
				if err != nil {
					return err
				}
				if current.ObjectMeta.Labels == nil {
					return nil
				}
				delete(current.ObjectMeta.Labels, workv1alpha2.DeletionProtectionLabelKey)
				_, err = karmadaClient.ClusterV1alpha1().Clusters().Update(context.TODO(), current, metav1.UpdateOptions{})
				return err
			})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			err = karmadaClient.ClusterV1alpha1().Clusters().Delete(context.TODO(), clusterName, metav1.DeleteOptions{})
			gomega.Expect(err == nil || apierrors.IsNotFound(err)).Should(gomega.BeTrue())
		})

		framework.UpdateClusterLabels(karmadaClient, clusterName, protectedLabelValues)
		err = karmadaClient.ClusterV1alpha1().Clusters().Delete(context.TODO(), clusterName, metav1.DeleteOptions{})
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(apierrors.IsForbidden(err)).Should(gomega.BeTrue())
		gomega.Expect(err.Error()).Should(gomega.ContainSubstring(workv1alpha2.DeletionProtectionLabelKey))

		_, err = karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			current, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			delete(current.ObjectMeta.Labels, workv1alpha2.DeletionProtectionLabelKey)
			_, err = karmadaClient.ClusterV1alpha1().Clusters().Update(context.TODO(), current, metav1.UpdateOptions{})
			return err
		})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		err = karmadaClient.ClusterV1alpha1().Clusters().Delete(context.TODO(), clusterName, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Eventually(func() bool {
			_, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, pollTimeout, pollInterval).Should(gomega.BeTrue())
	})

	ginkgo.AfterEach(func() {
		// remove the resource's protection.
		framework.UpdateDeploymentLabels(kubeClient, deployment, noProtectedLabelValues)
		framework.UpdateNamespaceLabels(kubeClient, namespace, noProtectedLabelValues)

		// remove the test namespace.
		framework.RemoveNamespace(kubeClient, namespaceName)
	})
})
