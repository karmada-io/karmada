package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("porting workloads testing", func() {

	ginkgo.Context("porting workloads from legacy clusters testing", func() {
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

		ginkgo.It("porting Deployments from legacy clusters testing", func() {
			member1 := framework.ClusterNames()[0]
			member1Client := framework.GetClusterClient(member1)
			klog.Infof(
				"Creating deployment(%s/%s) on the member cluster %s first to simulate a scenario where the target cluster already has a deployment with the same name",
				deploymentNamespace, deploymentName, member1,
			)
			framework.CreateDeployment(member1Client, deployment)

			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})

			ginkgo.By("check deployment's replicas", func() {
				wantedReplicas := *deployment.Spec.Replicas * int32(len(framework.Clusters())-1)

				klog.Infof("Waiting for deployment(%s/%s) collecting status", deploymentNamespace, deploymentName)
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					currentDeployment, err := kubeClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					klog.Infof("deployment(%s/%s) readyReplicas: %d, wanted replicas: %d", deploymentNamespace, deploymentName, currentDeployment.Status.ReadyReplicas, wantedReplicas)
					if currentDeployment.Status.ReadyReplicas == wantedReplicas &&
						currentDeployment.Status.AvailableReplicas == wantedReplicas &&
						currentDeployment.Status.UpdatedReplicas == wantedReplicas &&
						currentDeployment.Status.Replicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			klog.Infof(
				"Add a ResourceConflictResolution annotation to deployment(%s/%s) to tell karmada that it needs to take over the deployment that already exists in the target cluster %s",
				deploymentNamespace, deploymentName, member1,
			)
			annotations := map[string]string{workv1alpha2.ResourceConflictResolutionAnnotation: workv1alpha2.ResourceConflictResolutionOverwrite}
			framework.UpdateDeploymentAnnotations(kubeClient, deployment, annotations)

			ginkgo.By("check deployment's replicas after applying the ResourceConflictResolution annotation", func() {
				wantedReplicas := *deployment.Spec.Replicas * int32(len(framework.Clusters()))

				klog.Infof("Waiting for deployment(%s/%s) collecting status", deploymentNamespace, deploymentName)
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					currentDeployment, err := kubeClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					klog.Infof("deployment(%s/%s) readyReplicas: %d, wanted replicas: %d", deploymentNamespace, deploymentName, currentDeployment.Status.ReadyReplicas, wantedReplicas)
					if currentDeployment.Status.ReadyReplicas == wantedReplicas &&
						currentDeployment.Status.AvailableReplicas == wantedReplicas &&
						currentDeployment.Status.UpdatedReplicas == wantedReplicas &&
						currentDeployment.Status.Replicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})
})
