/*
Copyright 2021 The Karmada Authors.

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
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = framework.SerialDescribe("propagation with taint and toleration testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
		var policyNamespace, policyName string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var tolerationKey, tolerationValue string
		var clusterTolerations []corev1.Toleration
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = policyName
			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
			tolerationKey = "cluster-toleration.karmada.io"
			tolerationValue = framework.ClusterNames()[0]

			// set clusterTolerations to tolerate taints in member1.
			clusterTolerations = []corev1.Toleration{
				{
					Key:      tolerationKey,
					Operator: corev1.TolerationOpEqual,
					Value:    tolerationValue,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			}

			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
				ClusterTolerations: clusterTolerations,
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By("adding taints to clusters", func() {
				for _, clusterName := range framework.ClusterNames() {
					taints := constructAddedTaints(tolerationKey, clusterName)

					gomega.Eventually(func(g gomega.Gomega) (bool, error) {
						clusterObj := &clusterv1alpha1.Cluster{}
						err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj)
						g.Expect(err).NotTo(gomega.HaveOccurred())

						clusterObj.Spec.Taints = append(clusterObj.Spec.Taints, taints...)
						klog.Infof("update taints(%s) of cluster(%s)", clusterObj.Spec.Taints, clusterName)

						err = controlPlaneClient.Update(context.TODO(), clusterObj)
						if err != nil {
							klog.Errorf("Failed to update cluster(%s), err: %v", clusterName, err)
							return false, err
						}
						return true, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(true))
				}
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By("removing taints in cluster", func() {
				for _, clusterName := range framework.ClusterNames() {
					gomega.Eventually(func(g gomega.Gomega) (bool, error) {
						clusterObj := &clusterv1alpha1.Cluster{}
						err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj)
						g.Expect(err).NotTo(gomega.HaveOccurred())

						clusterObj.Spec.Taints = removeTargetFromSource(clusterObj.Spec.Taints, constructAddedTaints(tolerationKey, clusterName))
						klog.Infof("update taints(%s) of cluster(%s)", clusterObj.Spec.Taints, clusterName)

						err = controlPlaneClient.Update(context.TODO(), clusterObj)
						if err != nil {
							klog.Errorf("Failed to update cluster(%s), err: %v", clusterName, err)
							return false, err
						}
						return true, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(true))
				}
			})
		})

		ginkgo.BeforeEach(func() {
			// wait a little while for the karmada-scheduler to sync the cluster changes
			// before deploying the workload.
			// Note: 1 second might be not enough, bug should fit for the most cases.
			time.Sleep(time.Second)

			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
		})

		ginkgo.It("deployment with cluster tolerations testing", func() {
			ginkgo.By(fmt.Sprintf("check if deployment(%s/%s) only scheduled to tolerated cluster(%s)", deploymentNamespace, deploymentName, tolerationValue), func() {
				gomega.Eventually(func(g gomega.Gomega) {
					targetClusterNames := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
					g.Expect(len(targetClusterNames)).Should(gomega.Equal(1))
					g.Expect(targetClusterNames[0]).Should(gomega.Equal(tolerationValue))
				}, pollTimeout, pollInterval).Should(gomega.Succeed())
			})
		})
	})
})

func constructAddedTaints(tolerationKey, clusterName string) []corev1.Taint {
	return []corev1.Taint{
		{
			Key:    tolerationKey,
			Value:  clusterName,
			Effect: corev1.TaintEffectNoSchedule,
		},
	}
}

func removeTargetFromSource(source, target []corev1.Taint) []corev1.Taint {
	var result []corev1.Taint
	for si := range source {
		deleted := false
		for tj := range target {
			if source[si].MatchTaint(&target[tj]) {
				deleted = true
				break
			}
		}
		if !deleted {
			result = append(result, source[si])
		}
	}

	return result
}
