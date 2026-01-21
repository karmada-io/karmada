/*
Copyright 2026 The Karmada Authors.

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
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Quota plugin Testing", func() {
	var resourceQuota corev1.ResourceQuota
	var deployment *appsv1.Deployment
	var policy *policyv1alpha1.PropagationPolicy
	var rqNamespace, rqName string
	var deployNamespace, deployName string
	var policyNamespace, policyName string
	var targetCluster string

	var err error

	ginkgo.BeforeEach(func() {
		targetCluster = framework.ClusterNames()[0]

		ginkgo.By("set up namespace", func() {
			// To avoid conflicts with other test cases, use random strings to generate unique namespaces instead of using testNamespace.
			deployNamespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			err = setupTestNamespace(deployNamespace, kubeClient)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			ginkgo.DeferCleanup(func() {
				framework.RemoveNamespace(kubeClient, deployNamespace)
			})
		})

		deployName = deploymentNamePrefix + rand.String(RandomStrLength)
		deployment = helper.NewDeployment(deployNamespace, deployName)

		ginkgo.By("create resourceQuota", func() {
			rqNamespace = deployNamespace
			rqName = resourceQuotaPrefix + rand.String(RandomStrLength)
			resourceQuota = corev1.ResourceQuota{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ResourceQuota",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      rqName,
					Namespace: rqNamespace,
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						"requests.cpu": resource.MustParse("0.03"), // Equals to the resource requested by the deployment created by helper.NewDeployment: 3 replicas × 10 milli CPU
					},
				},
			}
			framework.CreateResourceQuota(kubeClient, &resourceQuota)
			ginkgo.DeferCleanup(func() {
				framework.RemoveResourceQuota(kubeClient, rqNamespace, rqName)
			})
		})

		ginkgo.By("create propagation policy", func() {
			policyNamespace = deployNamespace
			policyName = ppNamePrefix + rand.String(RandomStrLength)
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
				{
					APIVersion: resourceQuota.APIVersion,
					Kind:       resourceQuota.Kind,
					Name:       resourceQuota.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetCluster},
				},
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						DynamicWeight: policyv1alpha1.DynamicWeightByAvailableReplicas,
					},
				},
			})
			framework.CreatePropagationPolicy(karmadaClient, policy)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, policyNamespace, policyName)
			})
		})

		// To ensure that the resource quota is created on the target cluster before creating the deployment.
		framework.WaitResourceQuotaPresentOnCluster(targetCluster, rqNamespace, rqName)
	})

	ginkgo.It("Deployment should be successfully propagated to target cluster within resource quota limits", func() {
		ginkgo.By("Creating deployment within resource quota limits", func() {
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployNamespace, deployName)
			})
		})

		ginkgo.By("first verifying resource binding scheduling", func() {
			deployBindingName := names.GenerateBindingName(util.DeploymentKind, deployName)
			framework.AssertBindingScheduledClusters(karmadaClient, deployNamespace, deployBindingName, [][]string{{targetCluster}})
		})

		ginkgo.By("Verifying deployment propagation to target cluster", func() {
			framework.WaitDeploymentPresentOnClusterFitWith(targetCluster, deployNamespace, deployName, func(deploy *appsv1.Deployment) bool {
				return framework.CheckDeploymentReadyStatus(deploy, *deployment.Spec.Replicas)
			})
		})
	})

	ginkgo.It("Deployment should not be propagated to target cluster exceeding resource quota limits", func() {
		ginkgo.By("Creating deployment exceeding resource quota limits", func() {
			deployment.Spec.Replicas = ptr.To[int32](5) // This will request 0.05 CPU which exceeds the quota of 0.03 CPU
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployNamespace, deployName)
			})
		})

		ginkgo.By("first verifying resource binding scheduling", func() {
			deployBindingName := names.GenerateBindingName(util.DeploymentKind, deployName)
			framework.WaitResourceBindingFitWith(karmadaClient, deployNamespace, deployBindingName, func(binding *workv1alpha2.ResourceBinding) bool {
				cond := meta.FindStatusCondition(binding.Status.Conditions, workv1alpha2.Scheduled)
				return binding.Spec.Clusters == nil && cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == workv1alpha2.BindingReasonSchedulerError
			})
		})

		ginkgo.By("Verifying deployment is not propagated to target cluster", func() {
			framework.WaitDeploymentDisappearOnCluster(targetCluster, deployNamespace, deployName)
		})
	})
})
