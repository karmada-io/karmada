package e2e

import (
	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Test clusterOverridePolicy with nil resourceSelectors", func() {
	var deploymentNamespace, deploymentName string
	var propagationPolicyNamespace, propagationPolicyName string
	var clusterOverridePolicyName string
	var deployment *appsv1.Deployment
	var propagationPolicy *policyv1alpha1.PropagationPolicy
	var clusterOverridePolicy *policyv1alpha1.ClusterOverridePolicy

	ginkgo.BeforeEach(func() {
		deploymentNamespace = testNamespace
		deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
		propagationPolicyNamespace = testNamespace
		propagationPolicyName = deploymentName
		clusterOverridePolicyName = deploymentName

		deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
		propagationPolicy = testhelper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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

		clusterOverridePolicy = testhelper.NewClusterOverridePolicyByOverrideRules(clusterOverridePolicyName, nil, []policyv1alpha1.RuleWithCluster{
			{
				TargetCluster: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
				Overriders: policyv1alpha1.Overriders{
					ImageOverrider: []policyv1alpha1.ImageOverrider{
						{
							Predicate: &policyv1alpha1.ImagePredicate{
								Path: "/spec/template/spec/containers/0/image",
							},
							Component: "Registry",
							Operator:  "replace",
							Value:     "fictional.registry.us",
						},
					},
				},
			},
		})
	})

	ginkgo.Context("Deployment override testing", func() {
		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateDeployment(kubeClient, deployment)
		})

		ginkgo.AfterEach(func() {
			framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		})

		ginkgo.It("deployment imageOverride testing", func() {
			ginkgo.By("Check if deployment have presented on member clusters", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						return true
					})
			})

			framework.CreateClusterOverridePolicy(karmadaClient, clusterOverridePolicy)

			ginkgo.By("Check if deployment presented on member clusters have correct image value", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						return deployment.Spec.Template.Spec.Containers[0].Image == "fictional.registry.us/nginx:1.19.0"
					})
			})

			framework.RemoveClusterOverridePolicy(karmadaClient, clusterOverridePolicy.Name)
		})
	})
})
