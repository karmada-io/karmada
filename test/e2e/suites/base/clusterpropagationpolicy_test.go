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
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[BasicCase] ClusterPropagationPolicy testing", func() {
	var policyName string
	var policy *policyv1alpha1.ClusterPropagationPolicy

	ginkgo.JustBeforeEach(func() {
		framework.CreateClusterPropagationPolicy(karmadaClient, policy)
		ginkgo.DeferCleanup(func() {
			framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
		})
	})

	ginkgo.Context("CustomResourceDefinition propagation testing", func() {
		var crdGroup string
		var randStr string
		var crdSpecNames apiextensionsv1.CustomResourceDefinitionNames
		var crd *apiextensionsv1.CustomResourceDefinition

		ginkgo.BeforeEach(func() {
			crdGroup = fmt.Sprintf("example-%s.karmada.io", rand.String(RandomStrLength))
			randStr = rand.String(RandomStrLength)
			crdSpecNames = apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     fmt.Sprintf("Foo%s", randStr),
				ListKind: fmt.Sprintf("Foo%sList", randStr),
				Plural:   fmt.Sprintf("foo%ss", randStr),
				Singular: fmt.Sprintf("foo%s", randStr),
			}
			crd = testhelper.NewCustomResourceDefinition(crdGroup, crdSpecNames, apiextensionsv1.NamespaceScoped)
			policyName = crd.Name
			policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: crd.APIVersion,
					Kind:       crd.Kind,
					Name:       crd.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateCRD(dynamicClient, crd)
			ginkgo.DeferCleanup(func() {
				framework.RemoveCRD(dynamicClient, crd.Name)
				framework.WaitCRDDisappearedOnClusters(framework.ClusterNames(), crd.Name)
			})
		})

		ginkgo.It("crd propagation testing", func() {
			framework.GetCRD(dynamicClient, crd.Name)
			framework.WaitCRDPresentOnClusters(karmadaClient, framework.ClusterNames(),
				fmt.Sprintf("%s/%s", crd.Spec.Group, "v1alpha1"), crd.Spec.Names.Kind)
		})
	})

	ginkgo.Context("ClusterRole propagation testing", func() {
		var (
			clusterRoleName string
			clusterRole     *rbacv1.ClusterRole
		)

		ginkgo.BeforeEach(func() {
			policyName = clusterRoleNamePrefix + rand.String(RandomStrLength)
			clusterRoleName = fmt.Sprintf("system:test-%s", policyName)

			clusterRole = testhelper.NewClusterRole(clusterRoleName, nil)
			policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: clusterRole.APIVersion,
					Kind:       clusterRole.Kind,
					Name:       clusterRole.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterRole(kubeClient, clusterRole)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterRole(kubeClient, clusterRole.Name)
				framework.WaitClusterRoleDisappearOnClusters(framework.ClusterNames(), clusterRole.Name)
			})
		})

		ginkgo.It("clusterRole propagation testing", func() {
			framework.WaitClusterRolePresentOnClustersFitWith(framework.ClusterNames(), clusterRole.Name,
				func(*rbacv1.ClusterRole) bool {
					return true
				})
		})
	})

	ginkgo.Context("ClusterRoleBinding propagation testing", func() {
		var (
			clusterRoleBindingName string
			clusterRoleBinding     *rbacv1.ClusterRoleBinding
		)

		ginkgo.BeforeEach(func() {
			policyName = clusterRoleBindingNamePrefix + rand.String(RandomStrLength)
			clusterRoleBindingName = fmt.Sprintf("system:test.tt:%s", policyName)

			clusterRoleBinding = testhelper.NewClusterRoleBinding(clusterRoleBindingName, clusterRoleBindingName, nil)
			policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: clusterRoleBinding.APIVersion,
					Kind:       clusterRoleBinding.Kind,
					Name:       clusterRoleBinding.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterRoleBinding(kubeClient, clusterRoleBinding)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterRoleBinding(kubeClient, clusterRoleBinding.Name)
				framework.WaitClusterRoleBindingDisappearOnClusters(framework.ClusterNames(), clusterRoleBinding.Name)
			})
		})

		ginkgo.It("clusterRoleBinding propagation testing", func() {
			framework.WaitClusterRoleBindingPresentOnClustersFitWith(framework.ClusterNames(), clusterRoleBinding.Name,
				func(*rbacv1.ClusterRoleBinding) bool {
					return true
				})
		})
	})

	ginkgo.Context("Deployment propagation testing", func() {
		var deployment *appsv1.Deployment
		var targetMember string

		ginkgo.BeforeEach(func() {
			targetMember = framework.ClusterNames()[0]
			policyName = cppNamePrefix + rand.String(RandomStrLength)
			deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)

			deployment = testhelper.NewDeployment(testNamespace, deploymentName)

			policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				}}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetMember},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment propagation testing", func() {
			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
				func(d *appsv1.Deployment) bool {
					return *d.Spec.Replicas == *deployment.Spec.Replicas
				})

			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)
			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return *deployment.Spec.Replicas == updateDeploymentReplicas
				})
		})
	})
})

