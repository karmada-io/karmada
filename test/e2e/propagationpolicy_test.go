package e2e

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/helper"
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
				ClusterNames: clusterNames,
			},
		})

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
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
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
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
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
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
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
				ClusterNames: clusterNames,
			},
		})

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
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
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
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
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
				ClusterNames: clusterNames,
			},
		})

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

		ginkgo.It("pod propagation testing", func() {
			ginkgo.By(fmt.Sprintf("creating pod(%s/%s)", podNamespace, podName), func() {
				_, err := kubeClient.CoreV1().Pods(podNamespace).Create(context.TODO(), pod, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if pod present on member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
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
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
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
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
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
				ClusterNames: clusterNames,
			},
		})
		crdGVR := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

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
				ClusterNames: clusterNames,
			},
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating crdPolicy(%s)", crdPolicy.Name), func() {
				_, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Create(context.TODO(), crdPolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("creating crd(%s)", crd.Name), func() {
				unstructObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(crd)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				_, err = dynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Create(context.TODO(), &unstructured.Unstructured{Object: unstructObj}, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("get crd(%s)", crd.Name), func() {
				_, err := dynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Get(context.TODO(), crd.Name, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			// Check CRD enablement from cluster objects instead of member clusters.
			// After CRD installed on member cluster, the cluster status controller takes at most cluster-status-update-frequency
			// time to collect the API list, before that the scheduler will filter out the cluster from scheduling.
			ginkgo.By("check if crd present on member clusters", func() {
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					clusters, err := fetchClusters(karmadaClient)
					if err != nil {
						return false, err
					}
					for _, cluster := range clusters {
						if !helper.IsAPIEnabled(cluster.Status.APIEnablements, crAPIVersion, crdSpecNames.Kind) {
							return false, nil
						}
					}
					return true, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating crPolicy(%s/%s)", crPolicy.Namespace, crPolicy.Name), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(crPolicy.Namespace).Create(context.TODO(), crPolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing crPolicy(%s/%s)", crPolicy.Namespace, crPolicy.Name), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(crPolicy.Namespace).Delete(context.TODO(), crPolicy.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing crdPolicy(%s)", crdPolicy.Name), func() {
				err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Delete(context.TODO(), crdPolicy.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("removing crd(%s)", crd.Name), func() {
				err := dynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Delete(context.TODO(), crd.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("namespaceScoped cr propagation testing", func() {
			ginkgo.By(fmt.Sprintf("creating cr(%s/%s)", crNamespace, crName), func() {
				_, err := dynamicClient.Resource(crGVR).Namespace(crNamespace).Create(context.TODO(), cr, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if cr present on member clusters", func() {
				for _, cluster := range clusters {
					clusterDynamicClient := getClusterDynamicClient(cluster.Name)
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
				for _, cluster := range clusters {
					clusterDynamicClient := getClusterDynamicClient(cluster.Name)
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
				for _, cluster := range clusters {
					clusterDynamicClient := getClusterDynamicClient(cluster.Name)
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
				ClusterNames: clusterNames,
			},
		})

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

		ginkgo.It("job propagation testing", func() {
			ginkgo.By(fmt.Sprintf("creating job(%s/%s)", jobNamespace, jobName), func() {
				_, err := kubeClient.BatchV1().Jobs(jobNamespace).Create(context.TODO(), job, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if job present on member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
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
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
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
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
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
		})
	})
})
