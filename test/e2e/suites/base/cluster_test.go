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

package base

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
)

var _ = framework.SerialDescribe("cluster status ready condition collect testing", func() {
	var numOfFailedClusters int

	ginkgo.It("disable cluster by changing cluster APIEndpoint", func() {
		var disabledClusters []string
		numOfFailedClusters = 1
		targetClusterNames := framework.ClusterNames()

		ginkgo.By("disable cluster", func() {
			remainingToDisable := numOfFailedClusters
			for _, targetClusterName := range targetClusterNames {
				if remainingToDisable > 0 {
					klog.Infof("Set cluster %s to disable.", targetClusterName)
					err := disableCluster(controlPlaneClient, targetClusterName)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					disabledClusters = append(disabledClusters, targetClusterName)
					remainingToDisable--
				}
			}
		})

		ginkgo.By("wait for the cluster status to be unknown", func() {
			for _, disabledCluster := range disabledClusters {
				// wait for the current cluster status Ready condition to false
				framework.WaitClusterFitWith(controlPlaneClient, disabledCluster, func(cluster *clusterv1alpha1.Cluster) bool {
					readyCondition := meta.FindStatusCondition(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady)
					return readyCondition != nil && readyCondition.Status == metav1.ConditionFalse
				})
			}
		})

		ginkgo.By("recover cluster", func() {
			for _, disabledCluster := range disabledClusters {
				klog.Infof("cluster %s is waiting for recovering", disabledCluster)
				originalAPIEndpoint, err := getClusterAPIEndpoint(disabledCluster)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = recoverCluster(controlPlaneClient, disabledCluster, originalAPIEndpoint)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			}
		})

		ginkgo.By("wait for the cluster status to be true", func() {
			for _, disabledCluster := range disabledClusters {
				// wait for the current cluster status Ready condition to true
				framework.WaitClusterFitWith(controlPlaneClient, disabledCluster, func(cluster *clusterv1alpha1.Cluster) bool {
					readyCondition := meta.FindStatusCondition(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady)
					return readyCondition != nil && readyCondition.Status == metav1.ConditionTrue
				})
			}
		})
	})
})

// disableCluster will set wrong API endpoint of current cluster
func disableCluster(c client.Client, clusterName string) error {
	err := wait.PollUntilContextTimeout(context.TODO(), pollInterval, pollTimeout, true, func(ctx context.Context) (done bool, err error) {
		clusterObj := &clusterv1alpha1.Cluster{}
		if err := c.Get(ctx, client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
			return false, err
		}
		// set the APIEndpoint of matched cluster to a wrong value
		unavailableAPIEndpoint := "https://172.19.1.3:6443"
		clusterObj.Spec.APIEndpoint = unavailableAPIEndpoint
		if err := c.Update(ctx, clusterObj); err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	return err
}

// recoverCluster will recover API endpoint of the disable cluster
func recoverCluster(c client.Client, clusterName string, originalAPIEndpoint string) error {
	err := wait.PollUntilContextTimeout(context.TODO(), pollInterval, pollTimeout, true, func(ctx context.Context) (done bool, err error) {
		clusterObj := &clusterv1alpha1.Cluster{}
		if err := c.Get(ctx, client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
			return false, err
		}
		clusterObj.Spec.APIEndpoint = originalAPIEndpoint
		if err := c.Update(ctx, clusterObj); err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		klog.Infof("recovered API endpoint is %s", clusterObj.Spec.APIEndpoint)
		return true, nil
	})
	return err
}

// get the API endpoint of a specific cluster
func getClusterAPIEndpoint(clusterName string) (apiEndpoint string, err error) {
	for _, cluster := range framework.Clusters() {
		if cluster.Name == clusterName {
			apiEndpoint = cluster.Spec.APIEndpoint
			klog.Infof("original API endpoint of the cluster %s is %s", clusterName, apiEndpoint)
			return apiEndpoint, nil
		}
	}
	return apiEndpoint, fmt.Errorf("cluster %s not found", clusterName)
}
