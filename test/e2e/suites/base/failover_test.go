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
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

// cluster failover testing is used to test the rescheduling situation when some initially scheduled clusters fail
var _ = framework.SerialDescribe("cluster failover testing", func() {
	var policyNamespace, policyName string
	var deploymentNamespace, deploymentName string
	var deployment *appsv1.Deployment
	var maxGroups, minGroups int
	var policy *policyv1alpha1.PropagationPolicy

	ginkgo.BeforeEach(func() {
		policyNamespace = testNamespace
		policyName = deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace = testNamespace
		deploymentName = policyName
		deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
		maxGroups = 1
		minGroups = 1

		policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames()[:2],
			},
			ClusterTolerations: []corev1.Toleration{
				{
					Key:               framework.TaintClusterNotReady,
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](2),
				},
			},
			SpreadConstraints: []policyv1alpha1.SpreadConstraint{
				{
					SpreadByField: policyv1alpha1.SpreadByFieldCluster,
					MaxGroups:     maxGroups,
					MinGroups:     minGroups,
				},
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

	ginkgo.Context("Cluster failover testing with nil failover behavior", func() {
		ginkgo.It("deployment failover testing", func() {
			var disabledClusters []string
			targetClusterNames := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)

			ginkgo.By(fmt.Sprintf("add taint %v to the target clusters", framework.NotReadyTaintTemplate), func() {
				for _, targetClusterName := range targetClusterNames {
					err := framework.AddClusterTaint(controlPlaneClient, targetClusterName, *framework.NotReadyTaintTemplate)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					disabledClusters = append(disabledClusters, targetClusterName)
				}
			})

			ginkgo.By("check whether deployment of failed cluster is rescheduled to other available cluster", func() {
				gomega.Eventually(func() int {
					currentClusters := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
					for _, currentCluster := range currentClusters {
						// the current cluster should be overwritten to another available cluster
						if !testhelper.IsExclude(currentCluster, disabledClusters) {
							return 0
						}
					}

					return len(currentClusters)
				}, pollTimeout, pollInterval).Should(gomega.Equal(minGroups))
			})

			ginkgo.By(fmt.Sprintf("remove taint %v from the target clusters", framework.NotReadyTaintTemplate), func() {
				for _, disabledCluster := range disabledClusters {
					err := framework.RemoveClusterTaint(controlPlaneClient, disabledCluster, *framework.NotReadyTaintTemplate)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By("check whether the deployment disappears in the recovered clusters", func() {
				framework.WaitDeploymentDisappearOnClusters(disabledClusters, deploymentNamespace, deploymentName)
			})
		})
	})

	ginkgo.Context("Cluster failover testing with purgeMode gracefully", func() {
		var taint corev1.Taint

		ginkgo.BeforeEach(func() {
			policy.Spec.Placement.ClusterTolerations = []corev1.Toleration{
				{
					Key:               "fail-test",
					Effect:            corev1.TaintEffectNoExecute,
					Operator:          corev1.TolerationOpExists,
					TolerationSeconds: ptr.To[int64](3),
				},
			}
			policy.Spec.Failover = &policyv1alpha1.FailoverBehavior{
				Cluster: &policyv1alpha1.ClusterFailoverBehavior{
					PurgeMode: policyv1alpha1.PurgeModeGracefully,
				},
			}

			taint = corev1.Taint{
				Key:    "fail-test",
				Effect: corev1.TaintEffectNoExecute,
			}
		})

		ginkgo.It("taint Cluster with NoExecute taint", func() {
			var disabledClusters []string
			targetClusterNames := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
			ginkgo.By(fmt.Sprintf("add taint %v to the target clusters", taint), func() {
				for _, targetClusterName := range targetClusterNames {
					err := framework.AddClusterTaint(controlPlaneClient, targetClusterName, taint)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					disabledClusters = append(disabledClusters, targetClusterName)
				}
			})

			ginkgo.By("check whether deployment of taint cluster is rescheduled to other available cluster", func() {
				gomega.Eventually(func() int {
					currentClusters := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
					for _, currentCluster := range currentClusters {
						// the current cluster should be overwritten to another available cluster
						if !testhelper.IsExclude(currentCluster, disabledClusters) {
							return 0
						}
					}

					return len(currentClusters)
				}, pollTimeout, pollInterval).Should(gomega.Equal(minGroups))
			})

			ginkgo.By(fmt.Sprintf("remove taint %v from the target clusters", taint), func() {
				for _, disabledCluster := range disabledClusters {
					err := framework.RemoveClusterTaint(controlPlaneClient, disabledCluster, taint)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By("check whether the deployment disappears in the target clusters", func() {
				framework.WaitDeploymentDisappearOnClusters(disabledClusters, deploymentNamespace, deploymentName)
			})
		})
	})

	ginkgo.Context("Cluster failover testing with purgeMode directly", func() {
		var taint corev1.Taint

		ginkgo.BeforeEach(func() {
			// In order to prevent the injected information from being removed after the new application becomes healthy,
			// we set the deployment image in both clusters to a non-existent image.
			// After a failover occurs once, since there are only two candidate clusters, migration cannot be triggered again.
			deployment.Spec.Template.Spec.Containers[0].Image = "fake/nginx:1.19.0"

			policy.Spec.Placement.ClusterTolerations = []corev1.Toleration{
				{
					Key:               "fail-test",
					Effect:            corev1.TaintEffectNoExecute,
					Operator:          corev1.TolerationOpExists,
					TolerationSeconds: ptr.To[int64](3),
				},
			}
			policy.Spec.Failover = &policyv1alpha1.FailoverBehavior{
				Cluster: &policyv1alpha1.ClusterFailoverBehavior{
					PurgeMode: policyv1alpha1.PurgeModeDirectly,
					StatePreservation: &policyv1alpha1.StatePreservation{
						Rules: []policyv1alpha1.StatePreservationRule{
							{
								AliasLabelName: "test-alias",
								JSONPath:       "{.replicas}",
							},
						},
					},
				},
			}

			taint = corev1.Taint{
				Key:    "fail-test",
				Effect: corev1.TaintEffectNoExecute,
			}
		})

		ginkgo.It("taint Cluster with NoExecute taint", func() {
			targetClusters := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
			ginkgo.By(fmt.Sprintf("add taint %v to the target clusters", taint), func() {
				for _, targetCluster := range targetClusters {
					err := framework.AddClusterTaint(controlPlaneClient, targetCluster, taint)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			var newClusters []string
			ginkgo.By("check whether deployment of taint cluster is rescheduled to other available cluster", func() {
				gomega.Eventually(func() int {
					newClusters = framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
					for _, newCluster := range newClusters {
						// the target cluster should be overwritten to another available cluster
						if !testhelper.IsExclude(newCluster, targetClusters) {
							return 0
						}
					}

					return len(newClusters)
				}, pollTimeout, pollInterval).Should(gomega.Equal(minGroups))
			})

			ginkgo.By("check whether the new deployment has the correct inject info", func() {
				gomega.Eventually(func() bool {
					for _, cluster := range newClusters {
						clusterClient := framework.GetClusterClient(cluster)
						gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

						deployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							return false
						}
						_, exist := deployment.Labels["test-alias"]
						if !exist {
							return false
						}
					}
					return true
				}, pollTimeout, pollInterval).Should(gomega.BeTrue())
			})

			ginkgo.By("check whether the failed deployment disappears in the targetClusters", func() {
				for _, cluster := range targetClusters {
					clusterClient := framework.GetClusterClient(cluster)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					_, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					gomega.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())
				}
			})

			ginkgo.By(fmt.Sprintf("remove taint %v from the target clusters", taint), func() {
				for _, targetCluster := range targetClusters {
					err := framework.RemoveClusterTaint(controlPlaneClient, targetCluster, taint)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})
	})
})

var _ = ginkgo.Describe("application failover testing", func() {
	var policyNamespace, policyName string
	var deploymentNamespace, deploymentName string
	var deployment *appsv1.Deployment
	var policy *policyv1alpha1.PropagationPolicy
	var maxGroups, minGroups int
	var gracePeriodSeconds, tolerationSeconds int32

	ginkgo.BeforeEach(func() {
		policyNamespace = testNamespace
		policyName = deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace = testNamespace
		deploymentName = policyName
		deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
		maxGroups = 1
		minGroups = 1

		policy = &policyv1alpha1.PropagationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: policyNamespace,
				Name:      policyName,
			},
			Spec: policyv1alpha1.PropagationSpec{
				ResourceSelectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				},
				Placement: policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: framework.ClusterNames()[:2],
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     maxGroups,
							MinGroups:     minGroups,
						},
					},
				},
				PropagateDeps: true,
			},
		}
	})

	ginkgo.JustBeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy)
		framework.CreateDeployment(kubeClient, deployment)
		ginkgo.DeferCleanup(func() {
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	ginkgo.Context("Application failover testing with purgeMode gracefully", func() {
		ginkgo.BeforeEach(func() {
			gracePeriodSeconds = 30
			tolerationSeconds = 30
			policy.Spec.Failover = &policyv1alpha1.FailoverBehavior{
				Application: &policyv1alpha1.ApplicationFailoverBehavior{
					DecisionConditions: policyv1alpha1.DecisionConditions{
						TolerationSeconds: ptr.To[int32](tolerationSeconds),
					},
					PurgeMode:          policyv1alpha1.PurgeModeGracefully,
					GracePeriodSeconds: ptr.To[int32](gracePeriodSeconds),
				},
			}
		})

		ginkgo.It("application failover with purgeMode gracefully when the application come back to healthy on the new cluster", func() {
			disabledClusters := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
			ginkgo.By("create an overridePolicy to make the application unhealthy", func() {
				overridePolicy := testhelper.NewOverridePolicyByOverrideRules(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				}, []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							ClusterNames: disabledClusters,
						},
						Overriders: policyv1alpha1.Overriders{
							ImageOverrider: []policyv1alpha1.ImageOverrider{
								{
									Component: "Registry",
									Operator:  policyv1alpha1.OverriderOpReplace,
									Value:     "fake",
								},
							},
						},
					},
				})
				framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			})
			defer framework.RemoveOverridePolicy(karmadaClient, policyNamespace, policyName)

			ginkgo.By("check if deployment present on member clusters has correct image value", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(disabledClusters, deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						for _, container := range deployment.Spec.Template.Spec.Containers {
							if container.Image != "fake/nginx:1.19.0" {
								return false
							}
						}
						return true
					})
			})

			ginkgo.By("check whether the failed deployment disappears in the disabledClusters", func() {
				framework.WaitDeploymentDisappearOnClusters(disabledClusters, deploymentNamespace, deploymentName)
			})

			ginkgo.By("check whether the failed deployment is rescheduled to other available cluster", func() {
				gomega.Eventually(func() int {
					targetClusterNames := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
					for _, targetClusterName := range targetClusterNames {
						// the target cluster should be overwritten to another available cluster
						if !testhelper.IsExclude(targetClusterName, disabledClusters) {
							return 0
						}
					}

					return len(targetClusterNames)
				}, pollTimeout, pollInterval).Should(gomega.Equal(minGroups))
			})
		})

		ginkgo.It("application failover with purgeMode gracefully when the GracePeriodSeconds is reach out", func() {
			gracePeriodSeconds = 10
			ginkgo.By("update the gracePeriodSeconds of the pp", func() {
				// modify gracePeriodSeconds to create a time difference with tolerationSecond to avoid cluster interference
				patch := []map[string]interface{}{
					{
						"op":    policyv1alpha1.OverriderOpReplace,
						"path":  "/spec/failover/application/gracePeriodSeconds",
						"value": ptr.To[int32](gracePeriodSeconds),
					},
				}
				framework.PatchPropagationPolicy(karmadaClient, policy.Namespace, policy.Name, patch, types.JSONPatchType)
			})

			disabledClusters := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
			var beginTime time.Time
			ginkgo.By("create an overridePolicy to make the application unhealthy", func() {
				overridePolicy := testhelper.NewOverridePolicyByOverrideRules(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				}, []policyv1alpha1.RuleWithCluster{
					{
						TargetCluster: &policyv1alpha1.ClusterAffinity{
							// guarantee that application cannot come back to healthy on the new cluster
							ClusterNames: framework.ClusterNames(),
						},
						Overriders: policyv1alpha1.Overriders{
							ImageOverrider: []policyv1alpha1.ImageOverrider{
								{
									Component: "Registry",
									Operator:  policyv1alpha1.OverriderOpReplace,
									Value:     "fake",
								},
							},
						},
					},
				})
				framework.CreateOverridePolicy(karmadaClient, overridePolicy)
				beginTime = time.Now()
			})
			defer framework.RemoveOverridePolicy(karmadaClient, policyNamespace, policyName)

			ginkgo.By("check if deployment present on member clusters has correct image value", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(disabledClusters, deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						for _, container := range deployment.Spec.Template.Spec.Containers {
							if container.Image != "fake/nginx:1.19.0" {
								return false
							}
						}
						return true
					})
			})

			ginkgo.By("check whether application failover with purgeMode gracefully when the GracePeriodSeconds is reach out", func() {
				framework.WaitDeploymentDisappearOnClusters(disabledClusters, deploymentNamespace, deploymentName)
				evictionTime := time.Now()
				gomega.Expect(evictionTime.Sub(beginTime) > time.Duration(gracePeriodSeconds+tolerationSeconds)*time.Second).Should(gomega.BeTrue())
			})
		})
	})

	ginkgo.Context("Application failover testing with purgeMode never", func() {
		ginkgo.BeforeEach(func() {
			tolerationSeconds = 30
			policy.Spec.Failover = &policyv1alpha1.FailoverBehavior{
				Application: &policyv1alpha1.ApplicationFailoverBehavior{
					DecisionConditions: policyv1alpha1.DecisionConditions{
						TolerationSeconds: ptr.To[int32](tolerationSeconds),
					},
					PurgeMode: policyv1alpha1.Never,
				},
			}

			deployment.Spec.Template.Spec.Containers[0].Image = "fake/nginx:1.19.0"
		})

		ginkgo.It("application failover with purgeMode never", func() {
			disabledClusters := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)

			ginkgo.By("check if deployment present on member clusters has correct image value", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(disabledClusters, deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						for _, container := range deployment.Spec.Template.Spec.Containers {
							if container.Image != "fake/nginx:1.19.0" {
								return false
							}
						}
						return true
					})
			})

			ginkgo.By("check whether the failed deployment is rescheduled to other available cluster", func() {
				gomega.Eventually(func() int {
					targetClusterNames := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
					for _, targetClusterName := range targetClusterNames {
						// the target cluster should be overwritten to another available cluster
						if !testhelper.IsExclude(targetClusterName, disabledClusters) {
							return 0
						}
					}

					return len(targetClusterNames)
				}, pollTimeout, pollInterval).Should(gomega.Equal(minGroups))
			})

			ginkgo.By("check whether the failed deployment is present on the disabledClusters", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(disabledClusters, deploymentNamespace, deploymentName, func(*appsv1.Deployment) bool { return true })
			})
		})
	})

	ginkgo.Context("Application failover testing with purgeMode directly", func() {
		ginkgo.BeforeEach(func() {
			tolerationSeconds = 30
			policy.Spec.Failover = &policyv1alpha1.FailoverBehavior{
				Application: &policyv1alpha1.ApplicationFailoverBehavior{
					DecisionConditions: policyv1alpha1.DecisionConditions{
						TolerationSeconds: ptr.To[int32](tolerationSeconds),
					},
					PurgeMode: policyv1alpha1.PurgeModeDirectly,
					StatePreservation: &policyv1alpha1.StatePreservation{
						Rules: []policyv1alpha1.StatePreservationRule{
							{
								AliasLabelName: "test-alias",
								JSONPath:       "{.replicas}",
							},
						},
					},
				},
			}

			// In order to simulate a deployment resource failure scenario, we set a non-existent image here.
			// At the same time, to prevent the injected information from being removed after the new application becomes healthy,
			// we set the deployment image in both clusters to a non-existent image.
			// After a failover occurs once, since there are only two candidate clusters, migration cannot be triggered again.
			deployment.Spec.Template.Spec.Containers[0].Image = "fake/nginx:1.19.0"
		})

		ginkgo.It("Delete the old ones first, then create the new ones", func() {
			disabledClusters := framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)

			var targetClusterNames []string
			ginkgo.By("check whether the failed deployment is rescheduled to other available cluster", func() {
				gomega.Eventually(func() int {
					targetClusterNames = framework.ExtractTargetClustersFromRB(controlPlaneClient, deployment.Kind, deployment.Namespace, deployment.Name)
					for _, targetClusterName := range targetClusterNames {
						// the target cluster should be overwritten to another available cluster
						if !testhelper.IsExclude(targetClusterName, disabledClusters) {
							return 0
						}
					}
					return len(targetClusterNames)
				}, pollTimeout, pollInterval).Should(gomega.Equal(minGroups))
			})

			ginkgo.By("check whether the new deployment has the correct inject info", func() {
				gomega.Eventually(func() bool {
					for _, cluster := range targetClusterNames {
						clusterClient := framework.GetClusterClient(cluster)
						gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

						deployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							return false
						}
						_, exist := deployment.Labels["test-alias"]
						if !exist {
							return false
						}
					}
					return true
				}, pollTimeout, pollInterval).Should(gomega.BeTrue())
			})

			ginkgo.By("check whether the failed deployment disappears in the disabledClusters", func() {
				for _, cluster := range disabledClusters {
					clusterClient := framework.GetClusterClient(cluster)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					_, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					gomega.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())
				}
			})
		})
	})
})

// taintCluster will taint cluster
func taintCluster(c client.Client, clusterName string, taint corev1.Taint) error {
	err := wait.PollUntilContextTimeout(context.TODO(), pollInterval, pollTimeout, true, func(ctx context.Context) (done bool, err error) {
		clusterObj := &clusterv1alpha1.Cluster{}
		if err := c.Get(ctx, client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
			return false, err
		}
		clusterObj.Spec.Taints = append(clusterObj.Spec.Taints, taint)
		if err := c.Update(ctx, clusterObj); err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	return err
}

// recoverTaintedCluster will recover the taint of the disabled cluster
func recoverTaintedCluster(c client.Client, clusterName string, taint corev1.Taint) error {
	err := wait.PollUntilContextTimeout(context.TODO(), pollInterval, pollTimeout, true, func(ctx context.Context) (done bool, err error) {
		clusterObj := &clusterv1alpha1.Cluster{}
		if err := c.Get(ctx, client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
			return false, err
		}
		clusterObj.Spec.Taints = helper.SetCurrentClusterTaints(nil, []*corev1.Taint{&taint}, clusterObj)
		if err := c.Update(ctx, clusterObj); err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	return err
}
