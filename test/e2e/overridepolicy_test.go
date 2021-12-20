package e2e

import (
	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[OverridePolicy] apply overriders testing", func() {
	ginkgo.Context("Deployment override all images in container list", func() {
		deploymentNamespace := testNamespace
		deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
		propagationPolicyNamespace := testNamespace
		propagationPolicyName := deploymentName
		overridePolicyNamespace := testNamespace
		overridePolicyName := deploymentName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		propagationPolicy := helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
		overridePolicy := helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.ClusterAffinity{
			ClusterNames: framework.ClusterNames(),
		}, policyv1alpha1.Overriders{
			ImageOverrider: []policyv1alpha1.ImageOverrider{
				{
					Component: "Registry",
					Operator:  "replace",
					Value:     "fictional.registry.us",
				},
				{
					Component: "Repository",
					Operator:  "replace",
					Value:     "busybox",
				},
				{
					Component: "Tag",
					Operator:  "replace",
					Value:     "1.0",
				},
			},
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
		})

		ginkgo.AfterEach(func() {
			framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
		})

		ginkgo.It("deployment imageOverride testing", func() {
			framework.CreateDeployment(kubeClient, deployment)

			klog.Infof("check if deployment present on member clusters have correct image value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					for _, container := range deployment.Spec.Template.Spec.Containers {
						if container.Image != "fictional.registry.us/busybox:1.0" {
							return false
						}
					}
					return true
				})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		})

	})

	ginkgo.Context("Pod override all images in container list", func() {
		podNamespace := testNamespace
		podName := podNamePrefix + rand.String(RandomStrLength)
		propagationPolicyNamespace := testNamespace
		propagationPolicyName := podName
		overridePolicyNamespace := testNamespace
		overridePolicyName := podName

		pod := helper.NewPod(podNamespace, podName)
		propagationPolicy := helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: pod.APIVersion,
				Kind:       pod.Kind,
				Name:       pod.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})
		overridePolicy := helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: pod.APIVersion,
				Kind:       pod.Kind,
				Name:       pod.Name,
			},
		}, policyv1alpha1.ClusterAffinity{
			ClusterNames: framework.ClusterNames(),
		}, policyv1alpha1.Overriders{
			ImageOverrider: []policyv1alpha1.ImageOverrider{
				{
					Component: "Registry",
					Operator:  "replace",
					Value:     "fictional.registry.us",
				},
				{
					Component: "Repository",
					Operator:  "replace",
					Value:     "busybox",
				},
				{
					Component: "Tag",
					Operator:  "replace",
					Value:     "1.0",
				},
			},
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
		})

		ginkgo.AfterEach(func() {
			framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
		})

		ginkgo.It("pod imageOverride testing", func() {
			framework.CreatePod(kubeClient, pod)

			klog.Infof("check if pod present on member clusters have correct image value")
			framework.WaitPodPresentOnClustersFitWith(framework.ClusterNames(), pod.Namespace, pod.Name, func(pod *corev1.Pod) bool {
				for _, container := range pod.Spec.Containers {
					if container.Image != "fictional.registry.us/busybox:1.0" {
						return false
					}
				}
				return true
			})

			framework.RemovePod(kubeClient, pod.Namespace, pod.Name)
		})

	})

	ginkgo.Context("Deployment override specific images in container list", func() {
		deploymentNamespace := testNamespace
		deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
		propagationPolicyNamespace := testNamespace
		propagationPolicyName := deploymentName
		overridePolicyNamespace := testNamespace
		overridePolicyName := deploymentName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		propagationPolicy := helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
		overridePolicy := helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.ClusterAffinity{
			ClusterNames: framework.ClusterNames(),
		}, policyv1alpha1.Overriders{
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
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
		})

		ginkgo.AfterEach(func() {
			framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
		})

		ginkgo.It("deployment imageOverride testing", func() {
			framework.CreateDeployment(kubeClient, deployment)

			klog.Infof("check if deployment present on member clusters have correct image value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return deployment.Spec.Template.Spec.Containers[0].Image == "fictional.registry.us/nginx:1.19.0"
				})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		})

	})
})

var _ = ginkgo.Describe("OverridePolicy with nil resourceSelectors", func() {
	ginkgo.Context("Deployment override testing", func() {
		deploymentNamespace := testNamespace
		deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
		propagationPolicyNamespace := testNamespace
		propagationPolicyName := deploymentName
		overridePolicyNamespace := testNamespace
		overridePolicyName := deploymentName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		propagationPolicy := helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
		overridePolicy := helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, nil, policyv1alpha1.ClusterAffinity{
			ClusterNames: framework.ClusterNames(),
		}, policyv1alpha1.Overriders{
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
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
		})

		ginkgo.AfterEach(func() {
			framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
		})

		ginkgo.It("deployment imageOverride testing", func() {
			framework.CreateDeployment(kubeClient, deployment)

			klog.Infof("check if deployment present on member clusters have correct image value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return deployment.Spec.Template.Spec.Containers[0].Image == "fictional.registry.us/nginx:1.19.0"
				})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		})

	})
})