var _ = ginkgo.Describe("[CornerCase] ClusterPropagationPolicy testing", func() {
	var policyName string
	var policy *policyv1alpha1.ClusterPropagationPolicy

	ginkgo.JustBeforeEach(func() {
		framework.CreateClusterPropagationPolicy(karmadaClient, policy)
		ginkgo.DeferCleanup(func() {
			framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
		})
	})

	ginkgo.Context("Deployment propagation testing", func() {
		var deployment *appsv1.Deployment
		var targetMember string

		ginkgo.BeforeEach(func() {
			targetMember = framework.ClusterNames()[0]
			policyName = cppNamePrefix + rand.String(RandomStrLength)
			deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)

			deployment = testhelper.NewDeployment(testNamespace, deploymentName)
			policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				}}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetMember},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment propagation testing", func() {
			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
				func(d *appsv1.Deployment) bool {
					return *d.Spec.Replicas == *deployment.Spec.Replicas
				})

			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)
			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return *deployment.Spec.Replicas == updateDeploymentReplicas
				})
		})
	})
})

var _ = ginkgo.Describe("[AdvancedCase] ClusterPropagationPolicy testing", func() {
	ginkgo.Context("Edit ClusterPropagationPolicy ResourceSelectors", func() {
		ginkgo.When("propagate namespace scope resource", func() {
			var policy *policyv1alpha1.ClusterPropagationPolicy
			var deployment01, deployment02 *appsv1.Deployment
			var targetMember string

			ginkgo.BeforeEach(func() {
				targetMember = framework.ClusterNames()[0]
				policyName := deploymentNamePrefix + rand.String(RandomStrLength)

				deployment01 = testhelper.NewDeployment(testNamespace, policyName+"01")
				deployment02 = testhelper.NewDeployment(testNamespace, policyName+"02")

				policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment01.APIVersion,
						Kind:       deployment01.Kind,
						Name:       deployment01.Name,
					}}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetMember},
					},
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreateClusterPropagationPolicy(karmadaClient, policy)
				framework.CreateDeployment(kubeClient, deployment01)
				framework.CreateDeployment(kubeClient, deployment02)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
					framework.RemoveDeployment(kubeClient, deployment01.Namespace, deployment01.Name)
					framework.RemoveDeployment(kubeClient, deployment02.Namespace, deployment02.Name)
				})

				framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment01.Namespace, deployment01.Name,
					func(*appsv1.Deployment) bool { return true })
			})

			ginkgo.It("add resourceSelectors item", func() {
				framework.UpdateClusterPropagationPolicy(karmadaClient, policy.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment01.APIVersion,
						Kind:       deployment01.Kind,
						Name:       deployment01.Name,
					},
					{
						APIVersion: deployment02.APIVersion,
						Kind:       deployment02.Kind,
						Name:       deployment02.Name,
					},
				})

				framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment02.Namespace, deployment02.Name,
					func(*appsv1.Deployment) bool { return true })
			})

			ginkgo.It("update resourceSelectors item", func() {
				framework.UpdateClusterPropagationPolicy(karmadaClient, policy.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment02.APIVersion,
						Kind:       deployment02.Kind,
						Name:       deployment02.Name,
					},
				})

				framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment02.Namespace, deployment02.Name,
					func(*appsv1.Deployment) bool { return true })
				framework.WaitDeploymentGetByClientFitWith(kubeClient, deployment01.Namespace, deployment01.Name,
					func(deployment *appsv1.Deployment) bool {
						if deployment.Labels == nil {
							return true
						}

						_, exist := deployment.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel]
						return !exist
					})
			})
		})

		ginkgo.When("propagate cluster scope resource", func() {
			var policy *policyv1alpha1.ClusterPropagationPolicy
			var clusterRole01, clusterRole02 *rbacv1.ClusterRole
			var targetMember string

			ginkgo.BeforeEach(func() {
				policyName := clusterRoleNamePrefix + rand.String(RandomStrLength)
				targetMember = framework.ClusterNames()[0]

				clusterRole01 = testhelper.NewClusterRole(fmt.Sprintf("system:test-%s-01", policyName), nil)
				clusterRole02 = testhelper.NewClusterRole(fmt.Sprintf("system:test-%s-02", policyName), nil)
				policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: clusterRole01.APIVersion,
						Kind:       clusterRole01.Kind,
						Name:       clusterRole01.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetMember},
					},
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreateClusterPropagationPolicy(karmadaClient, policy)
				framework.CreateClusterRole(kubeClient, clusterRole01)
				framework.CreateClusterRole(kubeClient, clusterRole02)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
					framework.RemoveClusterRole(kubeClient, clusterRole01.Name)
					framework.RemoveClusterRole(kubeClient, clusterRole02.Name)
				})

				framework.WaitClusterRolePresentOnClusterFitWith(targetMember, clusterRole01.Name,
					func(*rbacv1.ClusterRole) bool {
						return true
					})
			})

			ginkgo.It("add resourceSelectors item", func() {
				framework.UpdateClusterPropagationPolicy(karmadaClient, policy.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: clusterRole01.APIVersion,
						Kind:       clusterRole01.Kind,
						Name:       clusterRole01.Name,
					},
					{
						APIVersion: clusterRole02.APIVersion,
						Kind:       clusterRole02.Kind,
						Name:       clusterRole02.Name,
					},
				})

				framework.WaitClusterRolePresentOnClusterFitWith(targetMember, clusterRole02.Name,
					func(*rbacv1.ClusterRole) bool {
						return true
					})
			})

			ginkgo.It("update resourceSelectors item", func() {
				framework.UpdateClusterPropagationPolicy(karmadaClient, policy.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: clusterRole02.APIVersion,
						Kind:       clusterRole02.Kind,
						Name:       clusterRole02.Name,
					},
				})

				framework.WaitClusterRolePresentOnClusterFitWith(targetMember, clusterRole02.Name,
					func(*rbacv1.ClusterRole) bool {
						return true
					})
				framework.WaitClusterRoleGetByClientFitWith(kubeClient, clusterRole01.Name,
					func(clusterRole *rbacv1.ClusterRole) bool {
						if clusterRole.Labels == nil {
							return true
						}

						_, exist := clusterRole.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel]
						return !exist
					})
			})
		})
	})

	ginkgo.Context("Edit ClusterPropagationPolicy fields other than resourceSelector", func() {
		ginkgo.When("namespace scope resource", func() {
			var policy *policyv1alpha1.ClusterPropagationPolicy
			var deployment *appsv1.Deployment
			var targetMember, updatedMember string

			ginkgo.BeforeEach(func() {
				targetMember = framework.ClusterNames()[0]
				updatedMember = framework.ClusterNames()[1]
				policyName := deploymentNamePrefix + rand.String(RandomStrLength)

				deployment = testhelper.NewDeployment(testNamespace, policyName+"01")

				policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					}}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetMember},
					},
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreateClusterPropagationPolicy(karmadaClient, policy)
				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				})

				gomega.Eventually(func() bool {
					observedPolicy, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Get(context.TODO(), policy.Name, metav1.GetOptions{})
					if err != nil {
						return false
					}
					bindings, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.SelectorFromSet(labels.Set{
							policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: observedPolicy.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel],
						}).String(),
					})
					if err != nil {
						return false
					}
					return len(bindings.Items) != 0
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.It("update policy propagateDeps", func() {
				patch := []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/spec/propagateDeps",
						"value": true,
					},
				}
				framework.PatchClusterPropagationPolicy(karmadaClient, policy.Name, patch, types.JSONPatchType)
				gomega.Eventually(func() bool {
					observedPolicy, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Get(context.TODO(), policy.Name, metav1.GetOptions{})
					if err != nil {
						return false
					}
					bindings, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.SelectorFromSet(labels.Set{
							policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: observedPolicy.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel],
						}).String(),
					})
					if err != nil {
						return false
					}
					return bindings.Items[0].Spec.PropagateDeps == true
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.It("update policy placement", func() {
				updatedPlacement := policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{updatedMember},
					}}
				patch := []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/spec/placement",
						"value": updatedPlacement,
					},
				}
				framework.PatchClusterPropagationPolicy(karmadaClient, policy.Name, patch, types.JSONPatchType)
				framework.WaitDeploymentDisappearOnCluster(targetMember, deployment.Namespace, deployment.Name)
				framework.WaitDeploymentPresentOnClusterFitWith(updatedMember, deployment.Namespace, deployment.Name,
					func(*appsv1.Deployment) bool { return true })
			})
		})

		ginkgo.When("cluster scope resource", func() {
			var policy *policyv1alpha1.ClusterPropagationPolicy
			var clusterRole *rbacv1.ClusterRole
			var targetMember, updatedMember string

			ginkgo.BeforeEach(func() {
				targetMember = framework.ClusterNames()[0]
				updatedMember = framework.ClusterNames()[1]
				policyName := deploymentNamePrefix + rand.String(RandomStrLength)

				clusterRole = testhelper.NewClusterRole(fmt.Sprintf("system:test-%s-01", policyName), nil)

				policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: clusterRole.APIVersion,
						Kind:       clusterRole.Kind,
						Name:       clusterRole.Name,
					}}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetMember},
					},
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreateClusterPropagationPolicy(karmadaClient, policy)
				framework.CreateClusterRole(kubeClient, clusterRole)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
					framework.RemoveClusterRole(kubeClient, clusterRole.Name)
				})

				framework.WaitClusterRolePresentOnClusterFitWith(targetMember, clusterRole.Name,
					func(*rbacv1.ClusterRole) bool {
						return true
					})
			})

			ginkgo.It("update policy placement", func() {
				updatedPlacement := policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{updatedMember},
					}}
				patch := []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/spec/placement",
						"value": updatedPlacement,
					},
				}
				framework.PatchClusterPropagationPolicy(karmadaClient, policy.Name, patch, types.JSONPatchType)
				framework.WaitClusterRoleDisappearOnCluster(targetMember, clusterRole.Name)
				framework.WaitClusterRolePresentOnClusterFitWith(updatedMember, clusterRole.Name,
					func(*rbacv1.ClusterRole) bool {
						return true
					})
			})
		})
	})
})

