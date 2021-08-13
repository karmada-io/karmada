package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var (
	serviceExportClusterName = "member1"
	serviceImportClusterName = "member2"

	serviceExportResource = "serviceexports"
	serviceImportResource = "serviceimports"

	replicaCount    = int32(1)
	helloDeployment = appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicaCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "hello",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "hello"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "hello-tcp",
							Image: "busybox",
							Args:  []string{"nc", "-lk", "-p", "42", "-v", "-e", "echo", "hello"},
						},
						{
							Name:  "hello-udp",
							Image: "busybox",
							Args:  []string{"nc", "-lk", "-p", "42", "-u", "-v", "-e", "echo", "hello"},
						},
					},
				},
			},
		},
	}
	helloService = corev1.Service{
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "hello",
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "tcp",
					Port:     42,
					Protocol: corev1.ProtocolTCP,
				},
				{
					Name:     "udp",
					Port:     42,
					Protocol: corev1.ProtocolUDP,
				},
			},
		},
	}
	helloServiceExport = mcsv1alpha1.ServiceExport{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "multicluster.x-k8s.io/v1alpha1",
			Kind:       "ServiceExport",
		},
	}
	helloServiceImport = mcsv1alpha1.ServiceImport{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "multicluster.x-k8s.io/v1alpha1",
			Kind:       "ServiceImport",
		},
		Spec: mcsv1alpha1.ServiceImportSpec{
			Type: mcsv1alpha1.ClusterSetIP,
			Ports: []mcsv1alpha1.ServicePort{
				{
					Name:     "tcp",
					Port:     42,
					Protocol: corev1.ProtocolTCP,
				},
				{
					Name:     "udp",
					Port:     42,
					Protocol: corev1.ProtocolUDP,
				},
			},
		},
	}
	requestPod = corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "request",
					Image: "busybox",
					Args:  []string{"nc"},
				},
			},
		},
	}
)

func getPrepareInfo() (serviceExport mcsv1alpha1.ServiceExport, serviceImport mcsv1alpha1.ServiceImport, exportPolicy,
	importPolicy *policyv1alpha1.PropagationPolicy, deployment appsv1.Deployment, service corev1.Service) {
	helloNamespace := testNamespace
	helloName := fmt.Sprintf("hello-%s", rand.String(RandomStrLength))

	serviceExport = helloServiceExport
	serviceExport.Namespace = helloNamespace
	serviceExport.Name = helloName

	serviceImport = helloServiceImport
	serviceImport.Namespace = helloNamespace
	serviceImport.Name = helloName

	exportPolicyNamespace := serviceExport.Namespace
	exportPolicyName := fmt.Sprintf("export-%s-policy", serviceExport.Name)
	exportPolicy = testhelper.NewPropagationPolicy(exportPolicyNamespace, exportPolicyName, []policyv1alpha1.ResourceSelector{
		{
			APIVersion: serviceExport.APIVersion,
			Kind:       serviceExport.Kind,
			Name:       serviceExport.Name,
			Namespace:  serviceExport.Namespace,
		},
	}, policyv1alpha1.Placement{
		ClusterAffinity: &policyv1alpha1.ClusterAffinity{
			ClusterNames: []string{serviceExportClusterName},
		},
	})

	importPolicyNamespace := serviceImport.Namespace
	importPolicyName := fmt.Sprintf("import-%s-policy", serviceImport.Name)
	importPolicy = testhelper.NewPropagationPolicy(importPolicyNamespace, importPolicyName, []policyv1alpha1.ResourceSelector{
		{
			APIVersion: serviceImport.APIVersion,
			Kind:       serviceImport.Kind,
			Name:       serviceImport.Name,
			Namespace:  serviceImport.Namespace,
		},
	}, policyv1alpha1.Placement{
		ClusterAffinity: &policyv1alpha1.ClusterAffinity{
			ClusterNames: []string{serviceImportClusterName},
		},
	})

	deployment = helloDeployment
	deployment.Namespace = helloNamespace
	deployment.Name = helloName

	service = helloService
	service.Namespace = helloNamespace
	service.Name = helloName

	return
}

