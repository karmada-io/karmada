/*
Copyright 2023 The Karmada Authors.
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
	"time"

	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/pointer"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

/*
CronFederatedHPA focus on scaling FederatedHPA or other resource with scale subresource (e.g. Deployment, StatefulSet).
Test Case Overview:

	case 1:
		Scale FederatedHPA.
	case 2:
		Scale deployment.
	case 3:
		Test suspend rule in CronFederatedHPA
	case 4:
		Test unsuspend rule then suspend it in CronFederatedHPA
*/
var _ = ginkgo.Describe("[CronFederatedHPA] CronFederatedHPA testing", func() {
	var cronFHPAName, fhpaName, policyName, deploymentName string
	var cronFHPA *autoscalingv1alpha1.CronFederatedHPA
	var fhpa *autoscalingv1alpha1.FederatedHPA
	var deployment *appsv1.Deployment
	var policy *policyv1alpha1.PropagationPolicy

	ginkgo.BeforeEach(func() {
		cronFHPAName = cronFedratedHPANamePrefix + rand.String(RandomStrLength)
		fhpaName = federatedHPANamePrefix + rand.String(RandomStrLength)
		policyName = deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentName = policyName

		deployment = helper.NewDeployment(testNamespace, deploymentName)
		policy = helper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deploymentName,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
		})
	})

	ginkgo.JustBeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy)
		framework.CreateDeployment(kubeClient, deployment)
		ginkgo.DeferCleanup(func() {
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	// case 1: Scale FederatedHPA.
	ginkgo.Context("Scale FederatedHPA", func() {
		targetMinReplicas := pointer.Int32(2)
		targetMaxReplicas := pointer.Int32(100)

		ginkgo.BeforeEach(func() {
			// */1 * * * * means the rule will be triggered every 1 minute
			rule := helper.NewCronFederatedHPARule("scale-up", "*/1 * * * *", false, nil, targetMinReplicas, targetMaxReplicas)
			fhpa = helper.NewFederatedHPA(testNamespace, fhpaName, deploymentName)
			cronFHPA = helper.NewCronFederatedHPAWithScalingFHPA(testNamespace, cronFHPAName, fhpaName, rule)

			framework.CreateFederatedHPA(karmadaClient, fhpa)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveFederatedHPA(karmadaClient, testNamespace, fhpaName)
			framework.RemoveCronFederatedHPA(karmadaClient, testNamespace, cronFHPAName)
		})

		ginkgo.It("Test scale FederatedHPA testing", func() {
			framework.WaitDeploymentReplicasFitWith(framework.ClusterNames(), testNamespace, deploymentName, int(*fhpa.Spec.MinReplicas))

			// Create CronFederatedHPA to scale FederatedHPA
			framework.CreateCronFederatedHPA(karmadaClient, cronFHPA)

			// Wait CronFederatedHPA to scale FederatedHPA's minReplicas which will trigger scaling deployment's replicas to minReplicas
			framework.WaitDeploymentReplicasFitWith(framework.ClusterNames(), testNamespace, deploymentName, int(*targetMinReplicas))
		})
	})

	// case 2. Scale deployment.
	ginkgo.Context("Test scale Deployment", func() {
		targetReplicas := pointer.Int32(4)

		ginkgo.BeforeEach(func() {
			// */1 * * * * means the rule will be executed every 1 minute
			rule := helper.NewCronFederatedHPARule("scale-up", "*/1 * * * *", false, targetReplicas, nil, nil)
			cronFHPA = helper.NewCronFederatedHPAWithScalingDeployment(testNamespace, cronFHPAName, deploymentName, rule)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveCronFederatedHPA(karmadaClient, testNamespace, cronFHPAName)
		})

		ginkgo.It("Scale Deployment testing", func() {
			framework.WaitDeploymentReplicasFitWith(framework.ClusterNames(), testNamespace, deploymentName, int(*deployment.Spec.Replicas))

			// Create CronFederatedHPA to scale Deployment
			framework.CreateCronFederatedHPA(karmadaClient, cronFHPA)

			framework.WaitDeploymentReplicasFitWith(framework.ClusterNames(), testNamespace, deploymentName, int(*targetReplicas))
		})
	})

	// case 3. Test suspend rule in CronFederatedHPA
	ginkgo.Context("Test suspend rule in CronFederatedHPA", func() {
		ginkgo.BeforeEach(func() {
			// */1 * * * * means the rule will be executed every 1 minute
			rule := helper.NewCronFederatedHPARule("scale-up", "*/1 * * * *", true, pointer.Int32(30), nil, nil)
			cronFHPA = helper.NewCronFederatedHPAWithScalingDeployment(testNamespace, cronFHPAName, deploymentName, rule)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveCronFederatedHPA(karmadaClient, testNamespace, cronFHPAName)
		})

		ginkgo.It("Test suspend rule with CronFederatedHPA", func() {
			framework.WaitDeploymentReplicasFitWith(framework.ClusterNames(), testNamespace, deploymentName, int(*deployment.Spec.Replicas))

			// Create CronFederatedHPA to scale Deployment
			framework.CreateCronFederatedHPA(karmadaClient, cronFHPA)

			// */1 * * * * means the rule will be triggered every 1 minute
			// So wait for 1m30s and check whether the replicas changed and whether the suspend field works
			time.Sleep(time.Minute*1 + time.Second*30)
			framework.WaitDeploymentReplicasFitWith(framework.ClusterNames(), testNamespace, deploymentName, int(*deployment.Spec.Replicas))
		})
	})

	// case 4. Test unsuspend rule then suspend it in CronFederatedHPA
	ginkgo.Context("Test unsuspend rule then suspend it in CronFederatedHPA", func() {
		rule := autoscalingv1alpha1.CronFederatedHPARule{}
		targetReplicas := pointer.Int32(4)

		ginkgo.BeforeEach(func() {
			// */1 * * * * means the rule will be executed every 1 minute
			rule = helper.NewCronFederatedHPARule("scale-up", "*/1 * * * *", false, targetReplicas, nil, nil)
			cronFHPA = helper.NewCronFederatedHPAWithScalingDeployment(testNamespace, cronFHPAName, deploymentName, rule)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveCronFederatedHPA(karmadaClient, testNamespace, cronFHPAName)
		})

		ginkgo.It("Test unsuspend rule then suspend it in CronFederatedHPA", func() {
			// Step 1.Check the init replicas, which should be 3(deployment.Spec.Replicas)
			framework.WaitDeploymentReplicasFitWith(framework.ClusterNames(), testNamespace, deploymentName, int(*deployment.Spec.Replicas))

			// Step 2.Create CronFederatedHPA to scale Deployment
			framework.CreateCronFederatedHPA(karmadaClient, cronFHPA)
			framework.WaitDeploymentReplicasFitWith(framework.ClusterNames(), testNamespace, deploymentName, int(*targetReplicas))

			// Step 3.Update replicas to 3(deployment.Spec.Replicas)
			framework.UpdateDeploymentReplicas(kubeClient, deployment, *deployment.Spec.Replicas)

			// Step 4. Suspend rule
			rule.Suspend = pointer.Bool(true)
			framework.UpdateCronFederatedHPAWithRule(karmadaClient, testNamespace, cronFHPAName, []autoscalingv1alpha1.CronFederatedHPARule{rule})

			// Step 5. Check the replicas, which should not be changed
			// */1 * * * * means the rule will be triggered every 1 minute
			// So wait for 1m30s and check whether the replicas changed and whether the suspend field works
			time.Sleep(time.Minute*1 + time.Second*30)
			framework.WaitDeploymentReplicasFitWith(framework.ClusterNames(), testNamespace, deploymentName, int(*deployment.Spec.Replicas))
		})
	})
})
