package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[BasicClusterPropagation] propagation testing", func() {
	ginkgo.Context("CustomResourceDefinition propagation testing", func() {
		var crdGroup string
		var randStr string
		var crdSpecNames apiextensionsv1.CustomResourceDefinitionNames
		var crd *apiextensionsv1.CustomResourceDefinition
		var crdPolicy *policyv1alpha1.ClusterPropagationPolicy

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
				framework.RemoveClusterPropagationPolicy(karmadaClient, crdPolicy.Name)
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
			policyName      string
			policy          *policyv1alpha1.ClusterPropagationPolicy
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
			framework.CreateClusterPropagationPolicy(karmadaClient, policy)
			framework.CreateClusterRole(kubeClient, clusterRole)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
				framework.RemoveClusterRole(kubeClient, clusterRole.Name)
				framework.WaitClusterRoleDisappearOnClusters(framework.ClusterNames(), clusterRole.Name)
			})
		})

		ginkgo.It("clusterRole propagation testing", func() {
			framework.WaitClusterRolePresentOnClustersFitWith(framework.ClusterNames(), clusterRole.Name,
				func(role *rbacv1.ClusterRole) bool {
					return true
				})
		})
	})

	ginkgo.Context("ClusterRoleBinding propagation testing", func() {
		var (
			clusterRoleBindingName string
			policyName             string
			policy                 *policyv1alpha1.ClusterPropagationPolicy
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
			framework.CreateClusterPropagationPolicy(karmadaClient, policy)
			framework.CreateClusterRoleBinding(kubeClient, clusterRoleBinding)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
				framework.RemoveClusterRoleBinding(kubeClient, clusterRoleBinding.Name)
				framework.WaitClusterRoleBindingDisappearOnClusters(framework.ClusterNames(), clusterRoleBinding.Name)
			})
		})

		ginkgo.It("clusterRoleBinding propagation testing", func() {
			framework.WaitClusterRoleBindingPresentOnClustersFitWith(framework.ClusterNames(), clusterRoleBinding.Name,
				func(binding *rbacv1.ClusterRoleBinding) bool {
					return true
				})
		})
	})
})

var _ = ginkgo.Describe("[AdvancedClusterPropagation] propagation testing", func() {
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
					func(deployment *appsv1.Deployment) bool { return true })
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
					func(deployment *appsv1.Deployment) bool { return true })
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
					func(deployment *appsv1.Deployment) bool { return true })
				framework.WaitDeploymentGetByClientFitWith(kubeClient, deployment01.Namespace, deployment01.Name,
					func(deployment *appsv1.Deployment) bool {
						if deployment.Labels == nil {
							return true
						}

						_, exist := deployment.Labels[policyv1alpha1.ClusterPropagationPolicyLabel]
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
					func(role *rbacv1.ClusterRole) bool {
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
					func(role *rbacv1.ClusterRole) bool {
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
					func(role *rbacv1.ClusterRole) bool {
						return true
					})
				framework.WaitClusterRoleGetByClientFitWith(kubeClient, clusterRole01.Name,
					func(clusterRole *rbacv1.ClusterRole) bool {
						if clusterRole.Labels == nil {
							return true
						}

						_, exist := clusterRole.Labels[policyv1alpha1.ClusterPropagationPolicyLabel]
						return !exist
					})
			})
		})
	})

	ginkgo.Context("Edit ClusterPropagationPolicy PropagateDeps", func() {

		ginkgo.When("namespace scope resource", func() {
			var policy *policyv1alpha1.ClusterPropagationPolicy
			var deployment *appsv1.Deployment
			var targetMember string

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
					framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				})

				gomega.Eventually(func() bool {
					bindings, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.SelectorFromSet(labels.Set{
							policyv1alpha1.ClusterPropagationPolicyLabel: policy.Name,
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
					bindings, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.SelectorFromSet(labels.Set{
							policyv1alpha1.ClusterPropagationPolicyLabel: policy.Name,
						}).String(),
					})
					if err != nil {
						return false
					}
					return bindings.Items[0].Spec.PropagateDeps == true
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})
})

// ExplicitPriority more than one CPP matches the object, we should select the one with the highest explicit priority, if the
// explicit priority is same, select the  one with the highest implicit priority.
var _ = ginkgo.Describe("[ExplicitPriority] propagation testing", func() {
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

		ginkgo.It("high explicit/low priority/implicit priority ClusterPropagationPolicy testing", func() {
			ginkgo.By("check whether the deployment uses the highest explicit priority ClusterPropagationPolicy", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						klog.Infof("Matched ClusterPropagationPolicy:%s", deployment.GetLabels()[policyv1alpha1.ClusterPropagationPolicyLabel])
						return deployment.GetLabels()[policyv1alpha1.ClusterPropagationPolicyLabel] == higherPriorityLabelSelector
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
						klog.Infof("Matched ClusterPropagationPolicy:%s", deployment.GetLabels()[policyv1alpha1.ClusterPropagationPolicyLabel])
						return deployment.GetLabels()[policyv1alpha1.ClusterPropagationPolicyLabel] == explicitPriorityMatchName
					})
			})
		})
	})
})
