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

package base

import (
	"context"
	"sort"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("deployment replicas syncer testing", func() {
	var namespace string
	var deploymentName, hpaName, policyName, bindingName string
	var deployment *appsv1.Deployment
	var hpa *autoscalingv2.HorizontalPodAutoscaler
	var policy *policyv1alpha1.PropagationPolicy
	var targetClusters []string

	ginkgo.BeforeEach(func() {
		namespace = testNamespace
		deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
		hpaName = deploymentName
		policyName = deploymentName
		bindingName = names.GenerateBindingName(util.DeploymentKind, deploymentName)

		// sort member clusters in increasing order
		targetClusters = framework.ClusterNames()[0:2]
		sort.Strings(targetClusters)

		deployment = helper.NewDeployment(namespace, deploymentName)
		hpa = helper.NewHPA(namespace, hpaName, deploymentName)
		hpa.Spec.MinReplicas = ptr.To[int32](2)
		policy = helper.NewPropagationPolicy(namespace, policyName, []policyv1alpha1.ResourceSelector{
			{APIVersion: deployment.APIVersion, Kind: deployment.Kind, Name: deployment.Name},
			{APIVersion: hpa.APIVersion, Kind: hpa.Kind, Name: hpa.Name},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: targetClusters,
			},
		})
	})

	ginkgo.JustBeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy)
		framework.CreateDeployment(kubeClient, deployment)
		framework.CreateHPA(kubeClient, hpa)

		ginkgo.DeferCleanup(func() {
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemoveHPA(kubeClient, namespace, hpa.Name)
			framework.WaitDeploymentDisappearOnClusters(targetClusters, deployment.Namespace, deployment.Name)
		})
	})

	ginkgo.Context("when policy is Duplicated schedule type", func() {
		ginkgo.BeforeEach(func() {
			deployment.Spec.Replicas = ptr.To[int32](2)
		})

		// Case 1: Deployment(replicas=2) | Policy(Duplicated, two clusters) | HPA(minReplicas=2)
		// Expected result: hpa scaling not take effect in updating spec, manually modify spec have no action.
		ginkgo.It("general case combined hpa scaling and manually modify in Duplicated type", func() {
			ginkgo.By("step1: propagate each 2 replicas to two clusters", func() {
				assertDeploymentWorkloadReplicas(namespace, deploymentName, targetClusters, []int32{2, 2})
				assertDeploymentTemplateReplicas(namespace, deploymentName, 2)
			})

			ginkgo.By("step2: hpa scale each member cluster replicas from 2 to 3", func() {
				framework.UpdateHPAWithMinReplicas(kubeClient, namespace, hpa.Name, 3)
				assertDeploymentWorkloadReplicas(namespace, deploymentName, targetClusters, []int32{3, 3})
				assertDeploymentTemplateReplicas(namespace, deploymentName, 2)
			})

			ginkgo.By("step3: manually add deployment template replicas from 2 to 4", func() {
				framework.UpdateDeploymentReplicas(kubeClient, deployment, 4)
				assertDeploymentWorkloadReplicas(namespace, deploymentName, targetClusters, []int32{3, 3})
				assertDeploymentTemplateReplicas(namespace, deploymentName, 4)
			})

			ginkgo.By("step4: manually decrease deployment template replicas from 2 to 1", func() {
				framework.UpdateDeploymentReplicas(kubeClient, deployment, 1)
				assertDeploymentWorkloadReplicas(namespace, deploymentName, targetClusters, []int32{3, 3})
				assertDeploymentTemplateReplicas(namespace, deploymentName, 1)
			})
		})
	})

	ginkgo.Context("when policy is Divided schedule type, each cluster have more that one replica", func() {
		ginkgo.BeforeEach(func() {
			policy.Spec.Placement.ReplicaScheduling = helper.NewStaticWeightPolicyStrategy(targetClusters, []int64{1, 1})
			deployment.Spec.Replicas = ptr.To[int32](4)
		})

		// Case 2: Deployment(replicas=4) | Policy(Divided, two clusters 1:1) | HPA(minReplicas=2)
		// Expected result: hpa scaling can take effect in updating spec, while manually modify not.
		ginkgo.It("general case combined hpa scaling and manually modify in Divided type", func() {
			ginkgo.By("step1: propagate 4 replicas to two clusters", func() {
				assertDeploymentWorkloadReplicas(namespace, deploymentName, targetClusters, []int32{2, 2})
				assertDeploymentTemplateReplicas(namespace, deploymentName, 4)
			})

			ginkgo.By("step2: hpa scale each member cluster replicas from 2 to 3", func() {
				framework.UpdateHPAWithMinReplicas(kubeClient, namespace, hpa.Name, 3)
				assertDeploymentWorkloadReplicas(namespace, deploymentName, targetClusters, []int32{3, 3})
				assertDeploymentTemplateReplicas(namespace, deploymentName, 6)
			})

			ginkgo.By("step3: manually add deployment template replicas from 6 to 10", func() {
				framework.UpdateDeploymentReplicas(kubeClient, deployment, 10)
				assertDeploymentWorkloadReplicas(namespace, deploymentName, targetClusters, []int32{3, 3})
				assertDeploymentTemplateReplicas(namespace, deploymentName, 6)
			})

			ginkgo.By("step4: manually decrease deployment template replicas from 6 to 2", func() {
				framework.UpdateDeploymentReplicas(kubeClient, deployment, 2)
				assertDeploymentWorkloadReplicas(namespace, deploymentName, targetClusters, []int32{3, 3})
				assertDeploymentTemplateReplicas(namespace, deploymentName, 6)
			})
		})
	})

	ginkgo.Context("when policy is Divided schedule type, one cluster have no replica", func() {
		ginkgo.BeforeEach(func() {
			policy.Spec.Placement.ReplicaScheduling = helper.NewStaticWeightPolicyStrategy(targetClusters, []int64{1, 1})
			deployment.Spec.Replicas = ptr.To[int32](1)
			hpa.Spec.MinReplicas = ptr.To[int32](1)
		})

		// Case 3: Deployment(replicas=1) | Policy(Divided, two clusters 1:1) | HPA(minReplicas=1)
		// Expected result: manually modify can take effect in updating spec.
		ginkgo.It("0/1 case, manually modify replicas from 1 to 2", func() {
			ginkgo.By("step1: propagate 1 replicas to two clusters", func() {
				assertDeploymentTemplateReplicas(namespace, deploymentName, 1)
			})

			ginkgo.By("step2: manually add deployment template replicas from 1 to 2", func() {
				framework.UpdateDeploymentReplicas(kubeClient, deployment, 2)
				assertDeploymentWorkloadReplicas(namespace, deploymentName, targetClusters, []int32{1, 1})
				assertDeploymentTemplateReplicas(namespace, deploymentName, 2)
			})
		})
	})

	ginkgo.Context("when policy is Divided schedule type, remove one cluster's replicas", func() {
		ginkgo.BeforeEach(func() {
			policy.Spec.Placement.ReplicaScheduling = helper.NewStaticWeightPolicyStrategy(targetClusters, []int64{1, 1})
			deployment.Spec.Replicas = ptr.To[int32](2)
			hpa.Spec.MinReplicas = ptr.To[int32](1)
		})

		// Case 4: Deployment(replicas=2) | Policy(Divided, two clusters 1:1) | HPA(minReplicas=1)
		// Expected result: manually modify can take effect in updating spec.
		ginkgo.It("0/1 case, manually modify replicas from 2 to 1", func() {
			ginkgo.By("step1: propagate 2 replicas to two clusters", func() {
				assertDeploymentWorkloadReplicas(namespace, deploymentName, targetClusters, []int32{1, 1})
				assertDeploymentTemplateReplicas(namespace, deploymentName, 2)
			})

			ginkgo.By("step2: manually add deployment template replicas from 2 to 1", func() {
				framework.UpdateDeploymentReplicas(kubeClient, deployment, 1)
				framework.WaitResourceBindingFitWith(karmadaClient, namespace, bindingName, func(rb *workv1alpha2.ResourceBinding) bool {
					return len(rb.Status.AggregatedStatus) == 1
				})
				assertDeploymentTemplateReplicas(namespace, deploymentName, 1)
			})
		})
	})

	ginkgo.Context("when policy is Divided schedule type, propagate 1 replica but hpa minReplicas is 2", func() {
		ginkgo.BeforeEach(func() {
			policy.Spec.Placement.ReplicaScheduling = helper.NewStaticWeightPolicyStrategy(targetClusters, []int64{1, 1})
			deployment.Spec.Replicas = ptr.To[int32](1)
			hpa.Spec.MinReplicas = ptr.To[int32](2)
		})

		// Case 5: Deployment(replicas=1) | Policy(Divided, two clusters 1:1) | HPA(minReplicas=2)
		// Expected result: it will go through such a process:
		//   1. deployment.spec.replicas=1, actual replicas in member1:member2 = 1:0
		//   2. hpa take effect in member1, so actual replicas in member1:member2 = 2:0
		//   3. deployment template updated to 2/2
		//   4. reschedule, assign replicas to member1:member2 = 1:1
		//   5. member1 replicas is retained, so actual replicas in member1:member2 = 2:1
		//   6. hpa take effect in member2, so replicas becomes member1:member2 = 2:2
		//   7. deployment template updated to 4/4
		ginkgo.It("propagate 1 replica but hpa minReplicas is 2", func() {
			assertDeploymentWorkloadReplicas(namespace, deploymentName, targetClusters, []int32{2, 2})
			assertDeploymentTemplateReplicas(namespace, deploymentName, 4)
		})
	})
})