var _ = ginkgo.Describe("[MCS] Multi-Cluster Service testing", func() {
	serviceExportPolicyName := fmt.Sprintf("%s-%s-policy", serviceExportResource, rand.String(RandomStrLength))
	serviceImportPolicyName := fmt.Sprintf("%s-%s-policy", serviceImportResource, rand.String(RandomStrLength))

	serviceExportPolicy := testhelper.NewClusterPropagationPolicy(serviceExportPolicyName, []policyv1alpha1.ResourceSelector{
		{
			APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
			Kind:       util.CRDKind,
			Name:       fmt.Sprintf("%s.%s", serviceExportResource, mcsv1alpha1.GroupName),
		},
	}, policyv1alpha1.Placement{
		ClusterAffinity: &policyv1alpha1.ClusterAffinity{
			ClusterNames: clusterNames,
		},
	})
	serviceImportPolicy := testhelper.NewClusterPropagationPolicy(serviceImportPolicyName, []policyv1alpha1.ResourceSelector{
		{
			APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
			Kind:       util.CRDKind,
			Name:       fmt.Sprintf("%s.%s", serviceImportResource, mcsv1alpha1.GroupName),
		},
	}, policyv1alpha1.Placement{
		ClusterAffinity: &policyv1alpha1.ClusterAffinity{
			ClusterNames: clusterNames,
		},
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By(fmt.Sprintf("Create ClusterPropagationPolicy(%s) to Propagation ServiceExport CRD", serviceExportPolicy.Name), func() {
			_, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Create(context.TODO(), serviceExportPolicy, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.By(fmt.Sprintf("Create ClusterPropagationPolicy(%s) to Propagation ServiceImport CRD", serviceImportPolicy.Name), func() {
			_, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Create(context.TODO(), serviceImportPolicy, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.By("Wait ServiceExport CRD present on member clusters", func() {
			err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
				clusters, err := fetchClusters(karmadaClient)
				if err != nil {
					return false, err
				}
				for _, cluster := range clusters {
					if !helper.IsAPIEnabled(cluster.Status.APIEnablements, mcsv1alpha1.GroupVersion.String(), util.ServiceExportKind) {
						klog.Infof("Waiting for CRD(%s) present on member cluster(%s)", util.ServiceExportKind, cluster.Name)
						return false, nil
					}
				}
				return true, nil
			})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.By("Wait ServiceImport CRD present on member clusters", func() {
			err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
				clusters, err := fetchClusters(karmadaClient)
				if err != nil {
					return false, err
				}
				for _, cluster := range clusters {
					if !helper.IsAPIEnabled(cluster.Status.APIEnablements, mcsv1alpha1.GroupVersion.String(), util.ServiceImportKind) {
						klog.Infof("Waiting for CRD(%s) present on member cluster(%s)", util.ServiceImportKind, cluster.Name)
						return false, nil
					}
				}
				return true, nil
			})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})
	})

	ginkgo.AfterEach(func() {
		ginkgo.By(fmt.Sprintf("Delete ClusterPropagationPolicy %s", serviceExportPolicy.Name), func() {
			err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Delete(context.TODO(), serviceExportPolicy.Name, metav1.DeleteOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.By(fmt.Sprintf("Delete ClusterPropagationPolicy %s", serviceImportPolicy.Name), func() {
			err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Delete(context.TODO(), serviceImportPolicy.Name, metav1.DeleteOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})
		// Now the deletion of ClusterPropagationPolicy will not cause the deletion of related binding and workload on member clusters,
		// so we do not need to wait the disappearance of ServiceExport CRD and ServiceImport CRD
	})

	ginkgo.Context("Connectivity testing", func() {
		serviceExport, serviceImport, exportPolicy, importPolicy, demoDeployment, demoService := getPrepareInfo()

		ginkgo.BeforeEach(func() {
			exportClusterClient := getClusterClient(serviceExportClusterName)
			gomega.Expect(exportClusterClient).ShouldNot(gomega.BeNil())

			ginkgo.By(fmt.Sprintf("Create Deployment(%s/%s) in %s cluster", demoDeployment.Namespace, demoDeployment.Name, serviceExportClusterName), func() {
				_, err := exportClusterClient.AppsV1().Deployments(demoDeployment.Namespace).Create(context.TODO(), &demoDeployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Create Service(%s/%s) in %s cluster", demoService.Namespace, demoService.Name, serviceExportClusterName), func() {
				_, err := exportClusterClient.CoreV1().Services(demoService.Namespace).Create(context.TODO(), &demoService, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Wait Service(%s/%s)'s EndpointSlice exist in %s cluster", demoService.Namespace, demoService.Name, serviceExportClusterName), func() {
				gomega.Eventually(func() int {
					endpointSlices, err := exportClusterClient.DiscoveryV1beta1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set{discoveryv1beta1.LabelServiceName: demoService.Name}.AsSelector().String(),
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					epNums := 0
					for _, slice := range endpointSlices.Items {
						epNums += len(slice.Endpoints)
					}
					return epNums
				}, pollTimeout, pollInterval).Should(gomega.Equal(1))
			})
		})

		ginkgo.AfterEach(func() {
			exportClusterClient := getClusterClient(serviceExportClusterName)
			gomega.Expect(exportClusterClient).ShouldNot(gomega.BeNil())

			ginkgo.By(fmt.Sprintf("Delete Deployment(%s/%s) in %s cluster", demoDeployment.Namespace, demoDeployment.Name, serviceExportClusterName), func() {
				err := exportClusterClient.AppsV1().Deployments(demoDeployment.Namespace).Delete(context.TODO(), demoDeployment.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Delete Service(%s/%s) in %s cluster", demoService.Namespace, demoService.Name, serviceExportClusterName), func() {
				err := exportClusterClient.CoreV1().Services(demoService.Namespace).Delete(context.TODO(), demoService.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("Export Service from source-clusters, import Service to destination-clusters", func() {
			importClusterClient := getClusterClient(serviceImportClusterName)
			gomega.Expect(importClusterClient).ShouldNot(gomega.BeNil())

			ginkgo.By(fmt.Sprintf("Create ServiceExport(%s/%s)", serviceExport.Namespace, serviceExport.Name), func() {
				err := controlPlaneClient.Create(context.TODO(), &serviceExport)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Create PropagationPolicy(%s/%s) to propagate ServiceExport", exportPolicy.Namespace, exportPolicy.Name), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(exportPolicy.Namespace).Create(context.TODO(), exportPolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Wait EndpointSlices collected to namespace(%s) in controller-plane", demoService.Namespace), func() {
				gomega.Eventually(func() int {
					endpointSlices, err := kubeClient.DiscoveryV1beta1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set{discoveryv1beta1.LabelServiceName: names.GenerateDerivedServiceName(demoService.Name)}.AsSelector().String(),
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					epNums := 0
					for _, slice := range endpointSlices.Items {
						epNums += len(slice.Endpoints)
					}
					return epNums
				}, pollTimeout, pollInterval).Should(gomega.Equal(1))
			})

			ginkgo.By(fmt.Sprintf("Create ServiceImport(%s/%s)", serviceImport.Namespace, serviceImport.Name), func() {
				err := controlPlaneClient.Create(context.TODO(), &serviceImport)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Create PropagationPolicy(%s/%s) to propagate ServiveImport", importPolicy.Namespace, importPolicy.Name), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(importPolicy.Namespace).Create(context.TODO(), importPolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Wait derived-service(%s/%s) exist in %s cluster", demoService.Namespace, names.GenerateDerivedServiceName(demoService.Name), serviceImportClusterName), func() {
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					_, err = importClusterClient.CoreV1().Services(demoService.Namespace).Get(context.TODO(), names.GenerateDerivedServiceName(demoService.Name), metav1.GetOptions{})
					if err != nil {
						if errors.IsNotFound(err) {
							return false, nil
						}
						return false, err
					}
					return true, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Wait EndpointSlices have been imported to %s cluster", serviceImportClusterName), func() {
				gomega.Eventually(func() int {
					endpointSlices, err := importClusterClient.DiscoveryV1beta1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set{discoveryv1beta1.LabelServiceName: names.GenerateDerivedServiceName(demoService.Name)}.AsSelector().String(),
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					epNums := 0
					for _, slice := range endpointSlices.Items {
						epNums += len(slice.Endpoints)
					}
					return epNums
				}, pollTimeout, pollInterval).Should(gomega.Equal(1))
			})

			ginkgo.By("TCP connects across clusters using the ClusterIP", func() {
				derivedSvc, err := importClusterClient.CoreV1().Services(demoService.Namespace).Get(context.TODO(), names.GenerateDerivedServiceName(demoService.Name), metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				pod := requestPod
				pod.Name = fmt.Sprintf("request-%s", rand.String(RandomStrLength))
				pod.Spec.Containers[0].Args = []string{"nc", derivedSvc.Spec.ClusterIP, "42"}
				_, err = importClusterClient.CoreV1().Pods(demoService.Namespace).Create(context.TODO(), &pod, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Eventually(func() (string, error) {
					return podLogs(context.TODO(), importClusterClient, demoService.Namespace, pod.Name)
				}, pollTimeout, pollInterval).Should(gomega.Equal("hello\n"))
			})

			ginkgo.By("UDP connects across clusters using the ClusterIP", func() {
				derivedSvc, err := importClusterClient.CoreV1().Services(demoService.Namespace).Get(context.TODO(), names.GenerateDerivedServiceName(demoService.Name), metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				pod := requestPod
				pod.Name = fmt.Sprintf("request-%s", rand.String(RandomStrLength))
				pod.Spec.Containers[0].Args = []string{"sh", "-c", fmt.Sprintf("echo hi | nc -u %s 42", derivedSvc.Spec.ClusterIP)}
				_, err = importClusterClient.CoreV1().Pods(demoService.Namespace).Create(context.TODO(), &pod, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Eventually(func() (string, error) {
					return podLogs(context.TODO(), importClusterClient, demoService.Namespace, pod.Name)
				}, pollTimeout, pollInterval).Should(gomega.Equal("hello\n"))
			})

			ginkgo.By("Cleanup", func() {
				err := controlPlaneClient.Delete(context.TODO(), &serviceExport)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = karmadaClient.PolicyV1alpha1().PropagationPolicies(exportPolicy.Namespace).Delete(context.TODO(), exportPolicy.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = controlPlaneClient.Delete(context.TODO(), &serviceImport)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = karmadaClient.PolicyV1alpha1().PropagationPolicies(importPolicy.Namespace).Delete(context.TODO(), importPolicy.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})

	ginkgo.Context("EndpointSlices change testing", func() {
		serviceExport, serviceImport, exportPolicy, importPolicy, demoDeployment, demoService := getPrepareInfo()

		ginkgo.BeforeEach(func() {
			exportClusterClient := getClusterClient(serviceExportClusterName)
			gomega.Expect(exportClusterClient).ShouldNot(gomega.BeNil())

			ginkgo.By(fmt.Sprintf("Create Deployment(%s/%s) in %s cluster", demoDeployment.Namespace, demoDeployment.Name, serviceExportClusterName), func() {
				_, err := exportClusterClient.AppsV1().Deployments(demoDeployment.Namespace).Create(context.TODO(), &demoDeployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Create Service(%s/%s) in %s cluster", demoService.Namespace, demoService.Name, serviceExportClusterName), func() {
				_, err := exportClusterClient.CoreV1().Services(demoService.Namespace).Create(context.TODO(), &demoService, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Wait Service(%s/%s)'s EndpointSlice exist in %s cluster", demoService.Namespace, demoService.Name, serviceExportClusterName), func() {
				gomega.Eventually(func() int {
					endpointSlices, err := exportClusterClient.DiscoveryV1beta1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set{discoveryv1beta1.LabelServiceName: demoService.Name}.AsSelector().String(),
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					epNums := 0
					for _, slice := range endpointSlices.Items {
						epNums += len(slice.Endpoints)
					}
					return epNums
				}, pollTimeout, pollInterval).Should(gomega.Equal(1))
			})
		})

		ginkgo.AfterEach(func() {
			exportClusterClient := getClusterClient(serviceExportClusterName)
			gomega.Expect(exportClusterClient).ShouldNot(gomega.BeNil())

			ginkgo.By(fmt.Sprintf("Delete Deployment(%s/%s) in %s cluster", demoDeployment.Namespace, demoDeployment.Name, serviceExportClusterName), func() {
				err := exportClusterClient.AppsV1().Deployments(demoDeployment.Namespace).Delete(context.TODO(), demoDeployment.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Delete Service(%s/%s) in %s cluster", demoService.Namespace, demoService.Name, serviceExportClusterName), func() {
				err := exportClusterClient.CoreV1().Services(demoService.Namespace).Delete(context.TODO(), demoService.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("Update Deployment's replicas", func() {
			exportClusterClient := getClusterClient(serviceExportClusterName)
			gomega.Expect(exportClusterClient).ShouldNot(gomega.BeNil())
			importClusterClient := getClusterClient(serviceImportClusterName)
			gomega.Expect(importClusterClient).ShouldNot(gomega.BeNil())

			ginkgo.By(fmt.Sprintf("Create ServiceExport(%s/%s)", serviceExport.Namespace, serviceExport.Name), func() {
				err := controlPlaneClient.Create(context.TODO(), &serviceExport)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Create PropagationPolicy(%s/%s)", exportPolicy.Namespace, exportPolicy.Name), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(exportPolicy.Namespace).Create(context.TODO(), exportPolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Wait EndpointSlices collected to namespace(%s) in controller-plane", demoService.Namespace), func() {
				gomega.Eventually(func() int {
					endpointSlices, err := kubeClient.DiscoveryV1beta1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set{discoveryv1beta1.LabelServiceName: names.GenerateDerivedServiceName(demoService.Name)}.AsSelector().String(),
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					epNums := 0
					for _, slice := range endpointSlices.Items {
						epNums += len(slice.Endpoints)
					}
					return epNums
				}, pollTimeout, pollInterval).Should(gomega.Equal(1))
			})

			ginkgo.By(fmt.Sprintf("Create ServiceImport(%s/%s)", serviceImport.Namespace, serviceImport.Name), func() {
				err := controlPlaneClient.Create(context.TODO(), &serviceImport)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Create PropagationPolicy(%s/%s) to propagate ServiveImport", importPolicy.Namespace, importPolicy.Name), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(importPolicy.Namespace).Create(context.TODO(), importPolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Wait EndpointSlice exist in %s cluster", serviceImportClusterName), func() {
				gomega.Eventually(func() int {
					endpointSlices, err := importClusterClient.DiscoveryV1beta1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set{discoveryv1beta1.LabelServiceName: names.GenerateDerivedServiceName(demoService.Name)}.AsSelector().String(),
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					epNums := 0
					for _, slice := range endpointSlices.Items {
						epNums += len(slice.Endpoints)
					}
					return epNums
				}, pollTimeout, pollInterval).Should(gomega.Equal(1))
			})

			ginkgo.By(fmt.Sprintf("Update Deployment's replicas in %s cluster", serviceExportClusterName), func() {
				updateReplicaCount := int32(2)
				demoDeployment.Spec.Replicas = &updateReplicaCount

				_, err := exportClusterClient.AppsV1().Deployments(demoDeployment.Namespace).Update(context.TODO(), &demoDeployment, metav1.UpdateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Wait EndpointSlice update in %s cluster", serviceImportClusterName), func() {
				gomega.Eventually(func() int {
					endpointSlices, err := importClusterClient.DiscoveryV1beta1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set{discoveryv1beta1.LabelServiceName: names.GenerateDerivedServiceName(demoService.Name)}.AsSelector().String(),
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					epNums := 0
					for _, slice := range endpointSlices.Items {
						epNums += len(slice.Endpoints)
					}
					return epNums
				}, pollTimeout, pollInterval).Should(gomega.Equal(2))
			})

			ginkgo.By("Cleanup", func() {
				err := controlPlaneClient.Delete(context.TODO(), &serviceExport)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = karmadaClient.PolicyV1alpha1().PropagationPolicies(exportPolicy.Namespace).Delete(context.TODO(), exportPolicy.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = controlPlaneClient.Delete(context.TODO(), &serviceImport)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = karmadaClient.PolicyV1alpha1().PropagationPolicies(importPolicy.Namespace).Delete(context.TODO(), importPolicy.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})
})