// ImplicitPriority more than one PP matches the object, we should choose the most suitable one.
// Set it to run sequentially to avoid affecting other test cases.
var _ = framework.SerialDescribe("[ImplicitPriority] ClusterPropagationPolicy testing", func() {
	ginkgo.Context("priorityMatchName/priorityMatchLabel/priorityMatchAll propagation testing", func() {
		var priorityMatchName, priorityMatchLabelSelector, priorityMatchAll string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var policyMatchName, policyMatchLabelSelector, policyPriorityMatchAll *policyv1alpha1.ClusterPropagationPolicy
		var implicitPriorityLabelKey = "priority"
		var implicitPriorityLabelValue = "implicit-priority"

		ginkgo.BeforeEach(func() {
			priorityMatchName = deploymentNamePrefix + "match-name" + rand.String(RandomStrLength)
			priorityMatchLabelSelector = deploymentNamePrefix + "match-labelselector" + rand.String(RandomStrLength)
			priorityMatchAll = deploymentNamePrefix + "match-all" + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)

			deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
			deployment.SetLabels(map[string]string{implicitPriorityLabelKey: implicitPriorityLabelValue})
			policyMatchName = testhelper.NewClusterPropagationPolicy(priorityMatchName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
					Namespace:  deployment.Namespace,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
			policyMatchLabelSelector = testhelper.NewClusterPropagationPolicy(priorityMatchLabelSelector, []policyv1alpha1.ResourceSelector{
				{
					APIVersion:    deployment.APIVersion,
					Kind:          deployment.Kind,
					Namespace:     deployment.Namespace,
					LabelSelector: metav1.SetAsLabelSelector(labels.Set{implicitPriorityLabelKey: implicitPriorityLabelValue}),
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
			policyPriorityMatchAll = testhelper.NewClusterPropagationPolicy(priorityMatchAll, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Namespace:  deployment.Namespace,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, policyMatchName)
			framework.CreateClusterPropagationPolicy(karmadaClient, policyMatchLabelSelector)
			framework.CreateClusterPropagationPolicy(karmadaClient, policyPriorityMatchAll)
			// after all ClusterPropagationPolicy are created, create deployment
			time.Sleep(time.Second * 3)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				// Used to ensure that the remaining resources are deleted when the following test cases fail.
				framework.RemoveClusterPropagationPolicy(karmadaClient, policyMatchName.Name)
				framework.RemoveClusterPropagationPolicy(karmadaClient, policyMatchLabelSelector.Name)
				framework.RemoveClusterPropagationPolicy(karmadaClient, policyPriorityMatchAll.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("priorityMatchName/priorityMatchLabel/priorityMatchAll testing", func() {
			ginkgo.By("check whether the deployment uses the highest priority propagationPolicy(priorityMatchName)", func() {
				defer framework.RemoveClusterPropagationPolicy(karmadaClient, policyMatchName.Name)
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						return deployment.GetAnnotations()[policyv1alpha1.ClusterPropagationPolicyAnnotation] == priorityMatchName
					})
			})
			ginkgo.By("check whether the deployment uses the highest priority propagationPolicy(priorityMatchLabel)", func() {
				defer framework.RemoveClusterPropagationPolicy(karmadaClient, policyMatchLabelSelector.Name)
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						return deployment.GetAnnotations()[policyv1alpha1.ClusterPropagationPolicyAnnotation] == priorityMatchLabelSelector
					})
			})
			ginkgo.By("check whether the deployment uses the highest priority propagationPolicy(priorityMatchAll)", func() {
				defer framework.RemoveClusterPropagationPolicy(karmadaClient, policyPriorityMatchAll.Name)
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						return deployment.GetAnnotations()[policyv1alpha1.ClusterPropagationPolicyAnnotation] == priorityMatchAll
					})
			})
		})
	})
})