// assertDeploymentWorkloadReplicas assert replicas in each member cluster eventually equal to @expectedReplicas
func assertDeploymentWorkloadReplicas(namespace, name string, clusters []string, expectedReplicas []int32) {
	gomega.Expect(len(clusters)).Should(gomega.Equal(len(expectedReplicas)))
	for i, cluster := range clusters {
		if expectedReplicas[i] == 0 {
			framework.WaitDeploymentDisappearOnCluster(cluster, namespace, name)
			return
		}
		framework.WaitDeploymentPresentOnClustersFitWith([]string{cluster}, namespace, name, func(deployment *appsv1.Deployment) bool {
			klog.Infof("in %s cluster, got: %d, expect: %d", cluster, *deployment.Spec.Replicas, expectedReplicas[i])
			return *deployment.Spec.Replicas == expectedReplicas[i]
		})
	}
}

// assertDeploymentTemplateReplicas assert replicas in template spec eventually equal to @expectedSpecReplicas
func assertDeploymentTemplateReplicas(namespace, name string, expectedSpecReplicas int32) {
	gomega.Eventually(func() bool {
		deploymentExist, err := kubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		klog.Infof("template spec replicas, got: %d, expect: %d", *deploymentExist.Spec.Replicas, expectedSpecReplicas)
		return (*deploymentExist.Spec.Replicas == expectedSpecReplicas) && (deploymentExist.Generation == deploymentExist.Status.ObservedGeneration)
	}, time.Minute, pollInterval).Should(gomega.Equal(true))
}
