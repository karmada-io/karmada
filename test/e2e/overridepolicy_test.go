package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
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
				ClusterNames: clusterNames,
			},
		})
		overridePolicy := helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.ClusterAffinity{
			ClusterNames: clusterNames,
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
			ginkgo.By(fmt.Sprintf("creating propagationPolicy(%s/%s)", propagationPolicyNamespace, propagationPolicyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(propagationPolicyNamespace).Create(context.TODO(), propagationPolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating overridePolicy(%s/%s)", overridePolicyNamespace, overridePolicyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().OverridePolicies(overridePolicyNamespace).Create(context.TODO(), overridePolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing overridePolicy(%s/%s)", overridePolicyNamespace, overridePolicyName), func() {
				err := karmadaClient.PolicyV1alpha1().OverridePolicies(overridePolicyNamespace).Delete(context.TODO(), overridePolicyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing propagationPolicy(%s/%s)", propagationPolicyNamespace, propagationPolicyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(propagationPolicyNamespace).Delete(context.TODO(), propagationPolicyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("deployment imageOverride testing", func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				_, err := kubeClient.AppsV1().Deployments(testNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment present on member clusters have correct image value", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					var deploymentInCluster *appsv1.Deployment

					klog.Infof("Waiting for deployment(%s/%s) present on cluster(%s)", deploymentNamespace, deploymentName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						deploymentInCluster, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					for _, container := range deploymentInCluster.Spec.Template.Spec.Containers {
						gomega.Expect(container.Image).Should(gomega.Equal("fictional.registry.us/busybox:1.0"))
					}
				}
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
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
				ClusterNames: clusterNames,
			},
		})
		overridePolicy := helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: pod.APIVersion,
				Kind:       pod.Kind,
				Name:       pod.Name,
			},
		}, policyv1alpha1.ClusterAffinity{
			ClusterNames: clusterNames,
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
			ginkgo.By(fmt.Sprintf("creating propagationPolicy(%s/%s)", propagationPolicyNamespace, propagationPolicyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(propagationPolicyNamespace).Create(context.TODO(), propagationPolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating overridePolicy(%s/%s)", overridePolicyNamespace, overridePolicyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().OverridePolicies(overridePolicyNamespace).Create(context.TODO(), overridePolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing overridePolicy(%s/%s)", overridePolicyNamespace, overridePolicyName), func() {
				err := karmadaClient.PolicyV1alpha1().OverridePolicies(overridePolicyNamespace).Delete(context.TODO(), overridePolicyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing propagationPolicy(%s/%s)", propagationPolicyNamespace, propagationPolicyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(propagationPolicyNamespace).Delete(context.TODO(), propagationPolicyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("pod imageOverride testing", func() {
			ginkgo.By(fmt.Sprintf("creating pod(%s/%s)", podNamespace, podName), func() {
				_, err := kubeClient.CoreV1().Pods(testNamespace).Create(context.TODO(), pod, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if pod present on member clusters have correct image value", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					var podInClusters *corev1.Pod

					klog.Infof("Waiting for pod(%s/%s) present on cluster(%s)", podNamespace, podName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						podInClusters, err = clusterClient.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metav1.GetOptions{})

						if err != nil {
							if apierrors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					for _, container := range podInClusters.Spec.Containers {
						gomega.Expect(container.Image).Should(gomega.Equal("fictional.registry.us/busybox:1.0"))
					}
				}
			})

			ginkgo.By(fmt.Sprintf("removing pod(%s/%s)", podNamespace, podName), func() {
				err := kubeClient.CoreV1().Pods(testNamespace).Delete(context.TODO(), podName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
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
				ClusterNames: clusterNames,
			},
		})
		overridePolicy := helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.ClusterAffinity{
			ClusterNames: clusterNames,
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
			ginkgo.By(fmt.Sprintf("creating propagationPolicy(%s/%s)", propagationPolicyNamespace, propagationPolicyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(propagationPolicyNamespace).Create(context.TODO(), propagationPolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating overridePolicy(%s/%s)", overridePolicyNamespace, overridePolicyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().OverridePolicies(overridePolicyNamespace).Create(context.TODO(), overridePolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing overridePolicy(%s/%s)", overridePolicyNamespace, overridePolicyName), func() {
				err := karmadaClient.PolicyV1alpha1().OverridePolicies(overridePolicyNamespace).Delete(context.TODO(), overridePolicyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing propagationPolicy(%s/%s)", propagationPolicyNamespace, propagationPolicyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(propagationPolicyNamespace).Delete(context.TODO(), propagationPolicyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("deployment imageOverride testing", func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				_, err := kubeClient.AppsV1().Deployments(testNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment present on member clusters have correct image value", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					var deploymentInCluster *appsv1.Deployment

					klog.Infof("Waiting for deployment(%s/%s) present on cluster(%s)", deploymentNamespace, deploymentName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						deploymentInCluster, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					gomega.Expect(deploymentInCluster.Spec.Template.Spec.Containers[0].Image).Should(gomega.Equal("fictional.registry.us/nginx:1.19.0"))
				}
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
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
				ClusterNames: clusterNames,
			},
		})
		overridePolicy := helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, nil, policyv1alpha1.ClusterAffinity{
			ClusterNames: clusterNames,
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
			ginkgo.By(fmt.Sprintf("creating propagationPolicy(%s/%s)", propagationPolicyNamespace, propagationPolicyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(propagationPolicyNamespace).Create(context.TODO(), propagationPolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating overridePolicy(%s/%s)", overridePolicyNamespace, overridePolicyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().OverridePolicies(overridePolicyNamespace).Create(context.TODO(), overridePolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing overridePolicy(%s/%s)", overridePolicyNamespace, overridePolicyName), func() {
				err := karmadaClient.PolicyV1alpha1().OverridePolicies(overridePolicyNamespace).Delete(context.TODO(), overridePolicyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing propagationPolicy(%s/%s)", propagationPolicyNamespace, propagationPolicyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(propagationPolicyNamespace).Delete(context.TODO(), propagationPolicyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("deployment imageOverride testing", func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				_, err := kubeClient.AppsV1().Deployments(testNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment present on member clusters have correct image value", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					var deploymentInCluster *appsv1.Deployment

					klog.Infof("Waiting for deployment(%s/%s) present on cluster(%s)", deploymentNamespace, deploymentName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						deploymentInCluster, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					gomega.Expect(deploymentInCluster.Spec.Template.Spec.Containers[0].Image).Should(gomega.Equal("fictional.registry.us/nginx:1.19.0"))
				}
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

	})
})
