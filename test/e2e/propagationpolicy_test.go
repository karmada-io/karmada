package e2e

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

// BasicPropagation focus on basic propagation functionality testing.
var _ = ginkgo.Describe("[BasicPropagation] basic propagation testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := testNamespace
		deploymentName := policyName

		deployment := testhelper.NewDeployment(deploymentNamespace, deploymentName)
		policy := testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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

		ginkgo.It("deployment propagation testing", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			framework.WaitDeploymentPresentOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)

			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)
			ginkgo.By("check if update has been synced to member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for deployment(%s/%s) synced on cluster(%s)", deploymentNamespace, deploymentName, cluster.Name)
					framework.WaitDeploymentPresentOnClusterFitWith(clusterClient, deploymentNamespace, deploymentName,
						func(deployment *appsv1.Deployment) bool {
							return *deployment.Spec.Replicas == updateDeploymentReplicas
						})
				}
			})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	ginkgo.Context("Service propagation testing", func() {
		policyNamespace := testNamespace
		policyName := serviceNamePrefix + rand.String(RandomStrLength)
		serviceNamespace := policyNamespace
		serviceName := policyName

		service := testhelper.NewService(serviceNamespace, serviceName)
		policy := testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: service.APIVersion,
				Kind:       service.Kind,
				Name:       service.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})

		ginkgo.It("service propagation testing", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)

			ginkgo.By(fmt.Sprintf("creating service(%s/%s)", serviceNamespace, serviceName), func() {
				_, err := kubeClient.CoreV1().Services(serviceNamespace).Create(context.TODO(), service, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if service present on member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for service(%s/%s) present on cluster(%s)", serviceNamespace, serviceName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.CoreV1().Services(serviceNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
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
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for service(%s/%s) synced on cluster(%s)", serviceNamespace, serviceName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
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
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for service(%s/%s) disappear on cluster(%s)", serviceNamespace, serviceName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.CoreV1().Services(serviceNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return true, nil
							}
							return false, err
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	ginkgo.Context("Pod propagation testing", func() {
		policyNamespace := testNamespace
		policyName := podNamePrefix + rand.String(RandomStrLength)
		podNamespace := policyNamespace
		podName := policyName

		pod := testhelper.NewPod(podNamespace, podName)
		policy := testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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

		ginkgo.It("pod propagation testing", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)

			ginkgo.By(fmt.Sprintf("creating pod(%s/%s)", podNamespace, podName), func() {
				_, err := kubeClient.CoreV1().Pods(podNamespace).Create(context.TODO(), pod, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if pod present on member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for pod(%s/%s) present on cluster(%s)", podNamespace, podName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By("updating pod", func() {
				patch := []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/spec/containers/0/image",
						"value": updatePodImage,
					},
				}
				bytes, err := json.Marshal(patch)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				_, err = kubeClient.CoreV1().Pods(podNamespace).Patch(context.TODO(), podName, types.JSONPatchType, bytes, metav1.PatchOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if update has been synced to member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for pod(%s/%s) synced on cluster(%s)", podNamespace, podName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						pod, err := clusterClient.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							return false, err
						}

						if pod.Spec.Containers[0].Image == updatePodImage {
							return true, nil
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By(fmt.Sprintf("removing pod(%s/%s)", podNamespace, podName), func() {
				err := kubeClient.CoreV1().Pods(podNamespace).Delete(context.TODO(), podName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if pod has been deleted from member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for pod(%s/%s) disappear on cluster(%s)", podNamespace, podName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return true, nil
							}
							return false, err
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	ginkgo.Context("NamespaceScoped CustomResource propagation testing", func() {
		crdGroup := fmt.Sprintf("example-%s.karmada.io", rand.String(RandomStrLength))
		randStr := rand.String(RandomStrLength)
		crdSpecNames := apiextensionsv1.CustomResourceDefinitionNames{
			Kind:     fmt.Sprintf("Foo%s", randStr),
			ListKind: fmt.Sprintf("Foo%sList", randStr),
			Plural:   fmt.Sprintf("foo%ss", randStr),
			Singular: fmt.Sprintf("foo%s", randStr),
		}
		crd := testhelper.NewCustomResourceDefinition(crdGroup, crdSpecNames, apiextensionsv1.NamespaceScoped)
		crdPolicy := testhelper.NewClusterPropagationPolicy(crd.Name, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: crd.APIVersion,
				Kind:       crd.Kind,
				Name:       crd.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})

		crNamespace := testNamespace
		crName := crdNamePrefix + rand.String(RandomStrLength)
		crGVR := schema.GroupVersionResource{Group: crd.Spec.Group, Version: "v1alpha1", Resource: crd.Spec.Names.Plural}

		crAPIVersion := fmt.Sprintf("%s/%s", crd.Spec.Group, "v1alpha1")
		cr := testhelper.NewCustomResource(crAPIVersion, crd.Spec.Names.Kind, crNamespace, crName)
		crPolicy := testhelper.NewPropagationPolicy(crNamespace, crName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: crAPIVersion,
				Kind:       crd.Spec.Names.Kind,
				Name:       crName,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})

		ginkgo.It("namespaceScoped cr propagation testing", func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, crdPolicy)
			framework.CreateCRD(dynamicClient, crd)
			framework.GetCRD(dynamicClient, crd.Name)
			framework.WaitCRDPresentOnClusters(karmadaClient, framework.ClusterNames(),
				fmt.Sprintf("%s/%s", crd.Spec.Group, "v1alpha1"), crd.Spec.Names.Kind)

			framework.CreatePropagationPolicy(karmadaClient, crPolicy)

			ginkgo.By(fmt.Sprintf("creating cr(%s/%s)", crNamespace, crName), func() {
				_, err := dynamicClient.Resource(crGVR).Namespace(crNamespace).Create(context.TODO(), cr, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if cr present on member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster.Name)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for cr(%s/%s) present on cluster(%s)", crNamespace, crName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterDynamicClient.Resource(crGVR).Namespace(crNamespace).Get(context.TODO(), crName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By("updating cr", func() {
				patch := []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/spec/resource/namespace",
						"value": updateCRnamespace,
					},
				}
				bytes, err := json.Marshal(patch)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				_, err = dynamicClient.Resource(crGVR).Namespace(crNamespace).Patch(context.TODO(), crName, types.JSONPatchType, bytes, metav1.PatchOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if update has been synced to member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster.Name)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for cr(%s/%s) synced on cluster(%s)", crNamespace, crName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						cr, err := dynamicClient.Resource(crGVR).Namespace(crNamespace).Get(context.TODO(), crName, metav1.GetOptions{})
						if err != nil {
							return false, err
						}

						namespace, found, err := unstructured.NestedString(cr.Object, "spec", "resource", "namespace")
						if err != nil || !found {
							return false, err
						}
						if namespace == updateCRnamespace {
							return true, nil
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By(fmt.Sprintf("removing cr(%s/%s)", crNamespace, crName), func() {
				err := dynamicClient.Resource(crGVR).Namespace(crNamespace).Delete(context.TODO(), crName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if cr has been deleted from member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster.Name)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for cr(%s/%s) disappear on cluster(%s)", crNamespace, crName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = dynamicClient.Resource(crGVR).Namespace(crNamespace).Get(context.TODO(), crName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return true, nil
							}
							return false, err
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			framework.RemovePropagationPolicy(karmadaClient, crPolicy.Namespace, crPolicy.Name)

			framework.RemoveClusterPropagationPolicy(karmadaClient, crdPolicy.Name)
			framework.RemoveCRD(dynamicClient, crd.Name)
		})
	})

	ginkgo.Context("Job propagation testing", func() {
		policyNamespace := testNamespace
		policyName := jobNamePrefix + rand.String(RandomStrLength)
		jobNamespace := testNamespace
		jobName := policyName

		job := testhelper.NewJob(jobNamespace, jobName)
		policy := testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: job.APIVersion,
				Kind:       job.Kind,
				Name:       job.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})

		ginkgo.It("job propagation testing", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)

			ginkgo.By(fmt.Sprintf("creating job(%s/%s)", jobNamespace, jobName), func() {
				_, err := kubeClient.BatchV1().Jobs(jobNamespace).Create(context.TODO(), job, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if job present on member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for job(%s/%s) present on cluster(%s)", jobNamespace, jobName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.BatchV1().Jobs(jobNamespace).Get(context.TODO(), jobName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By(fmt.Sprintf("updating job(%s/%s)", jobNamespace, jobName), func() {
				patch := []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/spec/backoffLimit",
						"value": pointer.Int32Ptr(updateBackoffLimit),
					},
				}
				bytes, err := json.Marshal(patch)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				_, err = kubeClient.BatchV1().Jobs(jobNamespace).Patch(context.TODO(), jobName, types.JSONPatchType, bytes, metav1.PatchOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if update has been synced to member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for job(%s/%s) synced on cluster(%s)", jobNamespace, jobName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						newJob, err := clusterClient.BatchV1().Jobs(jobNamespace).Get(context.TODO(), jobName, metav1.GetOptions{})
						if err != nil {
							return false, err
						}

						if *newJob.Spec.BackoffLimit == updateBackoffLimit {
							return true, nil
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By(fmt.Sprintf("removing job(%s/%s)", jobNamespace, jobName), func() {
				foregroundDelete := metav1.DeletePropagationForeground
				err := kubeClient.BatchV1().Jobs(jobNamespace).Delete(context.TODO(), jobName, metav1.DeleteOptions{PropagationPolicy: &foregroundDelete})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if job has been deleted from member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for job(%s/%s) disappear on cluster(%s)", jobNamespace, jobName, cluster.Name)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.BatchV1().Jobs(jobNamespace).Get(context.TODO(), jobName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return true, nil
							}
							return false, err
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})
})
