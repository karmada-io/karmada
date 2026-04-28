/*
Copyright 2022 The Karmada Authors.

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
	"os"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/utils/ptr"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/karmadactl/join"
	"github.com/karmada-io/karmada/pkg/karmadactl/unjoin"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = framework.SerialContext("resource reschedule when join or unJoin cluster", ginkgo.Labels{NeedCreateCluster}, func() {
	var newClusterName string
	var homeDir string
	var kubeConfigPath string
	var controlPlane string
	var clusterContext string
	var f cmdutil.Factory

	ginkgo.BeforeEach(func() {
		newClusterName = "member-e2e-" + rand.String(RandomStrLength)
		homeDir = os.Getenv("HOME")
		kubeConfigPath = fmt.Sprintf("%s/.kube/%s.config", homeDir, newClusterName)
		controlPlane = fmt.Sprintf("%s-control-plane", newClusterName)
		clusterContext = fmt.Sprintf("kind-%s", newClusterName)

		defaultConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)
		defaultConfigFlags.Context = &karmadaContext
		f = cmdutil.NewFactory(defaultConfigFlags)
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By(fmt.Sprintf("Creating cluster: %s", newClusterName), func() {
			err := createCluster(newClusterName, kubeConfigPath, controlPlane, clusterContext)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})
		ginkgo.DeferCleanup(func() {
			ginkgo.By(fmt.Sprintf("Deleting clusters: %s", newClusterName), func() {
				err := deleteCluster(newClusterName, kubeConfigPath)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				_ = os.Remove(kubeConfigPath)
			})
		})
	})

	ginkgo.Context("For Duplicated ReplicaSchedulingType, resource should reschedule when join or unJoin cluster", func() {
		ginkgo.Describe("test with namespace-scope resource: Deployment", func() {
			var policyNamespace, policyName string
			var deploymentNamespace, deploymentName string
			var deployment *appsv1.Deployment
			var policy *policyv1alpha1.PropagationPolicy

			ginkgo.BeforeEach(func() {
				policyNamespace = testNamespace
				policyName = deploymentNamePrefix + rand.String(RandomStrLength)
				deploymentNamespace = testNamespace
				deploymentName = policyName
				deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
				deployment.Spec.Replicas = ptr.To[int32](1)

				policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
					},
				})
			})

			ginkgo.It("propagate Deployment and waiting for reschedule", func() {
				framework.CreatePropagationPolicy(karmadaClient, policy)
				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
					framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				})

				ginkgo.By(fmt.Sprintf("Joining cluster: %s", newClusterName), func() {
					opts := join.CommandJoinOption{
						DryRun:            false,
						ClusterNamespace:  "karmada-cluster",
						ClusterName:       newClusterName,
						ClusterContext:    clusterContext,
						ClusterKubeConfig: kubeConfigPath,
					}
					err := opts.Run(f)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					// wait for the current cluster status changing to true
					framework.WaitClusterFitWith(controlPlaneClient, newClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
						return meta.IsStatusConditionPresentAndEqual(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
					})
				})

				ginkgo.By("check whether the deployment is rescheduled to a new cluster", func() {
					gomega.Eventually(func(gomega.Gomega) bool {
						targetClusterNames := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
						return !testhelper.IsExclude(newClusterName, targetClusterNames)
					}, pollTimeout, pollInterval).Should(gomega.BeTrue())
				})

				ginkgo.By(fmt.Sprintf("unJoin target cluster %s", newClusterName), func() {
					opts := unjoin.CommandUnjoinOption{
						ClusterNamespace: "karmada-cluster",
						ClusterName:      newClusterName,
						Wait:             60 * time.Second,
					}
					err := opts.Run(f)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				ginkgo.By("check whether the deployment is rescheduled to other available clusters", func() {
					gomega.Eventually(func(gomega.Gomega) bool {
						targetClusterNames := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
						return testhelper.IsExclude(newClusterName, targetClusterNames)
					}, pollTimeout, pollInterval).Should(gomega.BeTrue())
				})
			})
		})

		ginkgo.Describe("test with cluster-scope resource: ClusterRole", func() {
			var policyName string
			var policy *policyv1alpha1.ClusterPropagationPolicy
			var clusterRoleName string
			var clusterRole *rbacv1.ClusterRole

			ginkgo.BeforeEach(func() {
				policyName = clusterRoleNamePrefix + rand.String(RandomStrLength)
				clusterRoleName = policyName
				clusterRole = testhelper.NewClusterRole(clusterRoleName, nil)

				policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: clusterRole.APIVersion,
						Kind:       clusterRole.Kind,
						Name:       clusterRole.Name,
					},
				}, policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
					},
				})
			})

			ginkgo.It("propagate ClusterRole and waiting for reschedule", func() {
				framework.CreateClusterPropagationPolicy(karmadaClient, policy)
				framework.CreateClusterRole(kubeClient, clusterRole)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterRole(kubeClient, clusterRole.Name)
					framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
				})

				ginkgo.By(fmt.Sprintf("Joining cluster: %s", newClusterName), func() {
					opts := join.CommandJoinOption{
						DryRun:            false,
						ClusterNamespace:  "karmada-cluster",
						ClusterName:       newClusterName,
						ClusterContext:    clusterContext,
						ClusterKubeConfig: kubeConfigPath,
					}
					err := opts.Run(f)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					// wait for the current cluster status changing to true
					framework.WaitClusterFitWith(controlPlaneClient, newClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
						return meta.IsStatusConditionPresentAndEqual(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
					})
				})

				ginkgo.By("check whether the clusterrole is rescheduled to a new cluster", func() {
					gomega.Eventually(func(gomega.Gomega) bool {
						targetClusterNames := framework.ExtractTargetClustersFromCRB(controlPlaneClient, clusterRole.Kind, clusterRole.Name)
						return !testhelper.IsExclude(newClusterName, targetClusterNames)
					}, pollTimeout, pollInterval).Should(gomega.BeTrue())
				})

				ginkgo.By(fmt.Sprintf("unJoin target cluster %s", newClusterName), func() {
					opts := unjoin.CommandUnjoinOption{
						ClusterNamespace: "karmada-cluster",
						ClusterName:      newClusterName,
						Wait:             60 * time.Second,
					}
					err := opts.Run(f)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				ginkgo.By("check whether the clusterRole is rescheduled to other available clusters", func() {
					gomega.Eventually(func(gomega.Gomega) bool {
						targetClusterNames := framework.ExtractTargetClustersFromCRB(controlPlaneClient, clusterRole.Kind, clusterRole.Name)
						return testhelper.IsExclude(newClusterName, targetClusterNames)
					}, pollTimeout, pollInterval).Should(gomega.BeTrue())
				})
			})
		})
	})

	ginkgo.Context("For Divided ReplicaSchedulingType, resource should reschedule when unJoin cluster", func() {
		ginkgo.Describe("test with namespace-scope resource: Deployment", func() {
			var policyNamespace, policyName string
			var deploymentNamespace, deploymentName string
			var deployment *appsv1.Deployment
			var policy *policyv1alpha1.PropagationPolicy

			ginkgo.BeforeEach(func() {
				policyNamespace = testNamespace
				policyName = deploymentNamePrefix + rand.String(RandomStrLength)
				deploymentNamespace = testNamespace
				deploymentName = policyName
				deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
				deployment.Spec.Replicas = ptr.To[int32](10)

				policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					},
				})
			})

			ginkgo.It("propagate Deployment and waiting for reschedule", func() {
				ginkgo.By(fmt.Sprintf("Joining cluster: %s", newClusterName), func() {
					opts := join.CommandJoinOption{
						DryRun:            false,
						ClusterNamespace:  "karmada-cluster",
						ClusterName:       newClusterName,
						ClusterContext:    clusterContext,
						ClusterKubeConfig: kubeConfigPath,
					}
					err := opts.Run(f)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					// wait for the current cluster status changing to true
					framework.WaitClusterFitWith(controlPlaneClient, newClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
						return meta.IsStatusConditionPresentAndEqual(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
					})
				})

				framework.CreatePropagationPolicy(karmadaClient, policy)
				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
					framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				})

				ginkgo.By("check whether the deployment is scheduled to a new cluster", func() {
					gomega.Eventually(func(gomega.Gomega) bool {
						targetClusterNames := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
						return !testhelper.IsExclude(newClusterName, targetClusterNames)
					}, pollTimeout, pollInterval).Should(gomega.BeTrue())
				})

				ginkgo.By(fmt.Sprintf("unJoin target cluster %s", newClusterName), func() {
					opts := unjoin.CommandUnjoinOption{
						ClusterNamespace: "karmada-cluster",
						ClusterName:      newClusterName,
						Wait:             60 * time.Second,
					}
					err := opts.Run(f)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				ginkgo.By("check whether the deployment is rescheduled to other available clusters", func() {
					gomega.Eventually(func(gomega.Gomega) bool {
						targetClusterNames := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
						return testhelper.IsExclude(newClusterName, targetClusterNames)
					}, pollTimeout, pollInterval).Should(gomega.BeTrue())
				})
			})
		})
	})

})

var _ = ginkgo.Describe("resource reschedule when matched cluster labels changed", func() {
	var deployment *appsv1.Deployment
	var targetMember string
	var labelKey string
	var policyNamespace string
	var policyName string

	ginkgo.BeforeEach(func() {
		targetMember = framework.ClusterNames()[0]
		policyNamespace = testNamespace
		policyName = deploymentNamePrefix + rand.String(RandomStrLength)
		labelKey = "cluster" + rand.String(RandomStrLength)

		deployment = testhelper.NewDeployment(testNamespace, policyName)
		framework.CreateDeployment(kubeClient, deployment)

		labels := map[string]string{labelKey: "ok"}
		framework.UpdateClusterLabels(karmadaClient, targetMember, labels)

		ginkgo.DeferCleanup(func() {
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.DeleteClusterLabels(karmadaClient, targetMember, labels)
		})
	})

	ginkgo.Context("test with PropagationPolicy, ReplicaSchedulingType is Duplicated", func() {
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				}}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{labelKey: "ok"},
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)

			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})

			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
				func(*appsv1.Deployment) bool { return true })
		})

		ginkgo.It("change labels to testing deployment reschedule(PropagationPolicy)", func() {
			labelsUpdate := map[string]string{labelKey: "not_ok"}
			framework.UpdateClusterLabels(karmadaClient, targetMember, labelsUpdate)
			framework.WaitDeploymentDisappearOnCluster(targetMember, deployment.Namespace, deployment.Name)

			labelsUpdate = map[string]string{labelKey: "ok"}
			framework.UpdateClusterLabels(karmadaClient, targetMember, labelsUpdate)
			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
				func(*appsv1.Deployment) bool { return true })
		})
	})

	ginkgo.Context("test with ClusterPropagationPolicy, ReplicaSchedulingType is Duplicated", func() {
		var policy *policyv1alpha1.ClusterPropagationPolicy

		ginkgo.BeforeEach(func() {
			policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				}}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{labelKey: "ok"},
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, policy)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
			})
			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
				func(*appsv1.Deployment) bool { return true })
		})

		ginkgo.It("change labels to testing deployment reschedule(ClusterPropagationPolicy)", func() {
			labelsUpdate := map[string]string{labelKey: "not_ok"}
			framework.UpdateClusterLabels(karmadaClient, targetMember, labelsUpdate)
			framework.WaitDeploymentDisappearOnCluster(targetMember, deployment.Namespace, deployment.Name)

			labelsUpdate = map[string]string{labelKey: "ok"}
			framework.UpdateClusterLabels(karmadaClient, targetMember, labelsUpdate)
			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
				func(*appsv1.Deployment) bool { return true })
		})
	})
})
