package e2e

import (
	"fmt"

	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/rand"

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
})