var _ = ginkgo.Describe("[OverrideRules] apply overriders testing", func() {
	ginkgo.Context("Deployment override all images in container list", func() {
		deploymentNamespace := testNamespace
		deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
		propagationPolicyNamespace := testNamespace
		propagationPolicyName := deploymentName
		overridePolicyNamespace := testNamespace
		overridePolicyName := deploymentName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		propagationPolicy := helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
		overridePolicy := helper.NewOverridePolicyByOverrideRules(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, []policyv1alpha1.RuleWithCluster{
			{
				TargetCluster: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
				Overriders: policyv1alpha1.Overriders{
					ImageOverrider: []policyv1alpha1.ImageOverrider{
						{
							Component: "Registry",
							Operator:  "replace",
							Value:     "fictional.registry.us",
						},
						{
							Component: "Repository",
							Operator:  "replace",
							Value:     "busybox",
						},
						{
							Component: "Tag",
							Operator:  "replace",
							Value:     "1.0",
						},
					},
				},
			},
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
		})

		ginkgo.AfterEach(func() {
			framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
		})

		ginkgo.It("deployment imageOverride testing", func() {
			framework.CreateDeployment(kubeClient, deployment)

			klog.Infof("check if deployment present on member clusters have correct image value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					for _, container := range deployment.Spec.Template.Spec.Containers {
						if container.Image != "fictional.registry.us/busybox:1.0" {
							return false
						}
					}
					return true
				})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		})

	})

	ginkgo.Context("Pod override all images in container list", func() {
		podNamespace := testNamespace
		podName := podNamePrefix + rand.String(RandomStrLength)
		propagationPolicyNamespace := testNamespace
		propagationPolicyName := podName
		overridePolicyNamespace := testNamespace
		overridePolicyName := podName

		pod := helper.NewPod(podNamespace, podName)
		propagationPolicy := helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: pod.APIVersion,
				Kind:       pod.Kind,
				Name:       pod.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})
		overridePolicy := helper.NewOverridePolicyByOverrideRules(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: pod.APIVersion,
				Kind:       pod.Kind,
				Name:       pod.Name,
			},
		}, []policyv1alpha1.RuleWithCluster{
			{
				TargetCluster: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
				Overriders: policyv1alpha1.Overriders{
					ImageOverrider: []policyv1alpha1.ImageOverrider{
						{
							Component: "Registry",
							Operator:  "replace",
							Value:     "fictional.registry.us",
						},
						{
							Component: "Repository",
							Operator:  "replace",
							Value:     "busybox",
						},
						{
							Component: "Tag",
							Operator:  "replace",
							Value:     "1.0",
						},
					},
				},
			},
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
		})

		ginkgo.AfterEach(func() {
			framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
		})

		ginkgo.It("pod imageOverride testing", func() {
			framework.CreatePod(kubeClient, pod)

			klog.Infof("check if pod present on member clusters have correct image value")
			framework.WaitPodPresentOnClustersFitWith(framework.ClusterNames(), pod.Namespace, pod.Name, func(pod *corev1.Pod) bool {
				for _, container := range pod.Spec.Containers {
					if container.Image != "fictional.registry.us/busybox:1.0" {
						return false
					}
				}
				return true
			})

			framework.RemovePod(kubeClient, pod.Namespace, pod.Name)
		})

	})

	ginkgo.Context("Deployment override specific images in container list", func() {
		deploymentNamespace := testNamespace
		deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
		propagationPolicyNamespace := testNamespace
		propagationPolicyName := deploymentName
		overridePolicyNamespace := testNamespace
		overridePolicyName := deploymentName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		propagationPolicy := helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
		overridePolicy := helper.NewOverridePolicyByOverrideRules(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, []policyv1alpha1.RuleWithCluster{
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

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
		})

		ginkgo.AfterEach(func() {
			framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
		})

		ginkgo.It("deployment imageOverride testing", func() {
			framework.CreateDeployment(kubeClient, deployment)

			klog.Infof("check if deployment present on member clusters have correct image value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return deployment.Spec.Template.Spec.Containers[0].Image == "fictional.registry.us/nginx:1.19.0"
				})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		})

	})
})

var _ = ginkgo.Describe("OverrideRules with nil resourceSelectors", func() {
	ginkgo.Context("Deployment override testing", func() {
		deploymentNamespace := testNamespace
		deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
		propagationPolicyNamespace := testNamespace
		propagationPolicyName := deploymentName
		overridePolicyNamespace := testNamespace
		overridePolicyName := deploymentName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		propagationPolicy := helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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

		overridePolicy := helper.NewOverridePolicyByOverrideRules(overridePolicyNamespace, overridePolicyName, nil, []policyv1alpha1.RuleWithCluster{
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

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
		})

		ginkgo.AfterEach(func() {
			framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
		})

		ginkgo.It("deployment imageOverride testing", func() {
			framework.CreateDeployment(kubeClient, deployment)

			klog.Infof("check if deployment present on member clusters have correct image value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return deployment.Spec.Template.Spec.Containers[0].Image == "fictional.registry.us/nginx:1.19.0"
				})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		})

	})
})
