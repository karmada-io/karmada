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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// WaitResourceQuotaPresentOnClusters wait resourceQuota present on clusters until timeout.
func WaitResourceQuotaPresentOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if resourceQuota(%s/%s) present on member clusters", namespace, name), func() {
		for _, cluster := range clusters {
			WaitResourceQuotaPresentOnCluster(cluster, namespace, name)
		}
	})
}

// WaitResourceQuotaPresentOnCluster wait resourceQuota present on one cluster until timeout.
func WaitResourceQuotaPresentOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for resourceQuota(%s/%s) present on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func(g gomega.Gomega) (bool, error) {
		_, err := clusterClient.CoreV1().ResourceQuotas(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		g.Expect(err).NotTo(gomega.HaveOccurred())
		return true, nil
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitResourceQuotaDisappearOnClusters wait resourceQuota disappear on clusters until timeout.
func WaitResourceQuotaDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if resourceQuota(%s/%s) disappear on member clusters", namespace, name), func() {
		for _, cluster := range clusters {
			WaitResourceQuotaDisappearOnCluster(cluster, namespace, name)
		}
	})
}

// WaitResourceQuotaDisappearOnCluster wait resourceQuota disappear on one cluster until timeout.
func WaitResourceQuotaDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for resourceQuota(%s/%s) disappear on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.CoreV1().ResourceQuotas(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get resourceQuota(%s/%s) on cluster(%s), err: %v", namespace, name, cluster, err)
		return false
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}