// ExplicitPriority more than one CPP matches the object, we should select the one with the highest explicit priority, if the
// explicit priority is same, select the  one with the highest implicit priority.
var _ = ginkgo.Describe("[ExplicitPriority] ClusterPropagationPolicy testing", func() {
	ginkgo.Context("high explicit/low priority/implicit priority ClusterPropagationPolicy propagation testing", func() {
		var higherPriorityLabelSelector, lowerPriorityMatchName, implicitPriorityMatchName string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var policyHigherPriorityLabelSelector, policyLowerPriorityMatchName, policyImplicitPriorityMatchName *policyv1alpha1.ClusterPropagationPolicy

		ginkgo.BeforeEach(func() {
			higherPriorityLabelSelector = deploymentNamePrefix + "higherprioritylabelselector" + rand.String(RandomStrLength)
			lowerPriorityMatchName = deploymentNamePrefix + "lowerprioritymatchname" + rand.String(RandomStrLength)
			implicitPriorityMatchName = deploymentNamePrefix + "implicitprioritymatchname" + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			var priorityLabelKey = "priority"
			var priorityLabelValue = "priority" + rand.String(RandomStrLength)

			deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
			deployment.SetLabels(map[string]string{priorityLabelKey: priorityLabelValue})
			policyHigherPriorityLabelSelector = testhelper.NewExplicitPriorityClusterPropagationPolicy(higherPriorityLabelSelector, []policyv1alpha1.ResourceSelector{
				{
					APIVersion:    deployment.APIVersion,
					Kind:          deployment.Kind,
					LabelSelector: metav1.SetAsLabelSelector(labels.Set{priorityLabelKey: priorityLabelValue}),
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			}, 2)
			policyLowerPriorityMatchName = testhelper.NewExplicitPriorityClusterPropagationPolicy(lowerPriorityMatchName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			}, 1)
			policyImplicitPriorityMatchName = testhelper.NewClusterPropagationPolicy(implicitPriorityMatchName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, policyHigherPriorityLabelSelector)
			framework.CreateClusterPropagationPolicy(karmadaClient, policyLowerPriorityMatchName)
			framework.CreateClusterPropagationPolicy(karmadaClient, policyImplicitPriorityMatchName)

			// Wait till the informer's cache is synced in karmada-controller.
			// Note: We tested and find that it takes about 1s before cache synced.
			time.Sleep(time.Second * 5)

			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)
			})
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, policyHigherPriorityLabelSelector.Name)
				framework.RemoveClusterPropagationPolicy(karmadaClient, policyLowerPriorityMatchName.Name)
			})
		})

		ginkgo.It("high explicit/low priority/implicit priority ClusterPropagationPolicy propagation testing", func() {
			ginkgo.By("check whether the deployment uses the highest explicit priority ClusterPropagationPolicy", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						klog.Infof("Matched ClusterPropagationPolicy:%s", deployment.GetAnnotations()[policyv1alpha1.ClusterPropagationPolicyAnnotation])
						return deployment.GetAnnotations()[policyv1alpha1.ClusterPropagationPolicyAnnotation] == higherPriorityLabelSelector
					})
			})
		})
	})

	ginkgo.Context("same explicit priority ClusterPropagationPolicy propagation testing", func() {
		var explicitPriorityLabelSelector, explicitPriorityMatchName string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var policyExplicitPriorityLabelSelector, policyExplicitPriorityMatchName *policyv1alpha1.ClusterPropagationPolicy

		ginkgo.BeforeEach(func() {
			explicitPriorityLabelSelector = deploymentNamePrefix + "explicitprioritylabelselector" + rand.String(RandomStrLength)
			explicitPriorityMatchName = deploymentNamePrefix + "explicitprioritymatchname" + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			var priorityLabelKey = "priority"
			var priorityLabelValue = "priority" + rand.String(RandomStrLength)

			deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
			deployment.SetLabels(map[string]string{priorityLabelKey: priorityLabelValue})
			policyExplicitPriorityLabelSelector = testhelper.NewExplicitPriorityClusterPropagationPolicy(explicitPriorityLabelSelector, []policyv1alpha1.ResourceSelector{
				{
					APIVersion:    deployment.APIVersion,
					Kind:          deployment.Kind,
					LabelSelector: metav1.SetAsLabelSelector(labels.Set{priorityLabelKey: priorityLabelValue}),
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			}, 1)
			policyExplicitPriorityMatchName = testhelper.NewExplicitPriorityClusterPropagationPolicy(explicitPriorityMatchName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			}, 1)
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, policyExplicitPriorityLabelSelector)
			framework.CreateClusterPropagationPolicy(karmadaClient, policyExplicitPriorityMatchName)

			// Wait till the informer's cache is synced in karmada-controller.
			// Note: We tested and find that it takes about 1s before cache synced.
			time.Sleep(time.Second * 5)

			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)
			})
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, policyExplicitPriorityLabelSelector.Name)
				framework.RemoveClusterPropagationPolicy(karmadaClient, policyExplicitPriorityMatchName.Name)
			})
		})

		ginkgo.It("same explicit priority ClusterPropagationPolicy propagation testing", func() {
			ginkgo.By("check whether the deployment uses the ClusterPropagationPolicy with name matched", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						klog.Infof("Matched ClusterPropagationPolicy:%s", deployment.GetAnnotations()[policyv1alpha1.ClusterPropagationPolicyAnnotation])
						return deployment.GetAnnotations()[policyv1alpha1.ClusterPropagationPolicyAnnotation] == explicitPriorityMatchName
					})
			})
		})
	})
})

