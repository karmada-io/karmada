package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[resource-status collection] resource status collection testing", func() {

	ginkgo.Context("DeploymentStatus collection testing", func() {
		var policyNamespace, policyName string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = policyName

			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
		})

		ginkgo.It("deployment status collection testing", func() {
			ginkgo.By("check whether the deployment status can be correctly collected", func() {
				wantedReplicas := *deployment.Spec.Replicas * int32(len(framework.Clusters()))

				klog.Infof("Waiting for deployment(%s/%s) collecting correctly status", deploymentNamespace, deploymentName)
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
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

			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)

			ginkgo.By("check if deployment status has been update whit new collection", func() {
				wantedReplicas := updateDeploymentReplicas * int32(len(framework.Clusters()))

				klog.Infof("Waiting for deployment(%s/%s) collecting correctly status", deploymentNamespace, deploymentName)
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
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
		})
	})
})
