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

package base

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

const waitIntervalForLazyPolicyTest = 3 * time.Second

// e2e test for https://github.com/karmada-io/karmada/blob/release-1.9/docs/proposals/scheduling/activation-preference/lazy-activation-preference.md#test-plan
var _ = ginkgo.Describe("Lazy activation policy testing", func() {
	var namespace string
	var deploymentName, configMapName, policyName, policyHigherPriorityName string
	var originalCluster, modifiedCluster string
	var deployment *appsv1.Deployment
	var configMap *corev1.ConfigMap
	var policy, policyHigherPriority *policyv1alpha1.PropagationPolicy

	ginkgo.BeforeEach(func() {
		namespace = testNamespace
		deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
		configMapName = deploymentName
		policyName = deploymentName
		policyHigherPriorityName = deploymentName + "higherpriority"
		originalCluster = framework.ClusterNames()[0]
		modifiedCluster = framework.ClusterNames()[1]

		deployment = testhelper.NewDeployment(namespace, deploymentName)
		configMap = testhelper.NewConfigMap(namespace, configMapName, map[string]string{"test": "test"})
		policy = testhelper.NewLazyPropagationPolicy(namespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deploymentName,
			}}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{originalCluster},
			},
		})
		policyHigherPriority = testhelper.NewLazyPropagationPolicy(namespace, policyHigherPriorityName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deploymentName,
			}}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{modifiedCluster},
			},
		})
		policyHigherPriority.Spec.Priority = ptr.To[int32](2)
		policyHigherPriority.Spec.Preemption = policyv1alpha1.PreemptAlways
	})

	ginkgo.Context("1. Policy created before resource", func() {
		ginkgo.JustBeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			waitPropagatePolicyReconciled(namespace, policyName)
			framework.CreateDeployment(kubeClient, deployment)

			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicyIfExist(karmadaClient, namespace, policyName)
				framework.RemoveDeployment(kubeClient, namespace, deploymentName)
				framework.WaitDeploymentDisappearOnCluster(originalCluster, namespace, deploymentName)
				framework.WaitDeploymentDisappearOnCluster(modifiedCluster, namespace, deploymentName)
			})
		})

		// Simple Case 1 (Policy created before resource)
		// refer: https://github.com/karmada-io/karmada/blob/release-1.9/docs/proposals/scheduling/activation-preference/lazy-activation-preference.md#simple-case-1-policy-created-before-resource
		ginkgo.It("Simple Case 1 (Policy created before resource)", func() {
			ginkgo.By("step 1: deployment propagate success when policy created before it", func() {
				waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
			})

			ginkgo.By("step 2: after policy deleted, deployment still keep previous propagation states", func() {
				framework.RemovePropagationPolicy(karmadaClient, namespace, policyName)
				// wait to distinguish whether the state will not change or have no time to change
				time.Sleep(waitIntervalForLazyPolicyTest)
				waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
			})
		})

		// Simple Case 3 (Lazy to immediate)
		// refer: https://github.com/karmada-io/karmada/blob/release-1.9/docs/proposals/scheduling/activation-preference/lazy-activation-preference.md#simple-case-3-lazy-to-immediate
		ginkgo.It("Simple Case 3 (Lazy to immediate)", func() {
			ginkgo.By("step 1: deployment propagate success when policy created before it", func() {
				waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
			})

			ginkgo.By(fmt.Sprintf("step 2: after policy updated (cluster=%s, remove lazy activationPreference field), the propagation of deployment changed", modifiedCluster), func() {
				// 1. remove lazy activationPreference field
				policy.Spec.ActivationPreference = ""
				// 2. update policy placement with clusterAffinities
				changePlacementTargetCluster(policy, modifiedCluster)
				// 3. the propagation of deployment changed
				framework.WaitDeploymentDisappearOnCluster(originalCluster, namespace, deploymentName)
				waitDeploymentPresentOnCluster(modifiedCluster, namespace, deploymentName)
			})
		})

		ginkgo.Context("Immediate to lazy", func() {
			ginkgo.BeforeEach(func() {
				// remove lazy activationPreference field
				policy.Spec.ActivationPreference = ""
			})

			// Simple Case 4 (Immediate to lazy)
			// refer: https://github.com/karmada-io/karmada/blob/release-1.9/docs/proposals/scheduling/activation-preference/lazy-activation-preference.md#simple-case-4-immediate-to-lazy
			ginkgo.It("Simple Case 4 (Immediate to lazy)", func() {
				ginkgo.By("step 1: deployment propagate success", func() {
					waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
				})

				ginkgo.By(fmt.Sprintf("step 2: after policy updated (cluster=%s, activationPreference=lazy), the propagation of deployment unchanged", modifiedCluster), func() {
					// 1. activationPreference=lazy
					policy.Spec.ActivationPreference = policyv1alpha1.LazyActivation
					// 2. update policy placement with clusterAffinities
					changePlacementTargetCluster(policy, modifiedCluster)
					// 3. wait to distinguish whether the state will not change or have no time to change
					time.Sleep(waitIntervalForLazyPolicyTest)
					waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
					framework.WaitDeploymentDisappearOnCluster(modifiedCluster, namespace, deploymentName)
				})

				ginkgo.By("step3: resource would propagate when itself updated", func() {
					// 1. update annotation of the deployment
					updateDeploymentManually(deployment)
					// 2. the propagation of deployment changed
					framework.WaitDeploymentDisappearOnCluster(originalCluster, namespace, deploymentName)
					waitDeploymentPresentOnCluster(modifiedCluster, namespace, deploymentName)
				})
			})
		})

		// Combined Case 3 (Policy preemption)
		// refer: https://github.com/karmada-io/karmada/blob/release-1.9/docs/proposals/scheduling/activation-preference/lazy-activation-preference.md#combined-case-3-policy-preemption

		ginkgo.It("Combined Case 3 (Policy preemption)", func() {
			ginkgo.By("step 1: deployment propagate success when policy created before it", func() {
				waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
			})

			ginkgo.By(fmt.Sprintf("step 2: create PP2 (match nginx, cluster=%s, not lazy, priority=2, preemption=true)", modifiedCluster), func() {
				policyHigherPriority.Spec.ActivationPreference = "" // remove lazy activationPreference field
				framework.CreatePropagationPolicy(karmadaClient, policyHigherPriority)
				waitDeploymentPresentOnCluster(modifiedCluster, namespace, deploymentName)
			})

			ginkgo.By("step 3: clean up", func() {
				framework.RemovePropagationPolicyIfExist(karmadaClient, namespace, policyHigherPriorityName)
			})
		})

		// Combined Case 4 (Policy preemption)
		// refer: https://github.com/karmada-io/karmada/blob/release-1.9/docs/proposals/scheduling/activation-preference/lazy-activation-preference.md#combined-case-4-policy-preemption
		ginkgo.It("Policy preemption", func() {
			ginkgo.By("step 1: deployment propagate success when policy created before it", func() {
				waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
			})

			ginkgo.By(fmt.Sprintf("step 2: create PP2 (match nginx, cluster=%s, lazy, priority=2, preemption=true)", modifiedCluster), func() {
				framework.CreatePropagationPolicy(karmadaClient, policyHigherPriority)
				// 1. annotation of policy name changed
				framework.WaitDeploymentFitWith(kubeClient, namespace, deploymentName, func(deployment *appsv1.Deployment) bool {
					policyNameAnnotation := util.GetAnnotationValue(deployment.GetAnnotations(), policyv1alpha1.PropagationPolicyNameAnnotation)
					return policyNameAnnotation == policyHigherPriorityName
				})
				// 2. wait to distinguish whether the state will not change or have no time to change
				time.Sleep(waitIntervalForLazyPolicyTest)
				// propagation unchanged
				waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
				framework.WaitDeploymentDisappearOnCluster(modifiedCluster, namespace, deploymentName)
			})

			ginkgo.By("step 3: update deployment", func() {
				// 1. update annotation of the deployment
				updateDeploymentManually(deployment)
				// 2. the propagation of deployment changed
				waitDeploymentPresentOnCluster(modifiedCluster, namespace, deploymentName)
				framework.WaitDeploymentDisappearOnCluster(originalCluster, namespace, deploymentName)
			})

			ginkgo.By("step 4: clean up", func() {
				framework.RemovePropagationPolicyIfExist(karmadaClient, namespace, policyHigherPriorityName)
			})
		})

		ginkgo.Context("Propagate dependencies", func() {
			ginkgo.BeforeEach(func() {
				policy.Spec.PropagateDeps = true
				mountConfigMapToDeployment(deployment, configMapName)
			})

			ginkgo.JustBeforeEach(func() {
				framework.CreateConfigMap(kubeClient, configMap)
				ginkgo.DeferCleanup(func() {
					framework.RemoveConfigMap(kubeClient, namespace, configMapName)
				})
			})

			// Combined Case 5 (Propagate dependencies)
			// refer: https://github.com/karmada-io/karmada/blob/release-1.9/docs/proposals/scheduling/activation-preference/lazy-activation-preference.md#combined-case-5-propagate-dependencies
			ginkgo.It("Combined Case 5 (Propagate dependencies)", func() {
				ginkgo.By("step 1: deployment and its dependencies could propagate success.", func() {
					waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
					waitConfigMapPresentOnCluster(originalCluster, namespace, deploymentName)
				})

				ginkgo.By("step 2: change of lazy policy will not take effect", func() {
					changePlacementTargetCluster(policy, modifiedCluster)
					// wait to distinguish whether the policy will not take effect or have no time to take effect
					time.Sleep(waitIntervalForLazyPolicyTest)
					waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
					waitConfigMapPresentOnCluster(originalCluster, namespace, deploymentName)
				})

				ginkgo.By("step 3: lazy policy take effect when deployment updated, dependencies can also been propagated.", func() {
					updateDeploymentManually(deployment)
					waitDeploymentPresentOnCluster(modifiedCluster, namespace, deploymentName)
					waitConfigMapPresentOnCluster(modifiedCluster, namespace, deploymentName)
				})
			})
		})
	})

	ginkgo.Context("2. Policy created after resource", func() {
		ginkgo.JustBeforeEach(func() {
			framework.CreateDeployment(kubeClient, deployment)
			waitDeploymentReconciled(namespace, deploymentName)
			framework.CreatePropagationPolicy(karmadaClient, policy)

			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, namespace, policyName)
				framework.RemoveDeployment(kubeClient, namespace, deploymentName)
				framework.WaitDeploymentDisappearOnCluster(originalCluster, namespace, deploymentName)
			})
		})

		// Simple Case 2 (Policy created after resource)
		// refer: https://github.com/karmada-io/karmada/blob/release-1.9/docs/proposals/scheduling/activation-preference/lazy-activation-preference.md#simple-case-2-policy-created-after-resource
		ginkgo.It("Simple Case 2 (Policy created after resource)", func() {
			ginkgo.By("step1: deployment would not propagate when lazy policy created after deployment", func() {
				// wait to distinguish whether the deployment will not propagate or have no time to propagate
				time.Sleep(waitIntervalForLazyPolicyTest)
				framework.WaitDeploymentDisappearOnCluster(originalCluster, namespace, deploymentName)
			})

			ginkgo.By("step2: resource would propagate when itself updated", func() {
				updateDeploymentManually(deployment)
				waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
			})
		})

		ginkgo.Context("Propagate dependencies", func() {
			ginkgo.BeforeEach(func() {
				mountConfigMapToDeployment(deployment, configMapName)
				framework.CreateConfigMap(kubeClient, configMap)
				ginkgo.DeferCleanup(func() {
					framework.RemoveConfigMap(kubeClient, namespace, configMapName)
				})
			})

			// Combined Case 6 (Propagate dependencies)
			// refer: https://github.com/karmada-io/karmada/blob/release-1.9/docs/proposals/scheduling/activation-preference/lazy-activation-preference.md#combined-case-6-propagate-dependencies
			ginkgo.It("Combined Case 6 (Propagate dependencies)", func() {
				ginkgo.By("step 1: resources would not propagate when lazy policy created after resources", func() {
					// wait to distinguish whether the resource will not propagate or have no time to propagate
					time.Sleep(waitIntervalForLazyPolicyTest)
					framework.WaitDeploymentDisappearOnCluster(originalCluster, namespace, deploymentName)
					framework.WaitConfigMapDisappearOnCluster(originalCluster, namespace, configMapName)
				})

				ginkgo.By("step 2: configMap not propagate with deployment since policy PropagateDeps unset", func() {
					updateDeploymentManually(deployment)
					waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
					framework.WaitConfigMapDisappearOnCluster(originalCluster, namespace, configMapName)
				})

				ginkgo.By("step 3: set PropagateDeps of a lazy policy would not take effect immediately", func() {
					setPolicyPropagateDeps(policy)
					// wait to distinguish whether the policy will not take effect or have no time to take effect
					time.Sleep(waitIntervalForLazyPolicyTest)
					waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
					framework.WaitConfigMapDisappearOnCluster(originalCluster, namespace, configMapName)
				})

				ginkgo.By("step 4: set PropagateDeps of a lazy policy take effect when deployment itself updated", func() {
					updateDeploymentManually(deployment)
					waitDeploymentPresentOnCluster(originalCluster, namespace, deploymentName)
					waitConfigMapPresentOnCluster(originalCluster, namespace, deploymentName)
				})
			})
		})
	})
})

