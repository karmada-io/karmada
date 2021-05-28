package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

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
		propagationPolicy := helper.NewPolicyWithSingleDeployment(propagationPolicyNamespace, propagationPolicyName, deployment, clusterNames)
		overridePolicy := helper.NewOverridePolicyWithDeployment(overridePolicyNamespace, overridePolicyName, deployment, clusterNames, helper.NewImageOverriderWithEmptyPredicate())

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
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						deploymentInCluster, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
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
		propagationPolicy := helper.NewPolicyWithSinglePod(propagationPolicyNamespace, propagationPolicyName, pod, clusterNames)
		overridePolicy := helper.NewOverridePolicyWithPod(overridePolicyNamespace, overridePolicyName, pod, clusterNames, helper.NewImageOverriderWithEmptyPredicate())

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
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						podInClusters, err = clusterClient.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metav1.GetOptions{})

						if err != nil {
							if errors.IsNotFound(err) {
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
		propagationPolicy := helper.NewPolicyWithSingleDeployment(propagationPolicyNamespace, propagationPolicyName, deployment, clusterNames)
		overridePolicy := helper.NewOverridePolicyWithDeployment(overridePolicyNamespace, overridePolicyName, deployment, clusterNames, helper.NewImageOverriderWithPredicate())

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
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						deploymentInCluster, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
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
