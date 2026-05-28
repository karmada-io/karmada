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
	"context"
	"fmt"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

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
			framework.WaitResourceBindingFitWith(karmadaClient, deployNamespace, deployBindingName, func(binding *workv1alpha2.ResourceBinding) bool {
				if binding.Spec.ReplicaRequirements == nil {
					return false
				}
				return binding.Spec.ReplicaRequirements.Namespace == deployNamespace
			})
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
				return binding.Spec.Clusters == nil && cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == workv1alpha2.BindingReasonUnschedulable
			})
			framework.WaitResourceBindingFitWith(karmadaClient, deployNamespace, deployBindingName, func(binding *workv1alpha2.ResourceBinding) bool {
				if binding.Spec.ReplicaRequirements == nil {
					return false
				}
				return binding.Spec.ReplicaRequirements.Namespace == deployNamespace
			})
		})

		ginkgo.By("Verifying deployment is not propagated to target cluster", func() {
			framework.WaitDeploymentDisappearOnCluster(targetCluster, deployNamespace, deployName)
		})
	})
})

// flinkDeploymentGVR is the GroupVersionResource for FlinkDeployment.
var flinkDeploymentGVR = schema.GroupVersionResource{
	Group:    "flink.apache.org",
	Version:  "v1beta1",
	Resource: "flinkdeployments",
}

