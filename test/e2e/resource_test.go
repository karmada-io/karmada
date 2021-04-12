package e2e

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[resource-status collection] resource status collection testing", func() {

	ginkgo.Context("DeploymentStatus collection testing", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := testNamespace
		deploymentName := policyName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		policy := helper.NewPolicyWithSingleDeployment(policyNamespace, policyName, deployment, clusterNames)

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating policy(%s/%s)", policyNamespace, policyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing policy(%s/%s)", policyNamespace, policyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("deployment status collection testing", func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				_, err := kubeClient.AppsV1().Deployments(testNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check whether the deployment status can be correctly collected", func() {
				wantedReplicas := *deployment.Spec.Replicas * int32(len(clusters))

				klog.Infof("Waiting for deployment(%s/%s) collecting correctly status", deploymentNamespace, deploymentName)
				err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
					currentDeployment, err := kubeClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					klog.Infof("deployment(%s/%s) readyReplicas: %d, wanted replicas: %d", deploymentNamespace, deploymentName, currentDeployment.Status.ReadyReplicas, wantedReplicas)
					if currentDeployment.Status.ReadyReplicas == wantedReplicas &&
						currentDeployment.Status.AvailableReplicas == wantedReplicas &&
						currentDeployment.Status.UpdatedReplicas == wantedReplicas &&
						currentDeployment.Status.Replicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("updating deployment replicas", func() {
				patch := map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": pointer.Int32Ptr(updateDeploymentReplicas),
					},
				}
				bytes, err := json.Marshal(patch)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				_, err = kubeClient.AppsV1().Deployments(deploymentNamespace).Patch(context.TODO(), deploymentName, types.StrategicMergePatchType, bytes, metav1.PatchOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment status has been update whit new collection", func() {
				wantedReplicas := updateDeploymentReplicas * int32(len(clusters))

				klog.Infof("Waiting for deployment(%s/%s) collecting correctly status", deploymentNamespace, deploymentName)
				err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
					currentDeployment, err := kubeClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					if currentDeployment.Status.ReadyReplicas == wantedReplicas &&
						currentDeployment.Status.AvailableReplicas == wantedReplicas &&
						currentDeployment.Status.UpdatedReplicas == wantedReplicas &&
						currentDeployment.Status.Replicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})
})
