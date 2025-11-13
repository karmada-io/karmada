/*
Copyright 2021 The Karmada Authors.

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
	"os"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/karmadactl/join"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/unjoin"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

const waitTimeout = 3 * time.Second

var admissionWebhookDenyMsgPrefix = "admission webhook \"resourcebinding.karmada.io\" denied the request"

var _ = framework.SerialDescribe("FederatedResourceQuota auto-provision testing", func() {
	var frqNamespace, frqName string
	var federatedResourceQuota *policyv1alpha1.FederatedResourceQuota
	var f cmdutil.Factory

	ginkgo.BeforeEach(func() {
		frqNamespace = testNamespace
		frqName = federatedResourceQuotaPrefix + rand.String(RandomStrLength)

		defaultConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)
		defaultConfigFlags.Context = &karmadaContext
		f = cmdutil.NewFactory(defaultConfigFlags)
	})

	ginkgo.It("CURD a federatedResourceQuota", func() {
		var clusters = framework.ClusterNames()
		federatedResourceQuota = helper.NewFederatedResourceQuota(frqNamespace, frqName, framework.ClusterNames())
		ginkgo.By("[Create] federatedResourceQuota should be propagated to member clusters", func() {
			framework.CreateFederatedResourceQuota(karmadaClient, federatedResourceQuota)
			framework.WaitResourceQuotaPresentOnClusters(clusters, frqNamespace, frqName)
		})

		ginkgo.By("[Update] federatedResourceQuota should be propagated to member clusters according to the new staticAssignments", func() {
			clusters = []string{framework.ClusterNames()[0]}
			federatedResourceQuota = helper.NewFederatedResourceQuota(frqNamespace, frqName, clusters)
			patch := []map[string]interface{}{
				{
					"op":    "replace",
					"path":  "/spec/staticAssignments",
					"value": federatedResourceQuota.Spec.StaticAssignments,
				},
			}
			framework.UpdateFederatedResourceQuotaWithPatch(karmadaClient, frqNamespace, frqName, patch, types.JSONPatchType)
			framework.WaitResourceQuotaPresentOnClusters(clusters, frqNamespace, frqName)
			framework.WaitResourceQuotaDisappearOnClusters(framework.ClusterNames()[1:], frqNamespace, frqName)
		})

		ginkgo.By("[Delete] federatedResourceQuota should be removed from member clusters", func() {
			framework.RemoveFederatedResourceQuota(karmadaClient, frqNamespace, frqName)
			framework.WaitResourceQuotaDisappearOnClusters(clusters, frqNamespace, frqName)
		})
	})

	framework.SerialContext("join new cluster", ginkgo.Labels{NeedCreateCluster}, func() {
		var clusterNameInStaticAssignment string
		var clusterNameNotInStaticAssignment string
		var homeDir string
		var kubeConfigPathInStaticAssignment string
		var kubeConfigPathNotInStaticAssignment string
		var controlPlaneInStaticAssignment string
		var controlPlaneNotInStaticAssignment string
		var clusterContextInStaticAssignment string
		var clusterContextNotInStaticAssignment string

		ginkgo.BeforeEach(func() {
			clusterNameInStaticAssignment = "member-e2e-" + rand.String(RandomStrLength)
			clusterNameNotInStaticAssignment = "member-e2e-" + rand.String(RandomStrLength)
			homeDir = os.Getenv("HOME")
			kubeConfigPathInStaticAssignment = fmt.Sprintf("%s/.kube/%s.config", homeDir, clusterNameInStaticAssignment)
			kubeConfigPathNotInStaticAssignment = fmt.Sprintf("%s/.kube/%s.config", homeDir, clusterNameNotInStaticAssignment)
			controlPlaneInStaticAssignment = fmt.Sprintf("%s-control-plane", clusterNameInStaticAssignment)
			controlPlaneNotInStaticAssignment = fmt.Sprintf("%s-control-plane", clusterNameNotInStaticAssignment)
			clusterContextInStaticAssignment = fmt.Sprintf("kind-%s", clusterNameInStaticAssignment)
			clusterContextNotInStaticAssignment = fmt.Sprintf("kind-%s", clusterNameNotInStaticAssignment)
		})

		ginkgo.BeforeEach(func() {
			clusterNames := append(framework.ClusterNames(), clusterNameInStaticAssignment)
			federatedResourceQuota = helper.NewFederatedResourceQuota(frqNamespace, frqName, clusterNames)
			framework.CreateFederatedResourceQuota(karmadaClient, federatedResourceQuota)
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Creating cluster: %s", clusterNameInStaticAssignment), func() {
				err := createCluster(clusterNameInStaticAssignment, kubeConfigPathInStaticAssignment, controlPlaneInStaticAssignment, clusterContextInStaticAssignment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.By(fmt.Sprintf("Creating cluster: %s", clusterNameNotInStaticAssignment), func() {
				err := createCluster(clusterNameNotInStaticAssignment, kubeConfigPathNotInStaticAssignment, controlPlaneNotInStaticAssignment, clusterContextNotInStaticAssignment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("Unjoinning cluster: %s", clusterNameInStaticAssignment), func() {
				opts := unjoin.CommandUnjoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterNameInStaticAssignment,
					ClusterContext:    clusterContextInStaticAssignment,
					ClusterKubeConfig: kubeConfigPathInStaticAssignment,
					Wait:              5 * options.DefaultKarmadactlCommandDuration,
				}
				err := opts.Run(f)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.By(fmt.Sprintf("Unjoinning cluster: %s", clusterNameNotInStaticAssignment), func() {
				opts := unjoin.CommandUnjoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterNameNotInStaticAssignment,
					ClusterContext:    clusterContextNotInStaticAssignment,
					ClusterKubeConfig: kubeConfigPathNotInStaticAssignment,
					Wait:              5 * options.DefaultKarmadactlCommandDuration,
				}
				err := opts.Run(f)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("Deleting clusters: %s", clusterNameInStaticAssignment), func() {
				err := deleteCluster(clusterNameInStaticAssignment, kubeConfigPathInStaticAssignment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				_ = os.Remove(kubeConfigPathInStaticAssignment)
			})
			ginkgo.By(fmt.Sprintf("Deleting clusters: %s", clusterNameNotInStaticAssignment), func() {
				err := deleteCluster(clusterNameNotInStaticAssignment, kubeConfigPathNotInStaticAssignment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				_ = os.Remove(kubeConfigPathNotInStaticAssignment)
			})
		})

		ginkgo.AfterEach(func() {
			framework.RemoveFederatedResourceQuota(karmadaClient, frqNamespace, frqName)
		})

		ginkgo.It("federatedResourceQuota should only be propagated to newly joined cluster if the new cluster is declared in the StaticAssignment", func() {
			ginkgo.By(fmt.Sprintf("Joining cluster: %s", clusterNameInStaticAssignment), func() {
				opts := join.CommandJoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterNameInStaticAssignment,
					ClusterContext:    clusterContextInStaticAssignment,
					ClusterKubeConfig: kubeConfigPathInStaticAssignment,
				}
				err := opts.Run(f)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("waiting federatedResourceQuota(%s/%s) present on cluster: %s", frqNamespace, frqName, clusterNameInStaticAssignment), func() {
				clusterClient, err := util.NewClusterClientSet(clusterNameInStaticAssignment, controlPlaneClient, nil)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					_, err := clusterClient.KubeClient.CoreV1().ResourceQuotas(frqNamespace).Get(context.TODO(), frqName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())
					return true, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By(fmt.Sprintf("Joining cluster: %s", clusterNameNotInStaticAssignment), func() {
				opts := join.CommandJoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterNameNotInStaticAssignment,
					ClusterContext:    clusterContextNotInStaticAssignment,
					ClusterKubeConfig: kubeConfigPathNotInStaticAssignment,
				}
				err := opts.Run(f)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("check if federatedResourceQuota(%s/%s) present on cluster: %s", frqNamespace, frqName, clusterNameNotInStaticAssignment), func() {
				clusterClient, err := util.NewClusterClientSet(clusterNameNotInStaticAssignment, controlPlaneClient, nil)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				time.Sleep(waitTimeout)
				_, err = clusterClient.KubeClient.CoreV1().ResourceQuotas(frqNamespace).Get(context.TODO(), frqName, metav1.GetOptions{})
				gomega.Expect(apierrors.IsNotFound(err)).Should(gomega.Equal(true))
			})
		})
	})
})

var _ = framework.SerialDescribe("[FederatedResourceQuota] status collection testing", func() {
	var frqNamespace, frqName string
	var federatedResourceQuota *policyv1alpha1.FederatedResourceQuota

	ginkgo.BeforeEach(func() {
		frqNamespace = testNamespace
		frqName = federatedResourceQuotaPrefix + rand.String(RandomStrLength)
		federatedResourceQuota = helper.NewFederatedResourceQuota(frqNamespace, frqName, framework.ClusterNames())
	})

	ginkgo.Context("collect federatedResourceQuota status", func() {
		ginkgo.AfterEach(func() {
			framework.RemoveFederatedResourceQuota(karmadaClient, frqNamespace, frqName)
		})

		ginkgo.It("federatedResourceQuota status should be collect correctly", func() {
			framework.CreateFederatedResourceQuota(karmadaClient, federatedResourceQuota)
			framework.WaitFederatedResourceQuotaCollectStatus(karmadaClient, frqNamespace, frqName)

			patch := []map[string]interface{}{
				{
					"op":    "replace",
					"path":  "/spec/staticAssignments/0/hard/cpu",
					"value": "2",
				},
				{
					"op":    "replace",
					"path":  "/spec/staticAssignments/1/hard/memory",
					"value": "4Gi",
				},
			}
			framework.UpdateFederatedResourceQuotaWithPatch(karmadaClient, frqNamespace, frqName, patch, types.JSONPatchType)
			framework.WaitFederatedResourceQuotaCollectStatus(karmadaClient, frqNamespace, frqName)
		})
	})
})

var _ = ginkgo.Describe("FederatedResourceQuota enforcement testing", func() {
	var frqNamespace, frqName string
	var federatedResourceQuota *policyv1alpha1.FederatedResourceQuota
	var clusterNames []string
	var deployNamespace string

	ginkgo.BeforeEach(func() {
		// To avoid conflicts with other test cases, use random strings to generate unique namespaces instead of using testNamespace.
		deployNamespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
		err := setupTestNamespace(deployNamespace, kubeClient)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		ginkgo.DeferCleanup(func() {
			framework.RemoveNamespace(kubeClient, deployNamespace)
		})
		frqNamespace = deployNamespace
		frqName = federatedResourceQuotaPrefix + rand.String(RandomStrLength)
		clusterNames = framework.ClusterNames()[:1]
	})

	ginkgo.Context("[Compute resource] FederatedResourceQuota should be enforced correctly", func() {
		var policyNamespace, policyName string
		var deploymentNamespace, deploymentName string
		var overall corev1.ResourceList
		var deployment *appsv1.Deployment
		var rbName, rbNamespace string

		ginkgo.BeforeEach(func() {
			policyNamespace = deployNamespace
			policyName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentNamespace = deployNamespace
			deploymentName = policyName

			ginkgo.By("Creating federatedResourceQuota", func() {
				overall = corev1.ResourceList{
					"cpu":    resource.MustParse("10m"),
					"memory": resource.MustParse("100Mi"),
				}
				federatedResourceQuota = helper.NewFederatedResourceQuotaWithOverall(frqNamespace, frqName, overall)
				framework.CreateFederatedResourceQuota(karmadaClient, federatedResourceQuota)
				ginkgo.DeferCleanup(func() {
					framework.RemoveFederatedResourceQuota(karmadaClient, frqNamespace, frqName)
				})
			})

			ginkgo.By("Deploying a deployment and propagationpolicy", func() {
				deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
				deployment.Spec.Replicas = ptr.To[int32](1)
				rbName = names.GenerateBindingName(deployment.Kind, deploymentName)
				rbNamespace = deployment.GetNamespace()
				policy := helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: clusterNames,
					},
				})

				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
					framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)
				})

				framework.CreatePropagationPolicy(karmadaClient, policy)
				ginkgo.DeferCleanup(func() {
					framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				})
			})

			ginkgo.By("The quota has not been exceeded, and the deployment can be successfully propagated to the member clusters.", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(clusterNames, deploymentNamespace, deploymentName,
					func(*appsv1.Deployment) bool {
						return true
					})
				frq, err := karmadaClient.PolicyV1alpha1().FederatedResourceQuotas(frqNamespace).Get(context.TODO(), frqName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(frq.Status.Overall).Should(gomega.Equal(overall))
				gomega.Expect(frq.Status.OverallUsed).Should(gomega.Equal(corev1.ResourceList{
					"cpu": resource.MustParse("10m"),
				}))
			})
		})

		ginkgo.It("Intercept update requests for requirements if they exceed the quota.", func() {
			ginkgo.By("update the requirements of the deployment", func() {
				mutateFunc := func(deploy *appsv1.Deployment) {
					deploy.Spec.Template.Spec.Containers[0].Resources.Requests = corev1.ResourceList{
						"cpu": resource.MustParse("20m"),
					}
				}

				framework.UpdateDeploymentWith(kubeClient, deploymentNamespace, deploymentName, mutateFunc)
			})

			ginkgo.By("The quota has been exceeded, so the update request for the requirements in resourcebinding will be intercepted.", func() {
				framework.WaitEventFitWith(kubeClient, deploymentNamespace, deploymentName, func(event corev1.Event) bool {
					return event.Reason == events.EventReasonApplyPolicyFailed && strings.Contains(event.Message, admissionWebhookDenyMsgPrefix)
				})
			})

			ginkgo.By("The spec.replicaRequirements of resourcebinding was not updated.", func() {
				rb, err := karmadaClient.WorkV1alpha2().ResourceBindings(rbNamespace).Get(context.TODO(), rbName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(rb.Spec.ReplicaRequirements.ResourceRequest).Should(gomega.Equal(corev1.ResourceList{
					"cpu": resource.MustParse("10m"),
				}))
			})
		})

		ginkgo.It("Deploy workloads should be rejected in case of no enough quota left", func() {
			var newPolicyNamespace, newPolicyName string
			var newDeploymentNamespace, newDeploymentName string
			var newDeployment *appsv1.Deployment

			ginkgo.By("create a new deployment and propagationpolicy", func() {
				newPolicyNamespace = deployNamespace
				newPolicyName = deploymentNamePrefix + rand.String(RandomStrLength)
				newDeploymentNamespace = deployNamespace
				newDeploymentName = newPolicyName

				newDeployment = helper.NewDeployment(newDeploymentNamespace, newDeploymentName)
				newDeployment.Spec.Replicas = ptr.To[int32](1)
				newPolicy := helper.NewPropagationPolicy(newPolicyNamespace, newPolicyName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: newDeployment.APIVersion,
						Kind:       newDeployment.Kind,
						Name:       newDeployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: clusterNames,
					},
				})

				framework.CreateDeployment(kubeClient, newDeployment)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, newDeployment.Namespace, newDeployment.Name)
					framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), newDeployment.Namespace, newDeployment.Name)
				})

				framework.CreatePropagationPolicy(karmadaClient, newPolicy)
				ginkgo.DeferCleanup(func() {
					framework.RemovePropagationPolicy(karmadaClient, newPolicy.Namespace, newPolicy.Name)
				})

			})

			ginkgo.By("The quota has been exceeded, so the update request for the spec.clusters in the new resourcebinding will be intercepted.", func() {
				newRB := names.GenerateBindingName(newDeployment.Kind, newDeploymentName)
				framework.WaitEventFitWith(kubeClient, newDeploymentNamespace, newRB, func(event corev1.Event) bool {
					return event.Reason == events.EventReasonScheduleBindingFailed && strings.Contains(event.Message, admissionWebhookDenyMsgPrefix)
				})
				framework.WaitEventFitWith(kubeClient, newDeploymentNamespace, newDeploymentName, func(event corev1.Event) bool {
					return event.Reason == events.EventReasonScheduleBindingFailed && strings.Contains(event.Message, admissionWebhookDenyMsgPrefix)
				})

				gomega.Eventually(func() bool {
					rb, err := karmadaClient.WorkV1alpha2().ResourceBindings(newDeploymentNamespace).Get(context.TODO(), newRB, metav1.GetOptions{})
					if err != nil {
						return false
					}
					return rb != nil && meta.IsStatusConditionPresentAndEqual(rb.Status.Conditions, workv1alpha2.Scheduled, metav1.ConditionFalse)
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
				framework.WaitResourceBindingFitWith(karmadaClient, newDeploymentNamespace, newRB, func(resourceBinding *workv1alpha2.ResourceBinding) bool {
					return resourceBinding.Spec.Clusters == nil
				})
				framework.WaitDeploymentDisappearOnClusters(clusterNames, newDeploymentNamespace, newDeploymentName)
			})
		})

		ginkgo.It("When the quota is insufficient, scaling up will be blocked.", func() {
			ginkgo.By("update the replicas of the deployment", func() {
				mutateFunc := func(deploy *appsv1.Deployment) {
					deploy.Spec.Replicas = ptr.To[int32](2)
				}

				framework.UpdateDeploymentWith(kubeClient, deploymentNamespace, deploymentName, mutateFunc)
			})

			ginkgo.By("the quota has been exceed, so the update request for the spec.clusters in the resourcebinding will be intercepted.", func() {
				framework.WaitEventFitWith(kubeClient, rbNamespace, rbName, func(event corev1.Event) bool {
					return event.Reason == events.EventReasonScheduleBindingFailed && strings.Contains(event.Message, admissionWebhookDenyMsgPrefix)
				})
				framework.WaitEventFitWith(kubeClient, deploymentNamespace, deploymentName, func(event corev1.Event) bool {
					return event.Reason == events.EventReasonScheduleBindingFailed && strings.Contains(event.Message, admissionWebhookDenyMsgPrefix)
				})

				gomega.Eventually(func() bool {
					rb, err := karmadaClient.WorkV1alpha2().ResourceBindings(rbNamespace).Get(context.TODO(), rbName, metav1.GetOptions{})
					if err != nil {
						return false
					}
					return rb != nil && meta.IsStatusConditionPresentAndEqual(rb.Status.Conditions, workv1alpha2.Scheduled, metav1.ConditionFalse)
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By("The spec.clusters of resourcebinding was not updated.", func() {
				rb, err := karmadaClient.WorkV1alpha2().ResourceBindings(rbNamespace).Get(context.TODO(), rbName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				for i := range rb.Spec.Clusters {
					gomega.Expect(rb.Spec.Clusters[i].Replicas).Should(gomega.Equal(int32(1)))
				}

				time.Sleep(waitTimeout)
				framework.WaitDeploymentPresentOnClustersFitWith(clusterNames, deploymentNamespace, deploymentName, func(deployment *appsv1.Deployment) bool {
					return *deployment.Spec.Replicas == 1
				})
			})
		})
	})
})