// Delete when delete a clusterPropagationPolicy, and no more clusterPropagationPolicy matches the object, something like
// labels should be cleaned.
var _ = ginkgo.Describe("[DeleteCase] ClusterPropagationPolicy testing", func() {
	ginkgo.Context("delete clusterPropagation and remove the labels and annotations from the resource template and reference binding", func() {
		var policy *policyv1alpha1.ClusterPropagationPolicy
		var deployment *appsv1.Deployment
		var targetMember, updateMember string

		ginkgo.BeforeEach(func() {
			targetMember = framework.ClusterNames()[0]
			policyName := deploymentNamePrefix + rand.String(RandomStrLength)

			deployment = testhelper.NewDeployment(testNamespace, policyName+"01")

			policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				}}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetMember},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				// clusterPropagationPolicy will be removed in subsequent test cases
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})

			gomega.Eventually(func() bool {
				observedPolicy, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Get(context.TODO(), policy.Name, metav1.GetOptions{})
				if err != nil {
					return false
				}
				bindings, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labels.SelectorFromSet(labels.Set{
						policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: observedPolicy.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel],
					}).String(),
				})
				if err != nil {
					return false
				}
				return len(bindings.Items) != 0
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		})

		ginkgo.It("delete ClusterPropagationPolicy and check whether labels and annotations are deleted correctly", func() {
			framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
			framework.WaitDeploymentFitWith(kubeClient, deployment.Namespace, deployment.Name, func(dep *appsv1.Deployment) bool {
				if dep.Labels != nil && dep.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel] != "" {
					return false
				}
				if dep.Annotations != nil && dep.Annotations[policyv1alpha1.ClusterPropagationPolicyAnnotation] != "" {
					return false
				}
				return true
			})

			resourceBindingName := names.GenerateBindingName(deployment.Kind, deployment.Name)
			framework.WaitResourceBindingFitWith(karmadaClient, deployment.Namespace, resourceBindingName, func(resourceBinding *workv1alpha2.ResourceBinding) bool {
				if resourceBinding.Labels != nil && resourceBinding.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel] != "" {
					return false
				}
				if resourceBinding.Annotations != nil && resourceBinding.Annotations[policyv1alpha1.ClusterPropagationPolicyAnnotation] != "" {
					return false
				}
				return true
			})
		})

		ginkgo.It("delete the old ClusterPropagationPolicy to unbind and create a new one", func() {
			framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)

			policyName01 := deploymentNamePrefix + rand.String(RandomStrLength)
			updateMember = framework.ClusterNames()[1]
			policy01 := testhelper.NewClusterPropagationPolicy(policyName01, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				}}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{updateMember},
				},
			})
			framework.CreateClusterPropagationPolicy(karmadaClient, policy01)
			defer framework.RemoveClusterPropagationPolicy(karmadaClient, policyName01)

			framework.WaitDeploymentFitWith(kubeClient, deployment.Namespace, deployment.Name, func(dep *appsv1.Deployment) bool {
				return dep.Annotations[policyv1alpha1.ClusterPropagationPolicyAnnotation] == policyName01
			})
			resourceBindingName := names.GenerateBindingName(deployment.Kind, deployment.Name)
			framework.WaitResourceBindingFitWith(karmadaClient, deployment.Namespace, resourceBindingName, func(resourceBinding *workv1alpha2.ResourceBinding) bool {
				return resourceBinding.Annotations[policyv1alpha1.ClusterPropagationPolicyAnnotation] == policyName01
			})

			framework.WaitDeploymentDisappearOnCluster(targetMember, deployment.Namespace, deployment.Name)
			framework.WaitDeploymentPresentOnClusterFitWith(updateMember, deployment.Namespace, deployment.Name, func(*appsv1.Deployment) bool {
				return true
			})
		})
	})

	ginkgo.Context("delete clusterPropagation and remove the labels and annotations from the resource template and reference clusterBinding", func() {
		var crdGroup string
		var randStr string
		var crdSpecNames apiextensionsv1.CustomResourceDefinitionNames
		var crd *apiextensionsv1.CustomResourceDefinition
		var crdPolicy, crdPolicy01 *policyv1alpha1.ClusterPropagationPolicy

		ginkgo.BeforeEach(func() {
			crdGroup = fmt.Sprintf("example-%s.karmada.io", rand.String(RandomStrLength))
			randStr = rand.String(RandomStrLength)
			crdSpecNames = apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     fmt.Sprintf("Foo%s", randStr),
				ListKind: fmt.Sprintf("Foo%sList", randStr),
				Plural:   fmt.Sprintf("foo%ss", randStr),
				Singular: fmt.Sprintf("foo%s", randStr),
			}
			crd = testhelper.NewCustomResourceDefinition(crdGroup, crdSpecNames, apiextensionsv1.ClusterScoped)
			crdPolicy = testhelper.NewClusterPropagationPolicy(crd.Name, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: crd.APIVersion,
					Kind:       crd.Kind,
					Name:       crd.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, crdPolicy)
			framework.CreateCRD(dynamicClient, crd)
			ginkgo.DeferCleanup(func() {
				// clusterPropagationPolicy will be removed in subsequent test cases
				framework.RemoveCRD(dynamicClient, crd.Name)
				framework.WaitCRDDisappearedOnClusters(framework.ClusterNames(), crd.Name)
			})
			gomega.Eventually(func() bool {
				observedPolicy, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Get(context.TODO(), crdPolicy.Name, metav1.GetOptions{})
				if err != nil {
					return false
				}
				bindings, err := karmadaClient.WorkV1alpha2().ClusterResourceBindings().List(context.TODO(), metav1.ListOptions{
					LabelSelector: labels.SelectorFromSet(labels.Set{
						policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: observedPolicy.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel],
					}).String(),
				})
				if err != nil {
					return false
				}
				return len(bindings.Items) != 0
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		})

		ginkgo.It("delete ClusterPropagationPolicy and check whether labels and annotations are deleted correctly", func() {
			framework.RemoveClusterPropagationPolicy(karmadaClient, crdPolicy.Name)
			framework.WaitCRDFitWith(dynamicClient, crd.Name, func(crd *apiextensionsv1.CustomResourceDefinition) bool {
				if crd.Labels != nil && crd.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel] != "" {
					return false
				}
				if crd.Annotations != nil && crd.Annotations[policyv1alpha1.ClusterPropagationPolicyAnnotation] != "" {
					return false
				}
				return true
			})

			resourceBindingName := names.GenerateBindingName(crd.Kind, crd.Name)
			framework.WaitClusterResourceBindingFitWith(karmadaClient, resourceBindingName, func(*workv1alpha2.ClusterResourceBinding) bool {
				if crd.Labels != nil && crd.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel] != "" {
					return false
				}
				if crd.Annotations != nil && crd.Annotations[policyv1alpha1.ClusterPropagationPolicyAnnotation] != "" {
					return false
				}
				return true
			})
		})

		ginkgo.It("delete the old ClusterPropagationPolicy to unbind and create a new one", func() {
			framework.RemoveClusterPropagationPolicy(karmadaClient, crdPolicy.Name)

			crdPolicy01 = testhelper.NewClusterPropagationPolicy(crd.Name+"02", []policyv1alpha1.ResourceSelector{
				{
					APIVersion: crd.APIVersion,
					Kind:       crd.Kind,
					Name:       crd.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
			framework.CreateClusterPropagationPolicy(karmadaClient, crdPolicy01)
			defer framework.RemoveClusterPropagationPolicy(karmadaClient, crdPolicy01.Name)

			framework.WaitCRDFitWith(dynamicClient, crd.Name, func(crd *apiextensionsv1.CustomResourceDefinition) bool {
				return crd.Annotations[policyv1alpha1.ClusterPropagationPolicyAnnotation] == crdPolicy01.Name
			})

			resourceBindingName := names.GenerateBindingName(crd.Kind, crd.Name)
			framework.WaitClusterResourceBindingFitWith(karmadaClient, resourceBindingName, func(crb *workv1alpha2.ClusterResourceBinding) bool {
				return crb.Annotations[policyv1alpha1.ClusterPropagationPolicyAnnotation] == crdPolicy01.Name
			})
		})
	})
})

