package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

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

	ginkgo.It("delete the protected deployment", func() {
		framework.UpdateDeploymentLabels(kubeClient, deployment, protectedLabelValues)

		// the deletion operation should return an error like:
		// `admission webhook "resourcedeletionprotection.karmada.io" denied the request:
		// This resource is protected, please make sure to remove the label:
		// resourcetemplate.karmada.io/deletion-protected`
		err := kubeClient.AppsV1().Deployments(namespaceName).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
		gomega.Expect(err).Should(gomega.MatchError(gomega.ContainSubstring(deletionProtectionErrorSubStr)))
	})

	ginkgo.It("delete the protected namespace", func() {
		framework.UpdateNamespaceLabels(kubeClient, namespace, protectedLabelValues)

		// the deletion operation should return an error too.
		err := kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespaceName, metav1.DeleteOptions{})
		gomega.Expect(err).Should(gomega.MatchError(gomega.ContainSubstring(deletionProtectionErrorSubStr)))
	})

	ginkgo.It("delete the no protected namespace, the deployment is protected", func() {
		framework.UpdateDeploymentLabels(kubeClient, deployment, protectedLabelValues)

		// the deletion operation should not return an error, and the namespace
		// should not be deleted because the deployment was protected.
		err := kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespaceName, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		_, err = kubeClient.CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.It("delete the namespace after the protection has been removed", func() {
		framework.UpdateNamespaceLabels(kubeClient, namespace, protectedLabelValues)
		err := kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespaceName, metav1.DeleteOptions{})
		gomega.Expect(err).Should(gomega.MatchError(gomega.ContainSubstring(deletionProtectionErrorSubStr)))

		framework.UpdateNamespaceLabels(kubeClient, namespace, noProtectedLabelValues)
		err = kubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespaceName, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.AfterEach(func() {
		// remove the resource's protection.
		framework.UpdateDeploymentLabels(kubeClient, deployment, noProtectedLabelValues)
		framework.UpdateNamespaceLabels(kubeClient, namespace, noProtectedLabelValues)

		// remove the test namespace.
		framework.RemoveNamespace(kubeClient, namespaceName)
	})
})
