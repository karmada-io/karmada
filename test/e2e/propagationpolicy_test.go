package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[propagation policy] propagation policy functionality testing", func() {
	deploymentName := rand.String(6)
	deployment := helper.NewDeployment(testNamespace, deploymentName)
	var err error

	ginkgo.BeforeEach(func() {
		_, err = kubeClient.AppsV1().Deployments(testNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.AfterEach(func() {
		err = kubeClient.CoreV1().Namespaces().Delete(context.TODO(), testNamespace, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	// propagate single resource to two explicit clusters.
	ginkgo.Context("single resource propagation testing", func() {
		ginkgo.It("propagate deployment", func() {
			policyName := rand.String(6)
			policyNamespace := testNamespace // keep policy in the same namespace with the resource
			policy := helper.NewPolicyWithSingleDeployment(policyNamespace, policyName, deployment, clusterNames)

			ginkgo.By(fmt.Sprintf("creating policy: %s/%s", policyNamespace, policyName), func() {
				_, err = karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.By("check if resource appear in member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					err = wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
						if err != nil {
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By(fmt.Sprintf("deleting policy: %s/%s", policyNamespace, policyName), func() {
				err = karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.By("check if resource disappear from member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					err = wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
								return true, nil
							}
						}
						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})
	})

	// propagate two resource to two explicit clusters.
	ginkgo.Context("multiple resource propagation testing", func() {
	})
})
