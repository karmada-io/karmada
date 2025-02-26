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

// CreateConfigMap create ConfigMap.
func CreateConfigMap(client kubernetes.Interface, configMap *corev1.ConfigMap) {
	ginkgo.By(fmt.Sprintf("Creating ConfigMap(%s/%s)", configMap.Namespace, configMap.Name), func() {
		_, err := client.CoreV1().ConfigMaps(configMap.Namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveConfigMap delete ConfigMap.
func RemoveConfigMap(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing ConfigMap(%s/%s)", namespace, name), func() {
		err := client.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitConfigMapPresentOnClustersFitWith wait configmap present on clusters sync with fit func.
func WaitConfigMapPresentOnClustersFitWith(clusters []string, namespace, name string, fit func(configmap *corev1.ConfigMap) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for configmap(%s/%s) synced on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitConfigMapPresentOnClusterFitWith(clusterName, namespace, name, fit)
		}
	})
}

// WaitConfigMapPresentOnClusterFitWith wait configmap present on member cluster sync with fit func.
func WaitConfigMapPresentOnClusterFitWith(cluster, namespace, name string, fit func(configmap *corev1.ConfigMap) bool) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for configmap(%s/%s) synced on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		configmap, err := clusterClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(configmap)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// UpdateConfigMapWithPatch update configmap with patch bytes.
func UpdateConfigMapWithPatch(client kubernetes.Interface, namespace, name string, patch []map[string]interface{}, patchType types.PatchType) {
	ginkgo.By(fmt.Sprintf("Updating configmap(%s/%s)", namespace, name), func() {
		bytes, err := json.Marshal(patch)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		_, err = client.CoreV1().ConfigMaps(namespace).Patch(context.TODO(), name, patchType, bytes, metav1.PatchOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitConfigMapDisappearOnCluster wait configmap disappear on cluster until timeout.
func WaitConfigMapDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for configmap(%s/%s) disappears on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get configmap(%s/%s) on cluster(%s), err: %v", namespace, name, cluster, err)
		return false
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitConfigMapDisappearOnClusters wait configmap disappear on member clusters until timeout.
func WaitConfigMapDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if configmap(%s/%s) disappears on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitConfigMapDisappearOnCluster(clusterName, namespace, name)
		}
	})
}
