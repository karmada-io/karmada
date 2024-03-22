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
	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Lazy activation policy testing", func() {
	ginkgo.Context("Policy created before resource testing", func() {
		var policy *policyv1alpha1.PropagationPolicy
		var deployment *appsv1.Deployment
		var targetMember string

		ginkgo.BeforeEach(func() {
			targetMember = framework.ClusterNames()[0]
			policyNamespace := testNamespace
			policyName := deploymentNamePrefix + rand.String(RandomStrLength)

			deployment = testhelper.NewDeployment(testNamespace, policyName)

			policy = testhelper.NewLazyPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				}}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetMember},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})

			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool { return true })
		})

		ginkgo.It("policy created before resource testing", func() {
			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool { return true })
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool { return true })
		})
	})
})
