package e2e

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
		var policyNamespace, policyName string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = policyName

			deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment propagation testing", func() {
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return true
				})

			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return *deployment.Spec.Replicas == updateDeploymentReplicas
				})
		})
	})

	ginkgo.Context("Service propagation testing", func() {
		var policyNamespace, policyName string
		var serviceNamespace, serviceName string
		var service *corev1.Service
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = serviceNamePrefix + rand.String(RandomStrLength)
			serviceNamespace = policyNamespace
			serviceName = policyName

			service = testhelper.NewService(serviceNamespace, serviceName, corev1.ServiceTypeClusterIP)
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateService(kubeClient, service)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				framework.RemoveService(kubeClient, service.Namespace, service.Name)
				framework.WaitServiceDisappearOnClusters(framework.ClusterNames(), service.Namespace, service.Name)
			})
		})

		ginkgo.It("service propagation testing", func() {
			framework.WaitServicePresentOnClustersFitWith(framework.ClusterNames(), service.Namespace, service.Name,
				func(service *corev1.Service) bool {
					return true
				})

			patch := []map[string]interface{}{{"op": "replace", "path": "/spec/ports/0/port", "value": updateServicePort}}
			framework.UpdateServiceWithPatch(kubeClient, service.Namespace, service.Name, patch, types.JSONPatchType)

			framework.WaitServicePresentOnClustersFitWith(framework.ClusterNames(), service.Namespace, service.Name,
				func(service *corev1.Service) bool {
					return service.Spec.Ports[0].Port == updateServicePort
				})
		})
	})

	ginkgo.Context("Pod propagation testing", func() {
		var policyNamespace, policyName string
		var podNamespace, podName string
		var pod *corev1.Pod
		var policy *policyv1alpha1.PropagationPolicy
		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = podNamePrefix + rand.String(RandomStrLength)
			podNamespace = policyNamespace
			podName = policyName

			pod = testhelper.NewPod(podNamespace, podName)
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreatePod(kubeClient, pod)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				framework.RemovePod(kubeClient, pod.Namespace, pod.Name)
				framework.WaitPodDisappearOnClusters(framework.ClusterNames(), pod.Namespace, pod.Name)
			})
		})

		ginkgo.It("pod propagation testing", func() {
			framework.WaitPodPresentOnClustersFitWith(framework.ClusterNames(), pod.Namespace, pod.Name,
				func(pod *corev1.Pod) bool {
					return true
				})

			patch := []map[string]interface{}{{"op": "replace", "path": "/spec/containers/0/image", "value": updatePodImage}}
			framework.UpdatePodWithPatch(kubeClient, pod.Namespace, pod.Name, patch, types.JSONPatchType)

			framework.WaitPodPresentOnClustersFitWith(framework.ClusterNames(), pod.Namespace, pod.Name,
				func(pod *corev1.Pod) bool {
					return pod.Spec.Containers[0].Image == updatePodImage
				})
		})
	})

	ginkgo.Context("NamespaceScoped CustomResource propagation testing", func() {
		var crdGroup string
		var randStr string
		var crdSpecNames apiextensionsv1.CustomResourceDefinitionNames
		var crd *apiextensionsv1.CustomResourceDefinition
		var crdPolicy *policyv1alpha1.ClusterPropagationPolicy
		var crNamespace, crName string
		var crGVR schema.GroupVersionResource
		var crAPIVersion string
		var cr *unstructured.Unstructured
		var crPolicy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			crdGroup = fmt.Sprintf("example-%s.karmada.io", rand.String(RandomStrLength))
			randStr = rand.String(RandomStrLength)
			crdSpecNames = apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     fmt.Sprintf("Foo%s", randStr),
				ListKind: fmt.Sprintf("Foo%sList", randStr),
				Plural:   fmt.Sprintf("foo%ss", randStr),
				Singular: fmt.Sprintf("foo%s", randStr),
			}
			crd = testhelper.NewCustomResourceDefinition(crdGroup, crdSpecNames, apiextensionsv1.NamespaceScoped)
			crdPolicy = testhelper.NewClusterPropagationPolicy(crd.Name, []policyv1alpha1.ResourceSelector{
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

			crNamespace = testNamespace
			crName = crdNamePrefix + rand.String(RandomStrLength)
			crGVR = schema.GroupVersionResource{Group: crd.Spec.Group, Version: "v1alpha1", Resource: crd.Spec.Names.Plural}

			crAPIVersion = fmt.Sprintf("%s/%s", crd.Spec.Group, "v1alpha1")
			cr = testhelper.NewCustomResource(crAPIVersion, crd.Spec.Names.Kind, crNamespace, crName)
			crPolicy = testhelper.NewPropagationPolicy(crNamespace, crName, []policyv1alpha1.ResourceSelector{
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
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, crdPolicy)
			framework.CreateCRD(dynamicClient, crd)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, crdPolicy.Name)
				framework.RemoveCRD(dynamicClient, crd.Name)
			})
		})

		ginkgo.It("namespaceScoped cr propagation testing", func() {
			framework.GetCRD(dynamicClient, crd.Name)
			framework.WaitCRDPresentOnClusters(karmadaClient, framework.ClusterNames(),
				fmt.Sprintf("%s/%s", crd.Spec.Group, "v1alpha1"), crd.Spec.Names.Kind)

			framework.CreatePropagationPolicy(karmadaClient, crPolicy)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, crPolicy.Namespace, crPolicy.Name)
			})

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
		})
	})

	ginkgo.Context("Job propagation testing", func() {
		var policyNamespace, policyName string
		var jobNamespace, jobName string
		var job *batchv1.Job
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = jobNamePrefix + rand.String(RandomStrLength)
			jobNamespace = testNamespace
			jobName = policyName

			job = testhelper.NewJob(jobNamespace, jobName)
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateJob(kubeClient, job)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				framework.RemoveJob(kubeClient, job.Namespace, job.Name)
				framework.WaitJobDisappearOnClusters(framework.ClusterNames(), job.Namespace, job.Name)
			})
		})

		ginkgo.It("job propagation testing", func() {
			framework.WaitJobPresentOnClustersFitWith(framework.ClusterNames(), job.Namespace, job.Name,
				func(job *batchv1.Job) bool {
					return true
				})

			patch := []map[string]interface{}{{"op": "replace", "path": "/spec/backoffLimit", "value": pointer.Int32Ptr(updateBackoffLimit)}}
			bytes, err := json.Marshal(patch)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			framework.UpdateJobWithPatchBytes(kubeClient, job.Namespace, job.Name, bytes, types.JSONPatchType)

			framework.WaitJobPresentOnClustersFitWith(framework.ClusterNames(), job.Namespace, job.Name,
				func(job *batchv1.Job) bool {
					return *job.Spec.BackoffLimit == updateBackoffLimit
				})
		})
	})
})
