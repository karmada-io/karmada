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
	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[Preemption] propagation policy preemption testing", func() {
	var preemptingClusterName, preemptedClusterName string

	ginkgo.BeforeEach(func() {
		preemptingClusterName = framework.ClusterNames()[1]
		preemptedClusterName = framework.ClusterNames()[0]
	})

	ginkgo.When("[PropagationPolicy Preemption] PropagationPolicy preempts another (Cluster)PropagationPolicy", func() {
		ginkgo.Context("High-priority PropagationPolicy preempts low-priority PropagationPolicy", func() {
			var highPriorityPolicy, lowPriorityPolicy *policyv1alpha1.PropagationPolicy
			var deployment *appsv1.Deployment
			ginkgo.BeforeEach(func() {
				deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
				highPriorityPolicy = testhelper.NewExplicitPriorityPropagationPolicy(deployment.Namespace, deployment.Name+"high-pp", []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Namespace:  deployment.Namespace,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{preemptingClusterName},
					}}, 10)
				// enable preemption.
				highPriorityPolicy.Spec.Preemption = policyv1alpha1.PreemptAlways

				lowPriorityPolicy = testhelper.NewExplicitPriorityPropagationPolicy(deployment.Namespace, deployment.Name+"low-pp", []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Namespace:  deployment.Namespace,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{preemptedClusterName},
					}}, 5)
			})

			ginkgo.BeforeEach(func() {
				framework.CreateDeployment(kubeClient, deployment)
				framework.CreatePropagationPolicy(karmadaClient, lowPriorityPolicy)
				ginkgo.DeferCleanup(func() {
					framework.RemovePropagationPolicy(karmadaClient, lowPriorityPolicy.Namespace, lowPriorityPolicy.Name)
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				})
			})

			ginkgo.It("Propagate the deployment with the low-priority PropagationPolicy and then create the high-priority PropagationPolicy to preempt it", func() {
				ginkgo.By("Wait for propagating deployment by the low-priority PropagationPolicy", func() {
					framework.WaitDeploymentPresentOnClusterFitWith(preemptedClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})

				ginkgo.By("Create the high-priority PropagationPolicy to preempt the low-priority PropagationPolicy", func() {
					framework.CreatePropagationPolicy(karmadaClient, highPriorityPolicy)
					framework.WaitDeploymentPresentOnClusterFitWith(preemptingClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})

				ginkgo.By("Delete the high-priority PropagationPolicy to let the low-priority PropagationPolicy preempt the deployment", func() {
					framework.RemovePropagationPolicy(karmadaClient, highPriorityPolicy.Namespace, highPriorityPolicy.Name)
					framework.WaitDeploymentPresentOnClusterFitWith(preemptedClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})
			})
		})

		ginkgo.Context("PropagationPolicy preempts ClusterPropagationPolicy", func() {
			var propagationPolicy *policyv1alpha1.PropagationPolicy
			var clusterPropagationPolicy *policyv1alpha1.ClusterPropagationPolicy
			var deployment *appsv1.Deployment
			ginkgo.BeforeEach(func() {
				deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
				propagationPolicy = testhelper.NewPropagationPolicy(deployment.Namespace, deployment.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Namespace:  deployment.Namespace,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{preemptingClusterName},
					}})
				// enable preemption.
				propagationPolicy.Spec.Preemption = policyv1alpha1.PreemptAlways

				clusterPropagationPolicy = testhelper.NewClusterPropagationPolicy(deployment.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Namespace:  deployment.Namespace,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{preemptedClusterName},
					}})
			})

			ginkgo.BeforeEach(func() {
				framework.CreateDeployment(kubeClient, deployment)
				framework.CreateClusterPropagationPolicy(karmadaClient, clusterPropagationPolicy)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
					framework.RemoveClusterPropagationPolicy(karmadaClient, clusterPropagationPolicy.Name)
				})
			})

			ginkgo.It("Propagate the deployment with the ClusterPropagationPolicy and then create the PropagationPolicy to preempt it", func() {
				ginkgo.By("Wait for propagating deployment by the ClusterPropagationPolicy", func() {
					framework.WaitDeploymentPresentOnClusterFitWith(preemptedClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})

				ginkgo.By("Create the PropagationPolicy to preempt the ClusterPropagationPolicy", func() {
					framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
					framework.WaitDeploymentPresentOnClusterFitWith(preemptingClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})

				ginkgo.By("Delete the PropagationPolicy to let the ClusterPropagationPolicy preempt the deployment", func() {
					framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
					framework.WaitDeploymentPresentOnClusterFitWith(preemptedClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})
			})
		})

		ginkgo.Context("High-priority PropagationPolicy reduces priority to be preempted by low-priority PropagationPolicy", func() {
			var highPriorityPolicy, lowPriorityPolicy *policyv1alpha1.PropagationPolicy
			var deployment *appsv1.Deployment
			ginkgo.BeforeEach(func() {
				deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
				highPriorityPolicy = testhelper.NewExplicitPriorityPropagationPolicy(deployment.Namespace, deployment.Name+"high-pp", []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Namespace:  deployment.Namespace,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{preemptedClusterName},
					}}, 10)
				// enable preemption.
				highPriorityPolicy.Spec.Preemption = policyv1alpha1.PreemptAlways

				lowPriorityPolicy = testhelper.NewExplicitPriorityPropagationPolicy(deployment.Namespace, deployment.Name+"low-pp", []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Namespace:  deployment.Namespace,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{preemptingClusterName},
					}}, 5)
				// enable preemption.
				lowPriorityPolicy.Spec.Preemption = policyv1alpha1.PreemptAlways
			})

			ginkgo.BeforeEach(func() {
				framework.CreateDeployment(kubeClient, deployment)
				framework.CreatePropagationPolicy(karmadaClient, highPriorityPolicy)
				framework.CreatePropagationPolicy(karmadaClient, lowPriorityPolicy)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
					framework.RemovePropagationPolicy(karmadaClient, lowPriorityPolicy.Namespace, lowPriorityPolicy.Name)
					framework.RemovePropagationPolicy(karmadaClient, highPriorityPolicy.Namespace, highPriorityPolicy.Name)
				})
			})

			ginkgo.It("Propagate the deployment with the high-priority PropagationPolicy and then reduce it's priority to be preempted by the low-priority PropagationPolicy", func() {
				ginkgo.By("Wait for propagating deployment by the high-priority PropagationPolicy", func() {
					framework.WaitDeploymentPresentOnClusterFitWith(preemptedClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})

				ginkgo.By("Reduce the priority of the high-priority PropagationPolicy to be preempted by the low-priority PropagationPolicy", func() {
					highPriorityPolicy.Spec.Priority = ptr.To[int32](4)
					patch := []map[string]interface{}{
						{
							"path":  "/spec/priority",
							"op":    "replace",
							"value": 4,
						},
					}
					framework.PatchPropagationPolicy(karmadaClient, highPriorityPolicy.Namespace, highPriorityPolicy.Name, patch, types.JSONPatchType)
					framework.WaitDeploymentPresentOnClusterFitWith(preemptingClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})
			})
		})
	})

	ginkgo.When("[ClusterPropagationPolicy Preemption] ClusterPropagationPolicy preempts another ClusterPropagationPolicy", func() {
		ginkgo.Context("High-priority ClusterPropagationPolicy preempts low-priority ClusterPropagationPolicy", func() {
			var highPriorityPolicy, lowPriorityPolicy *policyv1alpha1.ClusterPropagationPolicy
			var deployment *appsv1.Deployment
			ginkgo.BeforeEach(func() {
				deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
				highPriorityPolicy = testhelper.NewExplicitPriorityClusterPropagationPolicy(deployment.Name+"high-cpp", []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Namespace:  deployment.Namespace,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{preemptingClusterName},
					}}, 10)
				// enable preemption.
				highPriorityPolicy.Spec.Preemption = policyv1alpha1.PreemptAlways

				lowPriorityPolicy = testhelper.NewExplicitPriorityClusterPropagationPolicy(deployment.Name+"low-cpp", []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Namespace:  deployment.Namespace,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{preemptedClusterName},
					}}, 5)
			})

			ginkgo.BeforeEach(func() {
				framework.CreateDeployment(kubeClient, deployment)
				framework.CreateClusterPropagationPolicy(karmadaClient, lowPriorityPolicy)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
					framework.RemoveClusterPropagationPolicy(karmadaClient, lowPriorityPolicy.Name)
				})
			})

			ginkgo.It("Propagate the deployment with the low-priority ClusterPropagationPolicy and then create the high-priority ClusterPropagationPolicy to preempt it", func() {
				ginkgo.By("Wait for propagating deployment by the low-priority ClusterPropagationPolicy", func() {
					framework.WaitDeploymentPresentOnClusterFitWith(preemptedClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})

				ginkgo.By("Create the high-priority ClusterPropagationPolicy to preempt the low-priority ClusterPropagationPolicy", func() {
					framework.CreateClusterPropagationPolicy(karmadaClient, highPriorityPolicy)
					framework.WaitDeploymentPresentOnClusterFitWith(preemptingClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})

				ginkgo.By("Delete the high-priority ClusterPropagationPolicy to let the low-priority ClusterPropagationPolicy preempt the deployment", func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, highPriorityPolicy.Name)
					framework.WaitDeploymentPresentOnClusterFitWith(preemptedClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})
			})
		})

		ginkgo.Context("High-priority ClusterPropagationPolicy reduces priority to be preempted by low-priority ClusterPropagationPolicy", func() {
			var highPriorityPolicy, lowPriorityPolicy *policyv1alpha1.ClusterPropagationPolicy
			var deployment *appsv1.Deployment
			ginkgo.BeforeEach(func() {
				deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
				highPriorityPolicy = testhelper.NewExplicitPriorityClusterPropagationPolicy(deployment.Name+"high-cpp", []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Namespace:  deployment.Namespace,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{preemptedClusterName},
					}}, 10)
				// enable preemption.
				highPriorityPolicy.Spec.Preemption = policyv1alpha1.PreemptAlways

				lowPriorityPolicy = testhelper.NewExplicitPriorityClusterPropagationPolicy(deployment.Name+"low-cpp", []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Namespace:  deployment.Namespace,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{preemptingClusterName},
					}}, 5)
				// enable preemption.
				lowPriorityPolicy.Spec.Preemption = policyv1alpha1.PreemptAlways
			})

			ginkgo.BeforeEach(func() {
				framework.CreateDeployment(kubeClient, deployment)
				framework.CreateClusterPropagationPolicy(karmadaClient, highPriorityPolicy)
				framework.CreateClusterPropagationPolicy(karmadaClient, lowPriorityPolicy)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
					framework.RemoveClusterPropagationPolicy(karmadaClient, lowPriorityPolicy.Name)
					framework.RemoveClusterPropagationPolicy(karmadaClient, highPriorityPolicy.Name)
				})
			})

			ginkgo.It("Propagate the deployment with the high-priority ClusterPropagationPolicy and then reduce it's priority to be preempted by the low-priority ClusterPropagationPolicy", func() {
				ginkgo.By("Wait for propagating deployment by the high-priority ClusterPropagationPolicy", func() {
					framework.WaitDeploymentPresentOnClusterFitWith(preemptedClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})

				ginkgo.By("Reduce the priority of the high-priority ClusterPropagationPolicy to be preempted by the low-priority ClusterPropagationPolicy", func() {
					highPriorityPolicy.Spec.Priority = ptr.To[int32](4)
					patch := []map[string]interface{}{
						{
							"path":  "/spec/priority",
							"op":    "replace",
							"value": 4,
						},
					}
					framework.PatchClusterPropagationPolicy(karmadaClient, highPriorityPolicy.Name, patch, types.JSONPatchType)
					framework.WaitDeploymentPresentOnClusterFitWith(preemptingClusterName, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
				})
			})
		})
	})
})
