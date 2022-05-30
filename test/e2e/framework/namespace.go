package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// CreateNamespace create Namespace.
func CreateNamespace(client kubernetes.Interface, namespace *corev1.Namespace) {
	ginkgo.By(fmt.Sprintf("Creating Namespace(%s)", namespace.Name), func() {
		_, err := client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveNamespace delete Namespace.
func RemoveNamespace(client kubernetes.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Removing Namespace(%s)", name), func() {
		err := client.CoreV1().Namespaces().Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitNamespacePresentOnClusterByClient wait namespace present on cluster until timeout directly by kube client.
func WaitNamespacePresentOnClusterByClient(client kubernetes.Interface, name string) {
	klog.Infof("Waiting for namespace present on karmada control client")
	gomega.Eventually(func(g gomega.Gomega) (bool, error) {
		_, err := client.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
		g.Expect(err).NotTo(gomega.HaveOccurred())
		return true, nil
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// WaitNamespacePresentOnCluster wait namespace present on cluster until timeout.
func WaitNamespacePresentOnCluster(cluster, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for namespace present on cluster(%s)", cluster)
	gomega.Eventually(func(g gomega.Gomega) (bool, error) {
		_, err := clusterClient.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
		g.Expect(err).NotTo(gomega.HaveOccurred())
		return true, nil
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// WaitNamespacePresentOnClusters wait namespace present on clusters until timeout.
func WaitNamespacePresentOnClusters(clusters []string, name string) {
	ginkgo.By(fmt.Sprintf("Check if namespace(%s) present on member clusters", name), func() {
		for _, clusterName := range clusters {
			WaitNamespacePresentOnCluster(clusterName, name)
		}
	})
}

// WaitNamespaceDisappearOnCluster wait namespace disappear on cluster until timeout.
func WaitNamespaceDisappearOnCluster(cluster, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for namespace(%s) disappear on cluster(%s)", name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get namespace(%s) on cluster(%s), err: %v", name, cluster, err)
		return false
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// WaitNamespaceDisappearOnClusters wait namespace disappear on clusters until timeout.
func WaitNamespaceDisappearOnClusters(clusters []string, name string) {
	ginkgo.By(fmt.Sprintf("Check if namespace(%s) disappear on member clusters", name), func() {
		for _, clusterName := range clusters {
			WaitNamespaceDisappearOnCluster(clusterName, name)
		}
	})
}
