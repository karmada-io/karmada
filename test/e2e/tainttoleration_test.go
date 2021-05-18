package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("propagation with taint and toleration testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := testNamespace
		deploymentName := policyName
		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		tolerationKey := "cluster-toleration.karmada.io"
		tolerationValue := "member1"

		// set clusterTolerations to tolerate taints in member1.
		clusterTolerations := []corev1.Toleration{
			{
				Key:      tolerationKey,
				Operator: corev1.TolerationOpEqual,
				Value:    tolerationValue,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		}

		policy := helper.NewPolicyWithClusterToleration(policyNamespace, policyName, deployment, clusterNames, clusterTolerations)

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating policy(%s/%s)", policyNamespace, policyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By("adding taints to clusters", func() {
				for _, cluster := range clusterNames {
					fmt.Printf("add taints to cluster %v", cluster)
					clusterObj := &clusterv1alpha1.Cluster{}
					err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: cluster}, clusterObj)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					clusterObj.Spec.Taints = []corev1.Taint{
						{
							Key:    tolerationKey,
							Value:  clusterObj.Name,
							Effect: corev1.TaintEffectNoSchedule,
						},
					}

					err = controlPlaneClient.Update(context.TODO(), clusterObj)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing policy(%s/%s)", policyNamespace, policyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By("removing taints in cluster", func() {
				for _, cluster := range clusterNames {
					clusterObj := &clusterv1alpha1.Cluster{}
					err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: cluster}, clusterObj)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					clusterObj.Spec.Taints = nil
					err = controlPlaneClient.Update(context.TODO(), clusterObj)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})

		ginkgo.It("deployment with cluster tolerations testing", func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				_, err := kubeClient.AppsV1().Deployments(testNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("check if deployment(%s/%s) only scheduled to tolerated cluster(%s)", deploymentNamespace, deploymentName, tolerationValue), func() {
				targetClusterNames, err := getTargetClusterNames(deployment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(len(targetClusterNames) == 1).Should(gomega.BeTrue())
				gomega.Expect(targetClusterNames[0] == tolerationValue).Should(gomega.BeTrue())
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})
})
