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

// CreatePVC create PersistentVolumeClaim.
func CreatePVC(client kubernetes.Interface, pvc *corev1.PersistentVolumeClaim) {
	ginkgo.By(fmt.Sprintf("Creating PersistentVolumeClaim(%s/%s)", pvc.Namespace, pvc.Name), func() {
		_, err := client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemovePVC delete PersistentVolumeClaim.
func RemovePVC(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing PersistentVolumeClaim(%s/%s)", namespace, name), func() {
		err := client.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitPVCPresentOnClustersFitWith wait PersistentVolumeClaim present on clusters sync with fit func.
func WaitPVCPresentOnClustersFitWith(clusters []string, namespace, name string, fit func(pvc *corev1.PersistentVolumeClaim) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for PersistentVolumeClaim(%s/%s) synced on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitPVCPresentOnClusterFitWith(clusterName, namespace, name, fit)
		}
	})
}

// WaitPVCPresentOnClusterFitWith wait PersistentVolumeClaim present on member cluster sync with fit func.
func WaitPVCPresentOnClusterFitWith(cluster, namespace, name string, fit func(pvc *corev1.PersistentVolumeClaim) bool) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for PersistentVolumeClaim(%s/%s) synced on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		pvc, err := clusterClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(pvc)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitPVCDisappearOnCluster wait PersistentVolumeClaim disappear on cluster until timeout.
func WaitPVCDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for PersistentVolumeClaim(%s/%s) disappear on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get PersistentVolumeClaim(%s/%s) on cluster(%s), err: %v", namespace, name, cluster, err)
		return false
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitPVCDisappearOnClusters Wait for the PersistentVolumeClaim to disappear on member clusters until timeout.
func WaitPVCDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if PersistentVolumeClaim(%s/%s) diappears on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitPVCDisappearOnCluster(clusterName, namespace, name)
		}
	})
}
