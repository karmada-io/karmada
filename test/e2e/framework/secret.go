package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// CreateSecret create Secret.
func CreateSecret(client kubernetes.Interface, secret *corev1.Secret) {
	ginkgo.By(fmt.Sprintf("Creating Secret(%s/%s)", secret.Namespace, secret.Name), func() {
		_, err := client.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveSecret delete Secret.
func RemoveSecret(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing Secret(%s/%s)", namespace, name), func() {
		err := client.CoreV1().Secrets(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitSecretPresentOnClustersFitWith wait secret present on clusters sync with fit func.
func WaitSecretPresentOnClustersFitWith(clusters []string, namespace, name string, fit func(secret *corev1.Secret) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for secret(%s/%s) synced on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitSecretPresentOnClusterFitWith(clusterName, namespace, name, fit)
		}
	})
}

// WaitSecretPresentOnClusterFitWith wait secret present on member cluster sync with fit func.
func WaitSecretPresentOnClusterFitWith(cluster, namespace, name string, fit func(secret *corev1.Secret) bool) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for secret(%s/%s) synced on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		secret, err := clusterClient.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(secret)
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// WaitSecretDisappearOnCluster wait secret disappear on cluster until timeout.
func WaitSecretDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for secret disappear on cluster(%s)", cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// WaitSecretDisappearOnClusters wait service disappear on member clusters until timeout.
func WaitSecretDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if secret(%s/%s) diappeare on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitSecretDisappearOnCluster(clusterName, namespace, name)
		}
	})
}
