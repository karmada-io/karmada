/*
Copyright 2024 The Karmada Authors.

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
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Lazy activation policy testing", func() {
	var policyNamespace, policyName1, policyName2 string
	var deploymentNamespace, deploymentName string
	var deployment *appsv1.Deployment
	var policy1, policy2 *policyv1alpha1.PropagationPolicy
	var targetCluster1, targetCluster2 string

	newPropagationPolicy := func(policyNamespace, policyName, targetCluster string, deployment *appsv1.Deployment) *policyv1alpha1.PropagationPolicy {
		policy := testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{targetCluster},
			},
		})
		policy.Spec.ActivationPreference = policyv1alpha1.LazyActivation
		return policy
	}

	ginkgo.BeforeEach(func() {
		policyNamespace = testNamespace
		policyName1 = deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace = testNamespace
		deploymentName = policyName1
		policyName2 = deploymentNamePrefix + rand.String(RandomStrLength)

		deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)

		clusters := framework.ClusterNames()
		targetCluster1 = clusters[0]
		targetCluster2 = clusters[1]

		policy1 = newPropagationPolicy(policyNamespace, policyName1, targetCluster1, deployment)
		policy2 = newPropagationPolicy(policyNamespace, policyName2, targetCluster2, deployment)
	})

	ginkgo.BeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy1)
		framework.CreateDeployment(kubeClient, deployment)
		framework.WaitDeploymentPresentOnClusterFitWith(targetCluster1, deployment.Namespace, deployment.Name,
			func(deployment *appsv1.Deployment) bool {
				return true
			})

		framework.CreatePropagationPolicy(karmadaClient, policy2)

		ginkgo.DeferCleanup(func() {
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.WaitDeploymentDisappearOnCluster(targetCluster2, deployment.Namespace, deployment.Name)
			framework.WaitDeploymentDisappearOnCluster(targetCluster1, deployment.Namespace, deployment.Name)
		})
	})

	assertNotRefreshBehavior := func() {
		ginkgo.It("should not perform the refresh of the binding if new policy is Lazy", func() {

			ginkgo.By("check if deployment present on member cluster 1 is unchanged")
			framework.WaitDeploymentPresentOnClusterFitWith(targetCluster1, deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return true
				})

			refreshTime := fmt.Sprintf("%d", time.Now().Unix())
			refreshTimeLabelValue := map[string]string{
				"refresh-time": refreshTime,
			}
			framework.UpdateDeploymentLabels(kubeClient, deployment, refreshTimeLabelValue)

			ginkgo.By("check if deployment present on member cluster 2 has correct label value")
			framework.WaitDeploymentPresentOnClusterFitWith(targetCluster2, deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					actualRefreshTime, labelExist := deployment.GetLabels()["refresh-time"]
					return labelExist && actualRefreshTime == refreshTime
				})
		})
	}

	ginkgo.Context("when policy is deleted", func() {
		ginkgo.BeforeEach(func() {
			framework.RemovePropagationPolicy(karmadaClient, policy1.Namespace, policy1.Name)
		})

		assertNotRefreshBehavior()
	})

	ginkgo.Context("when policy is not matched", func() {
		ginkgo.BeforeEach(func() {
			policy1.Spec.ResourceSelectors[0].Name += rand.String(RandomStrLength)
			framework.UpdatePropagationPolicyWithSpec(karmadaClient, policy1.Namespace, policy1.Name, policy1.Spec)
		})

		assertNotRefreshBehavior()
	})
})
