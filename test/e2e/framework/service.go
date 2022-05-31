package framework

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// CreateService create Service.
func CreateService(client kubernetes.Interface, service *corev1.Service) {
	ginkgo.By(fmt.Sprintf("Creating Service(%s/%s)", service.Namespace, service.Name), func() {
		_, err := client.CoreV1().Services(service.Namespace).Create(context.TODO(), service, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveService delete Service.
func RemoveService(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing Service(%s/%s)", namespace, name), func() {
		err := client.CoreV1().Services(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitServicePresentOnClusterFitWith wait service present on member clusters sync with fit func.
func WaitServicePresentOnClusterFitWith(cluster, namespace, name string, fit func(service *corev1.Service) bool) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for service(%s/%s) synced on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		svc, err := clusterClient.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(svc)
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// WaitServicePresentOnClustersFitWith wait service present on cluster sync with fit func.
func WaitServicePresentOnClustersFitWith(clusters []string, namespace, name string, fit func(service *corev1.Service) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for service(%s/%s) synced on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitServicePresentOnClusterFitWith(clusterName, namespace, name, fit)
		}
	})
}

// WaitServiceDisappearOnCluster wait service disappear on cluster until timeout.
func WaitServiceDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for service(%s/%s) disappear on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get service(%s/%s) on cluster(%s), err: %v", namespace, name, cluster, err)
		return false
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// WaitServiceDisappearOnClusters wait service disappear on member clusters until timeout.
func WaitServiceDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if service(%s/%s) diappeare on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitServiceDisappearOnCluster(clusterName, namespace, name)
		}
	})
}

// UpdateServiceWithPatch update service with patch bytes.
func UpdateServiceWithPatch(client kubernetes.Interface, namespace, name string, patch []map[string]interface{}, patchType types.PatchType) {
	ginkgo.By(fmt.Sprintf("Updating service(%s/%s)", namespace, name), func() {
		bytes, err := json.Marshal(patch)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		_, err = client.CoreV1().Services(namespace).Patch(context.TODO(), name, patchType, bytes, metav1.PatchOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
