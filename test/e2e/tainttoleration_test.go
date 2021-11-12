/*
Copyright The Karmada Authors.

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

package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("propagation with taint and toleration testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := testNamespace
		deploymentName := policyName
		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		tolerationKey := "cluster-toleration.karmada.io"
		tolerationValue := "member1"

		// set clusterTolerations to tolerate taints in member1.
		clusterTolerations := []corev1.Toleration{
			{
				Key:      tolerationKey,
				Operator: corev1.TolerationOpEqual,
				Value:    tolerationValue,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		}

		policy := helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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

		ginkgo.It("deployment with cluster tolerations testing", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)

			ginkgo.By(fmt.Sprintf("check if deployment(%s/%s) only scheduled to tolerated cluster(%s)", deploymentNamespace, deploymentName, tolerationValue), func() {
				gomega.Eventually(func(g gomega.Gomega) {
					targetClusterNames, err := getTargetClusterNames(deployment)
					g.Expect(err).ShouldNot(gomega.HaveOccurred())
					g.Expect(len(targetClusterNames) == 1).Should(gomega.BeTrue())
					g.Expect(targetClusterNames[0] == tolerationValue).Should(gomega.BeTrue())
				}, pollTimeout, pollInterval).Should(gomega.Succeed())
			})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
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
