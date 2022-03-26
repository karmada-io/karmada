package e2e

import (
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

const (
	clusterProxy = "/apis/cluster.karmada.io/v1alpha1/clusters/%s/proxy/"
)

var _ = ginkgo.Describe("Aggregated Kubernetes API Endpoint testing", func() {
	member1, member2 := "member1", "member2"

	saName := fmt.Sprintf("tom-%s", rand.String(RandomStrLength))
	saNamespace := testNamespace
	tomServiceAccount := helper.NewServiceaccount(saNamespace, saName)
	tomClusterRole := helper.NewClusterRole(tomServiceAccount.Name, []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"cluster.karmada.io"},
			Verbs:         []string{"*"},
			Resources:     []string{"clusters/proxy"},
			ResourceNames: []string{member1},
		},
	})
	tomClusterRoleBinding := helper.NewClusterRoleBinding(tomServiceAccount.Name, tomClusterRole.Name, []rbacv1.Subject{
		{Kind: "ServiceAccount", Name: tomServiceAccount.Name, Namespace: tomServiceAccount.Namespace},
		{Kind: "Group", Name: "system:serviceaccounts"},
		{Kind: "Group", Name: "system:serviceaccounts:" + tomServiceAccount.Namespace},
	})

	tomClusterRoleOnMember := helper.NewClusterRole(tomServiceAccount.Name, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"*"},
			Verbs:     []string{"*"},
			Resources: []string{"*"},
		},
	})
	tomClusterRoleBindingOnMember := helper.NewClusterRoleBinding(tomServiceAccount.Name, tomClusterRoleOnMember.Name, []rbacv1.Subject{
		{Kind: "ServiceAccount", Name: tomServiceAccount.Name, Namespace: tomServiceAccount.Namespace},
	})

	ginkgo.Context("Aggregated Kubernetes API Endpoint testing", func() {
		ginkgo.BeforeEach(func() {
			framework.CreateServiceAccount(kubeClient, tomServiceAccount)
			framework.CreateClusterRole(kubeClient, tomClusterRole)
			framework.CreateClusterRoleBinding(kubeClient, tomClusterRoleBinding)
		})

		ginkgo.BeforeEach(func() {
			// Wait for namespace present before creating resources in it.
			framework.WaitNamespacePresentOnCluster(member1, tomServiceAccount.Namespace)

			klog.Infof("Create ServiceAccount(%s) in the cluster(%s)", klog.KObj(tomServiceAccount).String(), member1)
			clusterClient := framework.GetClusterClient(member1)
			gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
			framework.CreateServiceAccount(clusterClient, tomServiceAccount)
		})

		ginkgo.AfterEach(func() {
			klog.Infof("Delete ServiceAccount(%s) in the cluster(%s)", klog.KObj(tomServiceAccount).String(), member1)
			clusterClient := framework.GetClusterClient(member1)
			gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
			framework.RemoveServiceAccount(clusterClient, tomServiceAccount.Namespace, tomServiceAccount.Name)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveServiceAccount(kubeClient, tomServiceAccount.Namespace, tomServiceAccount.Name)
			framework.RemoveClusterRole(kubeClient, tomClusterRole.Name)
			framework.RemoveClusterRoleBinding(kubeClient, tomClusterRoleBinding.Name)
		})

		ginkgo.AfterEach(func() {
			clusterClient := framework.GetClusterClient(member1)
			gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

			framework.RemoveClusterRole(clusterClient, tomClusterRoleOnMember.Name)
			framework.RemoveClusterRoleBinding(clusterClient, tomClusterRoleBindingOnMember.Name)
		})

		ginkgo.It("Serviceaccount(tom) access the member1 cluster", func() {
			tomToken, err := helper.GetTokenFromServiceAccount(kubeClient, tomServiceAccount.Namespace, tomServiceAccount.Name)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			ginkgo.By("access the member1 /api path with right", func() {
				gomega.Eventually(func() (bool, error) {
					code, err := helper.DoRequest(fmt.Sprintf(karmadaHost+clusterProxy+"api", member1), tomToken)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					if code == 200 {
						return true, nil
					}
					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By("access the member1 /api/v1/nodes path without right", func() {
				code, err := helper.DoRequest(fmt.Sprintf(karmadaHost+clusterProxy+"api/v1/nodes", member1), tomToken)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(code).Should(gomega.Equal(403))
			})

			ginkgo.By("create rbac in member1 cluster", func() {
				clusterClient := framework.GetClusterClient(member1)
				gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

				framework.CreateClusterRole(clusterClient, tomClusterRoleOnMember)
				framework.CreateClusterRoleBinding(clusterClient, tomClusterRoleBindingOnMember)
			})

			ginkgo.By("access the member1 /api/v1/nodes path with right", func() {
				gomega.Eventually(func() (bool, error) {
					code, err := helper.DoRequest(fmt.Sprintf(karmadaHost+clusterProxy+"api/v1/nodes", member1), tomToken)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					if code == 200 {
						return true, nil
					}
					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})

		ginkgo.It("Serviceaccount(tom) access the member2 cluster", func() {
			tomToken, err := helper.GetTokenFromServiceAccount(kubeClient, tomServiceAccount.Namespace, tomServiceAccount.Name)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			ginkgo.By("access the member2 /api path without right", func() {
				code, err := helper.DoRequest(fmt.Sprintf(karmadaHost+clusterProxy, member2), tomToken)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(code).Should(gomega.Equal(403))
			})
		})
	})
})