var _ = framework.SerialDescribe("[EstimatorAssumption] ResourceQuota plugin assumption testing", func() {
	const targetCluster = "member1"

	var flinkCRD apiextensionsv1.CustomResourceDefinition
	var quotaNamespace, rqName string

	ginkgo.BeforeEach(func() {
		// Use a dedicated namespace so the ResourceQuota only constrains this test's workloads.
		quotaNamespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
		err := setupTestNamespace(quotaNamespace, kubeClient)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		ginkgo.DeferCleanup(func() {
			framework.RemoveNamespace(kubeClient, quotaNamespace)
		})
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By("creating FlinkDeployment CRD on karmada control plane", func() {
			err := yaml.Unmarshal([]byte(flinkDeploymentCRDYAML), &flinkCRD)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			framework.CreateCRD(dynamicClient, &flinkCRD)
			framework.WaitCRDEstablished(dynamicClient, flinkCRD.Name)
			ginkgo.DeferCleanup(func() {
				framework.RemoveCRD(dynamicClient, flinkCRD.Name)
				framework.WaitCRDDisappeared(dynamicClient, flinkCRD.Name)
			})
		})

		ginkgo.By("propagating FlinkDeployment CRD to member1", func() {
			cpp := helper.NewClusterPropagationPolicy(cppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: flinkCRD.APIVersion,
					Kind:       flinkCRD.Kind,
					Name:       flinkCRD.Name,
				}},
				policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetCluster},
					},
				})
			framework.CreateClusterPropagationPolicy(karmadaClient, cpp)
			framework.WaitCRDPresentOnClusters(karmadaClient, []string{targetCluster},
				fmt.Sprintf("%s/%s", flinkCRD.Spec.Group, "v1beta1"), flinkCRD.Spec.Names.Kind)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, cpp.Name)
			})
		})
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By("creating ResourceQuota with cpu=1 and propagating to member1", func() {
			rqName = resourceQuotaPrefix + rand.String(RandomStrLength)
			rq := &corev1.ResourceQuota{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ResourceQuota",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      rqName,
					Namespace: quotaNamespace,
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
			}
			framework.CreateResourceQuota(kubeClient, rq)
			ginkgo.DeferCleanup(func() {
				framework.RemoveResourceQuota(kubeClient, quotaNamespace, rqName)
			})

			pp := helper.NewPropagationPolicy(quotaNamespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: rq.TypeMeta.APIVersion,
					Kind:       rq.TypeMeta.Kind,
					Name:       rqName,
				}},
				policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetCluster},
					},
				})
			framework.CreatePropagationPolicy(karmadaClient, pp)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, quotaNamespace, pp.Name)
			})

			// Ensure the quota exists on member1 before creating any FlinkDeployments.
			framework.WaitResourceQuotaPresentOnCluster(targetCluster, quotaNamespace, rqName)
		})
	})

	ginkgo.It("FlinkDeployment should be unschedulable when assumed workloads exhaust ResourceQuota", func(ctx context.Context) {
		// Each FlinkDeployment uses jobManager(50m) + taskManager(100m) = 150m CPU (from manifest).
		// With ResourceQuota of 1000m and no real pods running (ResourceQuota.Status.Used stays at 0),
		// the resourcequota estimator deducts in-flight assumed workloads:
		//   - FlinkDeployments 1-6: 6 × 150m = 900m assumed → each is schedulable.
		//   - FlinkDeployment 7:  900m + 150m = 1050m > 1000m → estimator returns 0 → unschedulable.
		const schedulableCount = 6

		// createFlinkDeployment creates a FlinkDeployment in quotaNamespace with a PropagationPolicy
		// targeting member1 using the cpu values already set in flinkDeploymentCRYAML,
		// and returns the corresponding ResourceBinding name.
		createFlinkDeployment := func() string {
			flinkName := fmt.Sprintf("flinkdeployment-%s", rand.String(RandomStrLength))

			flinkObj := &unstructured.Unstructured{}
			err := yaml.Unmarshal([]byte(flinkDeploymentCRYAML), flinkObj)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			flinkObj.SetNamespace(quotaNamespace)
			flinkObj.SetName(flinkName)
			_, err = dynamicClient.Resource(flinkDeploymentGVR).Namespace(quotaNamespace).
				Create(ctx, flinkObj, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			ginkgo.DeferCleanup(func() {
				_ = dynamicClient.Resource(flinkDeploymentGVR).Namespace(quotaNamespace).
					Delete(ctx, flinkName, metav1.DeleteOptions{})
			})

			pp := helper.NewPropagationPolicy(quotaNamespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: "flink.apache.org/v1beta1",
					Kind:       "FlinkDeployment",
					Name:       flinkName,
				}},
				policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetCluster},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
					},
				})
			framework.CreatePropagationPolicy(karmadaClient, pp)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, quotaNamespace, pp.Name)
			})

			return names.GenerateBindingName("FlinkDeployment", flinkName)
		}

		ginkgo.By(fmt.Sprintf("creating %d FlinkDeployments that fit within the ResourceQuota", schedulableCount), func() {
			for range schedulableCount {
				bindingName := createFlinkDeployment()
				// Wait for successful scheduling before creating the next one so that each
				// workload is recorded as an assumed workload before the quota is re-evaluated.
				framework.WaitResourceBindingFitWith(karmadaClient, quotaNamespace, bindingName,
					func(binding *workv1alpha2.ResourceBinding) bool {
						cond := meta.FindStatusCondition(binding.Status.Conditions, workv1alpha2.Scheduled)
						return cond != nil && cond.Status == metav1.ConditionTrue
					})
			}
		})

		ginkgo.By("verifying the 7th FlinkDeployment is unschedulable because ResourceQuota is exhausted by assumed workloads", func() {
			bindingName := createFlinkDeployment()
			framework.WaitResourceBindingFitWith(karmadaClient, quotaNamespace, bindingName,
				func(binding *workv1alpha2.ResourceBinding) bool {
					cond := meta.FindStatusCondition(binding.Status.Conditions, workv1alpha2.Scheduled)
					return cond != nil && cond.Status == metav1.ConditionFalse &&
						cond.Reason == workv1alpha2.BindingReasonSchedulerError &&
						strings.Contains(cond.Message, "no enough resource")
				})
		})
	})
})

