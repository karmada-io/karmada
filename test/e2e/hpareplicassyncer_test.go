package e2e

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/pointer"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("hpa replicas synchronization testing", func() {
	ginkgo.Context("Replicas synchronization testing", func() {
		var policyNamespace, policyName string
		var namespace, deploymentName, hpaName string
		var deployment *appsv1.Deployment
		var hpa *autoscalingv2.HorizontalPodAutoscaler
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			namespace = testNamespace
			policyName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentName = policyName
			hpaName = policyName

			deployment = helper.NewDeployment(namespace, deploymentName)
			deployment.Spec.Replicas = pointer.Int32(1)
			hpa = helper.NewHPA(namespace, hpaName, deploymentName)
			hpa.Spec.MinReplicas = pointer.Int32(2)

			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
				{
					APIVersion: hpa.APIVersion,
					Kind:       hpa.Kind,
					Name:       hpa.Name,
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
			framework.CreateHPA(kubeClient, hpa)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemoveHPA(kubeClient, namespace, hpa.Name)
				framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment has been scaled up and synchronized to Karmada", func() {
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return true
				})

			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return *deployment.Spec.Replicas == *hpa.Spec.MinReplicas
				})

			expectedReplicas := *hpa.Spec.MinReplicas * int32(len(framework.ClusterNames()))
			gomega.Eventually(func() bool {
				deploymentExist, err := kubeClient.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return (*deploymentExist.Spec.Replicas == expectedReplicas) && (deploymentExist.Generation == deploymentExist.Status.ObservedGeneration)
			}, time.Minute, pollInterval).Should(gomega.Equal(true))
		})
	})
})
