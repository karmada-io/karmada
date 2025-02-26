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
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitSecretDisappearOnCluster wait secret disappear on cluster until timeout.
func WaitSecretDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for secret(%s/%s) disappear on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get secret(%s/%s) on cluster(%s), err: %v", namespace, name, cluster, err)
		return false
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitSecretDisappearOnClusters wait service disappear on member clusters until timeout.
func WaitSecretDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if secret(%s/%s) diappeare on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitSecretDisappearOnCluster(clusterName, namespace, name)
		}
	})
}
