package e2e

import (
	"fmt"

	"github.com/onsi/ginkgo/v2"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[BasicClusterPropagation] basic cluster propagation testing", func() {
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
