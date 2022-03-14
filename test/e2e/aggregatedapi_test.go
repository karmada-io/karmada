package e2e

import (
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("aggregatedapi testing", func() {

	var member1, member2 string
	saName := "tom"
	saNamespace := testNamespace
	saTom := helper.NewServiceaccount(saNamespace, saName)
	clusterroleName := "cluster-proxy-clusterrole"
	crbSubjectGroupName := "system:serviceaccounts:" + saNamespace
	crbSubject := []rbacv1.Subject{{
		Kind:      "ServiceAccount",
		Name:      saName,
		Namespace: saTom.Namespace,
	},
		{
			Kind: "Group",
			Name: "system:serviceaccounts",
		},
		{
			Kind: "Group",
			Name: crbSubjectGroupName,
		},
	}
	saBinding := helper.NewClusterrolebindings(saName, clusterroleName, crbSubject)
	saClustrole := helper.NewClusterroles(saName, "*", "*", "")
	crbSubject2 := []rbacv1.Subject{{
		Kind:      "ServiceAccount",
		Name:      saName,
		Namespace: saTom.Namespace,
	},
	}
	saBinding2 := helper.NewClusterrolebindings(saName, saName, crbSubject2)

	ginkgo.BeforeEach(func() {
		member1 = framework.ClusterNames()[0]
		member2 = framework.ClusterNames()[1]
		karmdaApiGroup := "cluster.karmada.io"
		karmdaResource := "clusters/proxy"
		proxyClustrole := helper.NewClusterroles(clusterroleName, karmdaApiGroup, karmdaResource, member1)
		framework.CreateMermberclusterSa(member1, saTom)
		framework.CreateKarmadaSa(kubeClient, saTom)
		framework.CreateClusterrole(kubeClient, proxyClustrole)
		framework.CreateClusterroleBinding(kubeClient, saBinding)
	})

	ginkgo.AfterEach(func() {
		framework.RemoveMermberclusterSa(member1, saTom.Name, saTom.Namespace)
		framework.RemoveKarmadalusterSa(kubeClient, saTom.Name, saTom.Namespace)
		framework.RemoveClusterrole(kubeClient, clusterroleName)
		framework.RemoveclusterroleBinding(kubeClient, saBinding.Name)
	})

	ginkgo.When("Serviceaccount access test", func() {
		ginkgo.Context("Serviceaccount access test", func() {
			ginkgo.It("Serviceaccount  access in the member1 cluster", func() {
				framework.WaitServiceaccountPresentOnCluster(kubeClient, saName, saTom.Namespace)
				framework.WaitServiceaccountPresentOnMemberCluster(member1, saName, saTom.Namespace)
				framework.WaitClusterrolebindingPresentOnCluster(kubeClient, saBinding.Name)
				apiPath := "/apis/cluster.karmada.io/v1alpha1/clusters/" + member1 + "/proxy/apis"
				code := framework.GetClusterNode(apiPath, kubeClient, karmadaClient, saName, saTom.Namespace)
				gomega.Expect(code).Should(gomega.Equal(200))
			})

			ginkgo.It("Serviceaccount have no  access in the member2 cluster", func() {
				framework.WaitServiceaccountPresentOnCluster(kubeClient, saName, saTom.Namespace)
				framework.WaitServiceaccountPresentOnMemberCluster(member1, saName, saTom.Namespace)
				framework.WaitClusterrolebindingPresentOnCluster(kubeClient, saBinding.Name)
				apiPath := "/apis/cluster.karmada.io/v1alpha1/clusters/" + member2 + "/proxy/apis"
				code := framework.GetClusterNode(apiPath, kubeClient, karmadaClient, saName, saTom.Namespace)
				gomega.Expect(code).ShouldNot(gomega.Equal(200))
				gomega.Expect(code).Should(gomega.Equal(403))
			})

			ginkgo.It("Serviceaccount  does not have any permissions in the member1 cluster", func() {
				framework.WaitServiceaccountPresentOnCluster(kubeClient, saName, saTom.Namespace)
				framework.WaitServiceaccountPresentOnMemberCluster(member1, saName, saTom.Namespace)
				framework.WaitClusterrolebindingPresentOnCluster(kubeClient, saBinding.Name)
				apiPath := "/apis/cluster.karmada.io/v1alpha1/clusters/" + member1 + "/proxy/api/v1/nodes"
				code := framework.GetClusterNode(apiPath, kubeClient, karmadaClient, saName, saTom.Namespace)
				gomega.Expect(code).ShouldNot(gomega.Equal(200))
				gomega.Expect(code).Should(gomega.Equal(403))
			})
		})
	})

	ginkgo.When("Grant permission to Serviceaccount in member ", func() {
		ginkgo.JustBeforeEach(func() {
			framework.CreateMemberclusterRole(member1, saClustrole)
			framework.CreateMemberclusterRoleBinding(member1, saBinding2)
		})

		ginkgo.JustAfterEach(func() {
			framework.RemoveMemberclusterRole(member1, saClustrole.Name)
			framework.RemoveMemberclusterRoleBinding(member1, saBinding2.Name)
		})

		ginkgo.It("Grant permission to Serviceaccount in member1 cluster.", func() {
			framework.WaitServiceaccountPresentOnCluster(kubeClient, saName, saTom.Namespace)
			framework.WaitServiceaccountPresentOnMemberCluster(member1, saName, saTom.Namespace)
			framework.WaitClusterrolebindingPresentOnCluster(kubeClient, saBinding.Name)
			framework.WaitClusterrolebindingPresentOnMemberCluster(member1, saBinding2.Name)
			apiPath := "/apis/cluster.karmada.io/v1alpha1/clusters/" + member1 + "/proxy/api/v1/nodes"
			code := framework.GetClusterNode(apiPath, kubeClient, karmadaClient, saName, saTom.Namespace)
			gomega.Expect(code).Should(gomega.Equal(200))
		})

		ginkgo.It("Grant no permission to Serviceaccount in member2 cluster.", func() {
			framework.WaitServiceaccountPresentOnCluster(kubeClient, saName, saTom.Namespace)
			framework.WaitServiceaccountPresentOnMemberCluster(member1, saName, saTom.Namespace)
			framework.WaitClusterrolebindingPresentOnCluster(kubeClient, saBinding.Name)
			framework.WaitClusterrolebindingPresentOnMemberCluster(member1, saBinding2.Name)
			apiPath := "/apis/cluster.karmada.io/v1alpha1/clusters/" + member2 + "/proxy/api/v1/nodes"
			code := framework.GetClusterNode(apiPath, kubeClient, karmadaClient, saName, saTom.Namespace)
			gomega.Expect(code).ShouldNot(gomega.Equal(200))
			gomega.Expect(code).Should(gomega.Equal(403))
		})
	})
})