// updateDeploymentManually manually update deployment
func updateDeploymentManually(deployment *appsv1.Deployment) {
	framework.AppendDeploymentAnnotations(kubeClient, deployment, map[string]string{"reconcileAt": time.Now().Format(time.RFC3339)})
}

// waitDeploymentPresentOnCluster wait deployment present on cluster
func waitDeploymentPresentOnCluster(cluster, namespace, name string) {
	framework.WaitDeploymentPresentOnClusterFitWith(cluster, namespace, name, func(_ *appsv1.Deployment) bool {
		return true
	})
}

// waitDeploymentReconciled wait reconciliation of deployment finished
func waitDeploymentReconciled(namespace, name string) {
	framework.WaitDeploymentFitWith(kubeClient, namespace, name, func(_ *appsv1.Deployment) bool {
		// when applying deployment and policy sequentially, we expect deployment to perform the reconcile process before policy,
		// but the order is actually uncertain, so we sleep a while to wait reconciliation of deployment finished.
		time.Sleep(waitIntervalForLazyPolicyTest)
		return true
	})
}

// waitPropagatePolicyReconciled wait reconciliation of PropagatePolicy finished
func waitPropagatePolicyReconciled(namespace, name string) {
	framework.WaitPropagationPolicyFitWith(karmadaClient, namespace, name, func(_ *policyv1alpha1.PropagationPolicy) bool {
		// when applying policy and deployment sequentially, we expect policy to perform the reconcile process before deployment,
		// but the order is actually uncertain, so we sleep a while to wait reconciliation of policy finished.
		time.Sleep(waitIntervalForLazyPolicyTest)
		return true
	})
}

