package e2e

import (
	"fmt"
	"net/http"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

const (
	clusterProxy = "/apis/cluster.karmada.io/v1alpha1/clusters/%s/proxy/"
)

var _ = ginkgo.Describe("Aggregated Kubernetes API Endpoint testing", func() {
	var member1, member2 string
	var saName, saNamespace string
	var tomServiceAccount *corev1.ServiceAccount
	var tomSecret *corev1.Secret
	var tomClusterRole *rbacv1.ClusterRole
	var tomClusterRoleBinding *rbacv1.ClusterRoleBinding
	var tomClusterRoleOnMember *rbacv1.ClusterRole
	var tomClusterRoleBindingOnMember *rbacv1.ClusterRoleBinding

	ginkgo.BeforeEach(func() {
		member1, member2 = "member1", "member2"

		saName = fmt.Sprintf("tom-%s", rand.String(RandomStrLength))
		saNamespace = testNamespace
		tomServiceAccount = helper.NewServiceaccount(saNamespace, saName)
		tomSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: saNamespace,
				Name:      saName,
				Annotations: map[string]string{
					corev1.ServiceAccountNameKey: saName,
				},
			},
			Type: corev1.SecretTypeServiceAccountToken,
		}
		tomClusterRole = helper.NewClusterRole(tomServiceAccount.Name, []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"cluster.karmada.io"},
				Verbs:         []string{"*"},
				Resources:     []string{"clusters/proxy"},
				ResourceNames: []string{member1},
			},
		})
		tomClusterRoleBinding = helper.NewClusterRoleBinding(tomServiceAccount.Name, tomClusterRole.Name, []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: tomServiceAccount.Name, Namespace: tomServiceAccount.Namespace},
			{Kind: "Group", Name: "system:serviceaccounts"},
			{Kind: "Group", Name: "system:serviceaccounts:" + tomServiceAccount.Namespace},
		})

		tomClusterRoleOnMember = helper.NewClusterRole(tomServiceAccount.Name, []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Verbs:     []string{"*"},
				Resources: []string{"*"},
			},
		})
		tomClusterRoleBindingOnMember = helper.NewClusterRoleBinding(tomServiceAccount.Name, tomClusterRoleOnMember.Name, []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: tomServiceAccount.Name, Namespace: tomServiceAccount.Namespace},
		})
	})

	ginkgo.Context("Aggregated Kubernetes API Endpoint testing", func() {
		var tomToken string

		ginkgo.BeforeEach(func() {
			framework.CreateServiceAccount(kubeClient, tomServiceAccount)
			framework.CreateSecret(kubeClient, tomSecret)
			framework.CreateClusterRole(kubeClient, tomClusterRole)
			framework.CreateClusterRoleBinding(kubeClient, tomClusterRoleBinding)
			ginkgo.DeferCleanup(func() {
				framework.RemoveServiceAccount(kubeClient, tomServiceAccount.Namespace, tomServiceAccount.Name)
				framework.RemoveClusterRole(kubeClient, tomClusterRole.Name)
				framework.RemoveClusterRoleBinding(kubeClient, tomClusterRoleBinding.Name)
			})

			var err error
			tomToken, err = helper.GetTokenFromServiceAccount(kubeClient, tomServiceAccount.Namespace, tomServiceAccount.Name)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.When("Serviceaccount(tom) access the member1 cluster", func() {
			var clusterClient kubernetes.Interface

			ginkgo.BeforeEach(func() {
				clusterClient = framework.GetClusterClient(member1)
				gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
			})

			ginkgo.BeforeEach(func() {
				klog.Infof("Create ServiceAccount(%s) in the cluster(%s)", klog.KObj(tomServiceAccount).String(), member1)
				framework.CreateServiceAccount(clusterClient, tomServiceAccount)
				ginkgo.DeferCleanup(func() {
					klog.Infof("Delete ServiceAccount(%s) in the cluster(%s)", klog.KObj(tomServiceAccount).String(), member1)
					framework.RemoveServiceAccount(clusterClient, tomServiceAccount.Namespace, tomServiceAccount.Name)
				})
			})

			ginkgo.AfterEach(func() {
				clusterClient := framework.GetClusterClient(member1)
				gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

				framework.RemoveClusterRole(clusterClient, tomClusterRoleOnMember.Name)
				framework.RemoveClusterRoleBinding(clusterClient, tomClusterRoleBindingOnMember.Name)
			})

			ginkgo.It("tom access the member cluster", func() {
				ginkgo.By("access the cluster `/api` path with right", func() {
					gomega.Eventually(func() (int, error) {
						code, err := helper.DoRequest(fmt.Sprintf(karmadaHost+clusterProxy+"api", member1), tomToken)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						return code, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(http.StatusOK))
				})

				ginkgo.By("access the cluster /api/v1/nodes path without right", func() {
					code, err := helper.DoRequest(fmt.Sprintf(karmadaHost+clusterProxy+"api/v1/nodes", member1), tomToken)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(code).Should(gomega.Equal(http.StatusForbidden))
				})

				ginkgo.By("create rbac in the member1 cluster", func() {
					framework.CreateClusterRole(clusterClient, tomClusterRoleOnMember)
					framework.CreateClusterRoleBinding(clusterClient, tomClusterRoleBindingOnMember)
				})

				ginkgo.By("access the member1 /api/v1/nodes path with right", func() {
					gomega.Eventually(func() (int, error) {
						code, err := helper.DoRequest(fmt.Sprintf(karmadaHost+clusterProxy+"api/v1/nodes", member1), tomToken)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						return code, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(http.StatusOK))
				})
			})
		})

		ginkgo.When("Serviceaccount(tom) access the member2 cluster", func() {
			ginkgo.It("tom access the member cluster without right", func() {
				ginkgo.By("access the cluster `/api` path without right", func() {
					code, err := helper.DoRequest(fmt.Sprintf(karmadaHost+clusterProxy, member2), tomToken)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(code).Should(gomega.Equal(http.StatusForbidden))
				})
			})
		})
	})
})