var _ = framework.SerialDescribe("[EstimatorAssumption] NodeResource plugin assumption testing", func() {
	const targetCluster = "member1"

	var flinkCRD apiextensionsv1.CustomResourceDefinition

	ginkgo.BeforeEach(func() {
		ginkgo.By("creating FlinkDeployment CRD on karmada control plane", func() {
			err := yaml.Unmarshal([]byte(flinkDeploymentCRDYAML), &flinkCRD)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			framework.CreateCRD(dynamicClient, &flinkCRD)
			framework.WaitCRDEstablished(dynamicClient, flinkCRD.Name)
			ginkgo.DeferCleanup(func() {
				framework.RemoveCRD(dynamicClient, flinkCRD.Name)
				framework.WaitCRDDisappeared(dynamicClient, flinkCRD.Name)
			})
		})
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By("propagating FlinkDeployment CRD to member1", func() {
			cpp := helper.NewClusterPropagationPolicy(cppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: flinkCRD.APIVersion,
					Kind:       flinkCRD.Kind,
					Name:       flinkCRD.Name,
				}},
				policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetCluster},
					},
				})
			framework.CreateClusterPropagationPolicy(karmadaClient, cpp)
			framework.WaitCRDPresentOnClusters(karmadaClient, []string{targetCluster},
				fmt.Sprintf("%s/%s", flinkCRD.Spec.Group, "v1beta1"), flinkCRD.Spec.Names.Kind)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, cpp.Name)
			})
		})
	})

	ginkgo.It("FlinkDeployment should be unschedulable when assumed workloads exhaust cluster resources", func(ctx context.Context) {
		// Each FlinkDeployment has jobManager (100m) + taskManager (100m) = 200m total assumed CPU.
		// We create FlinkDeployments sequentially (each scheduled before the next is created) so the
		// assumption cache accumulates in-flight CPU on member1. Since no Flink Operator is installed,
		// no pods are created and no real CPU is consumed, but karmada-scheduler treats these as
		// assumed workloads. Within maxFlinkCount attempts, at least one must get NoClusterFit,
		// which proves the assumption feature is working correctly.
		const (
			componentCPU  = 0.1 // 0.1 core (100m) as a number, matching the CRD schema type
			maxFlinkCount = 50
		)

		// createFlinkDeployment creates a FlinkDeployment with fixed 100m per component and its
		// PropagationPolicy targeting member1, then returns the ResourceBinding name.
		createFlinkDeployment := func() string {
			flinkName := fmt.Sprintf("flinkdeployment-%s", rand.String(RandomStrLength))

			flinkObj := &unstructured.Unstructured{}
			err := yaml.Unmarshal([]byte(flinkDeploymentCRYAML), flinkObj)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			flinkObj.SetNamespace(testNamespace)
			flinkObj.SetName(flinkName)
			err = unstructured.SetNestedField(flinkObj.Object, float64(componentCPU), "spec", "jobManager", "resource", "cpu")
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			err = unstructured.SetNestedField(flinkObj.Object, float64(componentCPU), "spec", "taskManager", "resource", "cpu")
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			_, err = dynamicClient.Resource(flinkDeploymentGVR).Namespace(testNamespace).
				Create(ctx, flinkObj, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			ginkgo.DeferCleanup(func() {
				_ = dynamicClient.Resource(flinkDeploymentGVR).Namespace(testNamespace).
					Delete(ctx, flinkName, metav1.DeleteOptions{})
			})

			pp := helper.NewPropagationPolicy(testNamespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: "flink.apache.org/v1beta1",
					Kind:       "FlinkDeployment",
					Name:       flinkName,
				}},
				policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetCluster},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
					},
				})
			framework.CreatePropagationPolicy(karmadaClient, pp)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, testNamespace, pp.Name)
			})

			return names.GenerateBindingName("FlinkDeployment", flinkName)
		}

		ginkgo.By(fmt.Sprintf("creating FlinkDeployments one by one (up to %d) until assumption exhausts cluster resources", maxFlinkCount), func() {
			assumptionExhausted := false
			for range maxFlinkCount {
				bindingName := createFlinkDeployment()
				// Wait for a definitive scheduling result before creating the next one,
				// ensuring the assumption is recorded before the next workload is evaluated.
				framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, bindingName,
					func(binding *workv1alpha2.ResourceBinding) bool {
						cond := meta.FindStatusCondition(binding.Status.Conditions, workv1alpha2.Scheduled)
						if cond == nil {
							return false
						}
						if cond.Status == metav1.ConditionFalse && cond.Reason == workv1alpha2.BindingReasonSchedulerError &&
							strings.Contains(cond.Message, "no enough resource") {
							assumptionExhausted = true
						}
						return true
					})
				if assumptionExhausted {
					break
				}
			}
			gomega.Expect(assumptionExhausted).Should(gomega.BeTrue(),
				"expected assumption to exhaust cluster resources within %d FlinkDeployments", maxFlinkCount)
		})
	})
})