// waitConfigMapPresentOnCluster wait configmap present on cluster
func waitConfigMapPresentOnCluster(cluster, namespace, name string) {
	framework.WaitConfigMapPresentOnClusterFitWith(cluster, namespace, name, func(_ *corev1.ConfigMap) bool {
		return true
	})
}

// mountConfigMapToDeployment mount ConfigMap to Deployment
func mountConfigMapToDeployment(deployment *appsv1.Deployment, configMapName string) {
	volumes := []corev1.Volume{{
		Name: "vol-configmap",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				}}}}}
	deployment.Spec.Template.Spec.Volumes = volumes
}

// changePlacementTargetCluster change policy target cluster to @modifiedCluster
func changePlacementTargetCluster(policy *policyv1alpha1.PropagationPolicy, modifiedCluster string) {
	policySpec := policy.Spec
	policySpec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{ClusterNames: []string{modifiedCluster}}
	framework.UpdatePropagationPolicyWithSpec(karmadaClient, policy.Namespace, policy.Name, policySpec)
}

// setPolicyPropagateDeps set PropagateDeps of policy to true
func setPolicyPropagateDeps(policy *policyv1alpha1.PropagationPolicy) {
	policySpec := policy.Spec
	policySpec.PropagateDeps = true
	framework.UpdatePropagationPolicyWithSpec(karmadaClient, policy.Namespace, policy.Name, policySpec)
}
