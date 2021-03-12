package e2e

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"github.com/karmada-io/karmada/test/helper"
)

const (
	deploymentNamePrefix = "deploy-"
	serviceNamePrefix    = "service-"

	updateDeploymentReplicas = 6
	updateServicePort        = 81
)

// BasicPropagation focus on basic propagation functionality testing.
var _ = ginkgo.Describe("[BasicPropagation] basic propagation testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
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

		ginkgo.It("deployment propagation testing", func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				_, err := kubeClient.AppsV1().Deployments(testNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment present on member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for deployment(%s/%s) present on cluster(%s)", deploymentNamespace, deploymentName, cluster.Name)
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By("updating deployment", func() {
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

			ginkgo.By("check if update has been synced to member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for deployment(%s/%s) synced on cluster(%s)", deploymentNamespace, deploymentName, cluster.Name)
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						dep, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							return false, err
						}

						if *dep.Spec.Replicas == updateDeploymentReplicas {
							return true, nil
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment has been deleted from member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for deployment(%s/%s) disappear on cluster(%s)", deploymentNamespace, deploymentName, cluster.Name)
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
								return true, nil
							}
							return false, err
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})
	})

	ginkgo.Context("Service propagation testing", func() {
		policyNamespace := testNamespace
		policyName := serviceNamePrefix + rand.String(RandomStrLength)
		serviceNamespace := policyNamespace
		serviceName := policyName

		service := helper.NewService(serviceNamespace, serviceName)
		policy := helper.NewPolicyWithSingleService(policyNamespace, policyName, service, clusterNames)

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

		ginkgo.It("service propagation testing", func() {
			ginkgo.By(fmt.Sprintf("creating service(%s/%s)", serviceNamespace, serviceName), func() {
				_, err := kubeClient.CoreV1().Services(serviceNamespace).Create(context.TODO(), service, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if service present on member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for service(%s/%s) present on cluster(%s)", serviceNamespace, serviceName, cluster.Name)
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.CoreV1().Services(serviceNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By("updating service", func() {
				patch := []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/spec/ports/0/port",
						"value": updateServicePort,
					},
				}
				bytes, err := json.Marshal(patch)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				_, err = kubeClient.CoreV1().Services(serviceNamespace).Patch(context.TODO(), serviceName, types.JSONPatchType, bytes, metav1.PatchOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if update has been synced to member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for service(%s/%s) synced on cluster(%s)", serviceNamespace, serviceName, cluster.Name)
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						service, err := clusterClient.CoreV1().Services(serviceNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
						if err != nil {
							return false, err
						}

						if service.Spec.Ports[0].Port == updateServicePort {
							return true, nil
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By(fmt.Sprintf("removing service(%s/%s)", serviceNamespace, serviceName), func() {
				err := kubeClient.CoreV1().Services(serviceNamespace).Delete(context.TODO(), serviceName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if service has been deleted from member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for service(%s/%s) disappear on cluster(%s)", serviceNamespace, serviceName, cluster.Name)
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.CoreV1().Services(serviceNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
								return true, nil
							}
							return false, err
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})
	})
})
