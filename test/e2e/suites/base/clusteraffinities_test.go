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
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[ClusterAffinities] propagation testing", func() {
	ginkgo.When("[PropagationPolicy] replica scheduling type is Duplicated", func() {
		ginkgo.Context("schedule with multi clusterAffinity", func() {
			var deployment *appsv1.Deployment
			var policy *policyv1alpha1.PropagationPolicy
			var member1LabelKey, member2LabelKey string
			var member1, member2 string

			ginkgo.BeforeEach(func() {
				member1 = framework.ClusterNames()[0]
				member2 = framework.ClusterNames()[1]
				member1LabelKey = fmt.Sprintf("%s-%s", member1, rand.String(RandomStrLength))
				member2LabelKey = fmt.Sprintf("%s-%s", member2, rand.String(RandomStrLength))

				deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
				policy = testhelper.NewPropagationPolicy(deployment.Namespace, deployment.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{member1LabelKey: "ok"}}},
						},
						{
							AffinityName:    "group2",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{member2LabelKey: "ok"}}},
						},
					}})
			})

			ginkgo.BeforeEach(func() {
				framework.UpdateClusterLabels(karmadaClient, member1, map[string]string{member1LabelKey: "ok"})
				framework.UpdateClusterLabels(karmadaClient, member2, map[string]string{member2LabelKey: "ok"})
				ginkgo.DeferCleanup(func() {
					framework.DeleteClusterLabels(karmadaClient, member1, map[string]string{member1LabelKey: ""})
					framework.DeleteClusterLabels(karmadaClient, member2, map[string]string{member2LabelKey: ""})
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreatePropagationPolicy(karmadaClient, policy)
				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				})
			})

			ginkgo.It("propagate deployment and then update the cluster label", func() {
				// 1. wait for deployment present on member1 cluster
				framework.WaitDeploymentPresentOnClusterFitWith(member1, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })

				// 2. update member1 cluster label to make it's unmatched with the policy
				framework.UpdateClusterLabels(karmadaClient, member1, map[string]string{member1LabelKey: "not-ok"})
				framework.WaitDeploymentDisappearOnCluster(member1, deployment.Namespace, deployment.Name)

				// 3. wait for deployment present on member2 cluster
				framework.WaitDeploymentPresentOnClusterFitWith(member2, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
			})
		})

		ginkgo.Context("schedule change from clusterAffinity to clusterAffinities", func() {
			var deployment *appsv1.Deployment
			var policy *policyv1alpha1.PropagationPolicy
			var member1, member2 string

			ginkgo.BeforeEach(func() {
				member1 = framework.ClusterNames()[0]
				member2 = framework.ClusterNames()[1]
				deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
				policy = testhelper.NewPropagationPolicy(deployment.Namespace, deployment.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{member1}},
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreatePropagationPolicy(karmadaClient, policy)
				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				})
			})

			ginkgo.It("propagate deployment and then update the cluster label", func() {
				// 1. wait for deployment present on member1 cluster
				framework.WaitDeploymentPresentOnClusterFitWith(member1, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })

				// 2. update policy placement with clusterAffinities
				policy.Spec.Placement.ClusterAffinity = nil
				policy.Spec.Placement.ClusterAffinities = []policyv1alpha1.ClusterAffinityTerm{{
					AffinityName:    "group1",
					ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{member2}},
				}}
				framework.UpdatePropagationPolicyWithSpec(karmadaClient, policy.Namespace, policy.Name, policy.Spec)

				// 3. wait for deployment present on member2 cluster
				framework.WaitDeploymentPresentOnClusterFitWith(member2, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
			})
		})

		ginkgo.Context("schedule change from clusterAffinities to clusterAffinity", func() {
			var deployment *appsv1.Deployment
			var policy *policyv1alpha1.PropagationPolicy
			var member1, member2 string

			ginkgo.BeforeEach(func() {
				member1 = framework.ClusterNames()[0]
				member2 = framework.ClusterNames()[1]
				deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
				policy = testhelper.NewPropagationPolicy(deployment.Namespace, deployment.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{{
						AffinityName:    "group1",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{member1}},
					}},
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreatePropagationPolicy(karmadaClient, policy)
				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				})
			})

			ginkgo.It("propagate deployment and then update the cluster label", func() {
				// 1. wait for deployment present on member1 cluster
				framework.WaitDeploymentPresentOnClusterFitWith(member1, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })

				// 2. update policy placement with clusterAffinities
				policy.Spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{ClusterNames: []string{member2}}
				policy.Spec.Placement.ClusterAffinities = nil
				framework.UpdatePropagationPolicyWithSpec(karmadaClient, policy.Namespace, policy.Name, policy.Spec)

				// 3. wait for deployment present on member2 cluster
				framework.WaitDeploymentPresentOnClusterFitWith(member2, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })
			})
		})
	})

	ginkgo.When("[ClusterPropagationPolicy] replica scheduling type is Duplicated", func() {
		ginkgo.Context("schedule with multi clusterAffinity", func() {
			var policy *policyv1alpha1.ClusterPropagationPolicy
			var clusterRole *rbacv1.ClusterRole
			var member1LabelKey, member2LabelKey string
			var member1, member2 string

			ginkgo.BeforeEach(func() {
				member1 = framework.ClusterNames()[0]
				member2 = framework.ClusterNames()[1]
				member1LabelKey = fmt.Sprintf("%s-%s", member1, rand.String(RandomStrLength))
				member2LabelKey = fmt.Sprintf("%s-%s", member2, rand.String(RandomStrLength))

				clusterRole = testhelper.NewClusterRole(clusterRoleNamePrefix+rand.String(RandomStrLength), nil)
				policy = testhelper.NewClusterPropagationPolicy(clusterRole.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: clusterRole.APIVersion,
						Kind:       clusterRole.Kind,
						Name:       clusterRole.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{member1LabelKey: "ok"}}},
						},
						{
							AffinityName:    "group2",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{member2LabelKey: "ok"}}},
						},
					}})
			})

			ginkgo.BeforeEach(func() {
				framework.UpdateClusterLabels(karmadaClient, member1, map[string]string{member1LabelKey: "ok"})
				framework.UpdateClusterLabels(karmadaClient, member2, map[string]string{member2LabelKey: "ok"})
				ginkgo.DeferCleanup(func() {
					framework.DeleteClusterLabels(karmadaClient, member1, map[string]string{member1LabelKey: ""})
					framework.DeleteClusterLabels(karmadaClient, member2, map[string]string{member2LabelKey: ""})
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreateClusterPropagationPolicy(karmadaClient, policy)
				framework.CreateClusterRole(kubeClient, clusterRole)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
					framework.RemoveClusterRole(kubeClient, clusterRole.Name)
				})
			})

			ginkgo.It("propagate clusterRole and then update the cluster label", func() {
				// 1. wait for clusterRole present on member1 cluster
				framework.WaitClusterRolePresentOnClusterFitWith(member1, clusterRole.Name, func(*rbacv1.ClusterRole) bool { return true })

				// 2. update member1 cluster label to make it's unmatched with the policy
				framework.UpdateClusterLabels(karmadaClient, member1, map[string]string{member1LabelKey: "not-ok"})
				framework.WaitClusterRoleDisappearOnCluster(member1, clusterRole.Name)

				// 3. wait for clusterRole present on member2 cluster
				framework.WaitClusterRolePresentOnClusterFitWith(member2, clusterRole.Name, func(*rbacv1.ClusterRole) bool { return true })
			})
		})

		ginkgo.Context("schedule change from clusterAffinity to clusterAffinities", func() {
			var policy *policyv1alpha1.ClusterPropagationPolicy
			var clusterRole *rbacv1.ClusterRole
			var member1, member2 string

			ginkgo.BeforeEach(func() {
				member1 = framework.ClusterNames()[0]
				member2 = framework.ClusterNames()[1]
				clusterRole = testhelper.NewClusterRole(clusterRoleNamePrefix+rand.String(RandomStrLength), nil)
				policy = testhelper.NewClusterPropagationPolicy(clusterRole.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: clusterRole.APIVersion,
						Kind:       clusterRole.Kind,
						Name:       clusterRole.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{member1}},
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreateClusterPropagationPolicy(karmadaClient, policy)
				framework.CreateClusterRole(kubeClient, clusterRole)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
					framework.RemoveClusterRole(kubeClient, clusterRole.Name)
				})
			})

			ginkgo.It("propagate clusterRole and then update the cluster label", func() {
				// 1. wait for clusterRole present on member1 cluster
				framework.WaitClusterRolePresentOnClusterFitWith(member1, clusterRole.Name, func(*rbacv1.ClusterRole) bool { return true })

				// 2. update policy placement with clusterAffinities
				policy.Spec.Placement.ClusterAffinity = nil
				policy.Spec.Placement.ClusterAffinities = []policyv1alpha1.ClusterAffinityTerm{{
					AffinityName:    "group1",
					ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{member2}},
				}}
				framework.UpdateClusterPropagationPolicyWithSpec(karmadaClient, policy.Name, policy.Spec)

				// 3. wait for clusterRole present on member2 cluster
				framework.WaitClusterRolePresentOnClusterFitWith(member2, clusterRole.Name, func(*rbacv1.ClusterRole) bool { return true })
			})
		})

		ginkgo.Context("schedule change from clusterAffinities to clusterAffinity", func() {
			var policy *policyv1alpha1.ClusterPropagationPolicy
			var clusterRole *rbacv1.ClusterRole
			var member1, member2 string

			ginkgo.BeforeEach(func() {
				member1 = framework.ClusterNames()[0]
				member2 = framework.ClusterNames()[1]
				clusterRole = testhelper.NewClusterRole(clusterRoleNamePrefix+rand.String(RandomStrLength), nil)
				policy = testhelper.NewClusterPropagationPolicy(clusterRole.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: clusterRole.APIVersion,
						Kind:       clusterRole.Kind,
						Name:       clusterRole.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{{
						AffinityName:    "group1",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{member1}},
					}},
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreateClusterPropagationPolicy(karmadaClient, policy)
				framework.CreateClusterRole(kubeClient, clusterRole)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
					framework.RemoveClusterRole(kubeClient, clusterRole.Name)
				})
			})

			ginkgo.It("propagate clusterRole and then update the cluster label", func() {
				// 1. wait for clusterRole present on member1 cluster
				framework.WaitClusterRolePresentOnClusterFitWith(member1, clusterRole.Name, func(*rbacv1.ClusterRole) bool { return true })

				// 2. update policy placement with clusterAffinities
				policy.Spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{ClusterNames: []string{member2}}
				policy.Spec.Placement.ClusterAffinities = nil
				framework.UpdateClusterPropagationPolicyWithSpec(karmadaClient, policy.Name, policy.Spec)

				// 3. wait for clusterRole present on member2 cluster
				framework.WaitClusterRolePresentOnClusterFitWith(member2, clusterRole.Name, func(*rbacv1.ClusterRole) bool { return true })
			})
		})
	})

	ginkgo.When("[OverflowAffinities] replica scheduling with overflow affinities", func() {
		// overflowMaxReplicas caps each cluster via ResourceQuota.
		// Each replica requests 10m CPU (test/helper/resource.go), so the quota per cluster
		// is set to overflowMaxReplicas × 10m = 20m.
		const overflowMaxReplicas int32 = 2
		// totalOverflowReplicas exceeds the combined capacity of both clusters (2+2=4),
		// so scheduling should fail when this count is requested.
		const totalOverflowReplicas int32 = 5

		var primaryClusterName, secondaryClusterName string
		var overflowNamespace string
		var overflowRQName, rqPolicyName string

		ginkgo.BeforeEach(func() {
			primaryClusterName = framework.ClusterNames()[0]
			secondaryClusterName = framework.ClusterNames()[1]
		})

		ginkgo.BeforeEach(func() {
			overflowNamespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			err := setupTestNamespace(overflowNamespace, kubeClient)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			ginkgo.DeferCleanup(func() {
				framework.RemoveNamespace(kubeClient, overflowNamespace)
			})
		})

		ginkgo.BeforeEach(func() {
			// ResourceQuota caps each cluster at overflowMaxReplicas replicas.
			overflowRQName = resourceQuotaPrefix + rand.String(RandomStrLength)
			quotaCPU := resource.NewMilliQuantity(int64(overflowMaxReplicas)*10, resource.DecimalSI)
			overflowRQ := &corev1.ResourceQuota{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ResourceQuota"},
				ObjectMeta: metav1.ObjectMeta{Name: overflowRQName, Namespace: overflowNamespace},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{corev1.ResourceRequestsCPU: *quotaCPU},
				},
			}
			framework.CreateResourceQuota(kubeClient, overflowRQ)
			ginkgo.DeferCleanup(func() {
				framework.RemoveResourceQuota(kubeClient, overflowNamespace, overflowRQName)
			})

			rqPolicyName = ppNamePrefix + rand.String(RandomStrLength)
			rqPolicy := testhelper.NewPropagationPolicy(overflowNamespace, rqPolicyName,
				[]policyv1alpha1.ResourceSelector{{APIVersion: "v1", Kind: "ResourceQuota", Name: overflowRQName}},
				policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{primaryClusterName, secondaryClusterName},
					},
				},
			)
			framework.CreatePropagationPolicy(karmadaClient, rqPolicy)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, overflowNamespace, rqPolicyName)
			})
		})

		ginkgo.BeforeEach(func() {
			// Both cluster estimators must observe the quota before scheduling; otherwise
			// calAvailableReplicas falls back to spec.Replicas and overflow never triggers.
			framework.WaitResourceQuotaPresentOnCluster(primaryClusterName, overflowNamespace, overflowRQName)
			framework.WaitResourceQuotaPresentOnCluster(secondaryClusterName, overflowNamespace, overflowRQName)
		})

		ginkgo.Context("scale-up and scale-down with overflow affinities", func() {
			var deployment *appsv1.Deployment
			var deployPolicyName string

			ginkgo.BeforeEach(func() {
				deployName := deploymentNamePrefix + rand.String(RandomStrLength)
				deployment = testhelper.NewDeployment(overflowNamespace, deployName)
				// Start at the primary cap so replicas land only on the primary cluster.
				deployment.Spec.Replicas = ptr.To[int32](overflowMaxReplicas)

				deployPolicyName = ppNamePrefix + rand.String(RandomStrLength)
				deployPolicy := testhelper.NewPropagationPolicy(overflowNamespace, deployPolicyName,
					[]policyv1alpha1.ResourceSelector{{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					}},
					overflowAffinitiesPlacement(primaryClusterName, secondaryClusterName),
				)
				framework.CreatePropagationPolicy(karmadaClient, deployPolicy)
				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
					framework.RemovePropagationPolicy(karmadaClient, overflowNamespace, deployPolicyName)
				})
			})

			ginkgo.It("should expand to secondary on scale-up and retract from secondary on scale-down", func() {
				bindingName := names.GenerateBindingName(util.DeploymentKind, deployment.Name)

				ginkgo.By("replicas are only propagated to the primary group if the primary group has sufficient resources", func() {
					framework.AssertBindingScheduledClusters(karmadaClient, overflowNamespace, bindingName,
						[][]string{{primaryClusterName}},
					)
				})

				ginkgo.By("scale up: replicas overflow to the secondary group if the primary group has insufficient resources", func() {
					// scaling up to 4 replicas: both clusters fill to their quota cap
					framework.UpdateDeploymentReplicas(kubeClient, deployment, overflowMaxReplicas*2)

					// verifying both clusters scheduled to capacity: primary=2, secondary=2
					framework.WaitResourceBindingFitWith(karmadaClient, overflowNamespace, bindingName,
						func(rb *workv1alpha2.ResourceBinding) bool {
							if len(rb.Spec.Clusters) != 2 {
								return false
							}
							m := clusterReplicaMap(rb)
							return m[primaryClusterName] == overflowMaxReplicas &&
								m[secondaryClusterName] == overflowMaxReplicas
						},
					)
				})

				ginkgo.By("schedule fails due to insufficient resources in primary group and overflow affinity groups", func() {
					// scaling up to 5 replicas: exceeds combined quota, scheduling should fail
					framework.UpdateDeploymentReplicas(kubeClient, deployment, totalOverflowReplicas)
					framework.WaitResourceBindingFitWith(karmadaClient, overflowNamespace, bindingName,
						func(rb *workv1alpha2.ResourceBinding) bool {
							for _, cond := range rb.Status.Conditions {
								if cond.Type == "Scheduled" && cond.Status == "False" {
									return true
								}
							}
							return false
						},
					)
				})

				ginkgo.By("scale down: replicas are reduced first from overflow affinity groups", func() {
					// scaling back down to 2 replicas
					framework.UpdateDeploymentReplicas(kubeClient, deployment, overflowMaxReplicas)
					framework.AssertBindingScheduledClusters(karmadaClient, overflowNamespace, bindingName,
						[][]string{{primaryClusterName}},
					)
					framework.WaitDeploymentDisappearOnCluster(secondaryClusterName, overflowNamespace, deployment.Name)
				})
			})
		})
	})

	framework.SerialWhen("[Failover] member cluster become unReachable", func() {
		var deployment *appsv1.Deployment
		var policy *policyv1alpha1.PropagationPolicy
		var member1, member2 string

		ginkgo.BeforeEach(func() {
			member1 = framework.ClusterNames()[0]
			member2 = framework.ClusterNames()[1]
			deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))

			policy = testhelper.NewPropagationPolicy(deployment.Namespace, deployment.Name, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterTolerations: []corev1.Toleration{
					{
						Key:               framework.TaintClusterNotReady,
						Operator:          corev1.TolerationOpExists,
						Effect:            corev1.TaintEffectNoExecute,
						TolerationSeconds: ptr.To[int64](2),
					},
				},
				ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
					{
						AffinityName:    "group1",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{member1}},
					},
					{
						AffinityName:    "group2",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{member2}},
					}}})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
		})

		ginkgo.It("failover testing with propagate deployment by clusterAffinities", func() {
			// 1. set cluster member1 condition status to false
			ginkgo.By("add not-ready:NoExecute taint to the random one cluster", func() {
				err := framework.AddClusterTaint(controlPlaneClient, member1, *framework.NotReadyTaintTemplate)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			// 2. wait for deployment present on member2 cluster
			framework.WaitDeploymentPresentOnClusterFitWith(member2, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool { return true })

			// 3. remove taint from member1 cluster
			ginkgo.By("recover cluster", func() {
				err := framework.RemoveClusterTaint(controlPlaneClient, member1, *framework.NotReadyTaintTemplate)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			})
		})
	})
})

// overflowAffinitiesPlacement returns a Placement that schedules replicas to the primary cluster
// first, spilling to the secondary overflow affinity only when the primary is at capacity. Uses
// Aggregated division so the primary is filled before overflow triggers (Weighted would split
// proportionally).
func overflowAffinitiesPlacement(primary, secondary string) policyv1alpha1.Placement {
	return policyv1alpha1.Placement{
		ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{{
			AffinityName: "primary",
			ClusterAffinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{primary},
			},
			OverflowAffinities: []policyv1alpha1.OverflowClusterAffinity{{
				AffinityName: "secondary",
				ClusterAffinity: policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{secondary},
				},
			}},
		}},
		ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
			ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
			ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
		},
	}
}

// clusterReplicaMap returns a name→replicas map from a ResourceBinding's cluster list.
// Missing clusters return 0 (Go map zero value), so callers can compare directly.
func clusterReplicaMap(rb *workv1alpha2.ResourceBinding) map[string]int32 {
	m := make(map[string]int32, len(rb.Spec.Clusters))
	for _, c := range rb.Spec.Clusters {
		m[c.Name] = c.Replicas
	}
	return m
}