var _ = ginkgo.Describe("[Suspension] ClusterPropagationPolicy testing", func() {
	var policy *policyv1alpha1.ClusterPropagationPolicy
	var clusterRole *rbacv1.ClusterRole
	var targetMember string
	var resourceBindingName string
	var workName string

	ginkgo.BeforeEach(func() {
		targetMember = framework.ClusterNames()[0]
		policyName := clusterRoleNamePrefix + rand.String(RandomStrLength)
		clusterRoleName := fmt.Sprintf("system:test-%s", policyName)

		clusterRole = testhelper.NewClusterRole(clusterRoleName, nil)
		resourceBindingName = names.GenerateBindingName(clusterRole.Kind, clusterRole.Name)
		workName = names.GenerateWorkName(clusterRole.Kind, clusterRole.Name, clusterRole.Namespace)
		policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: clusterRole.APIVersion,
				Kind:       clusterRole.Kind,
				Name:       clusterRole.Name,
			}}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{targetMember},
			},
		})
	})

	ginkgo.BeforeEach(func() {
		framework.CreateClusterPropagationPolicy(karmadaClient, policy)
		framework.CreateClusterRole(kubeClient, clusterRole)
		framework.WaitClusterRolePresentOnClusterFitWith(targetMember, clusterRole.Name, func(*rbacv1.ClusterRole) bool {
			return true
		})
		ginkgo.DeferCleanup(func() {
			framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
			framework.RemoveClusterRole(kubeClient, clusterRole.Name)
		})
	})

	ginkgo.It("suspend the CPP dispatching", func() {
		ginkgo.By("update the cpp suspension dispatching to true", func() {
			policy.Spec.Suspension = &policyv1alpha1.Suspension{
				Dispatching: ptr.To(true),
			}
			framework.UpdateClusterPropagationPolicyWithSpec(karmadaClient, policy.Name, policy.Spec)
		})

		ginkgo.By("check CRB suspension spec", func() {
			framework.WaitClusterResourceBindingFitWith(karmadaClient, resourceBindingName, func(binding *workv1alpha2.ClusterResourceBinding) bool {
				return binding.Spec.Suspension != nil && ptr.Deref(binding.Spec.Suspension.Dispatching, false)
			})
		})

		ginkgo.By("check Work suspension spec", func() {
			esName := names.GenerateExecutionSpaceName(targetMember)
			gomega.Eventually(func() bool {
				work, err := karmadaClient.WorkV1alpha1().Works(esName).Get(context.TODO(), workName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return work != nil && util.IsWorkSuspendDispatching(work)
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		})

		ginkgo.By("check Work Dispatching status condition", func() {
			esName := names.GenerateExecutionSpaceName(targetMember)
			gomega.Eventually(func() bool {
				work, err := karmadaClient.WorkV1alpha1().Works(esName).Get(context.TODO(), workName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return work != nil && meta.IsStatusConditionPresentAndEqual(work.Status.Conditions, workv1alpha1.WorkDispatching, metav1.ConditionFalse)
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		})
	})
})
