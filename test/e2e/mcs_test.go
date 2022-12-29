package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
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

var _ = ginkgo.Describe("Multi-Cluster Service testing", func() {
	var serviceExportPolicyName, serviceImportPolicyName string
	var serviceExportPolicy, serviceImportPolicy *policyv1alpha1.ClusterPropagationPolicy
	var serviceExport mcsv1alpha1.ServiceExport
	var serviceImport mcsv1alpha1.ServiceImport
	var exportPolicy, importPolicy *policyv1alpha1.PropagationPolicy
	var demoDeployment appsv1.Deployment
	var demoService corev1.Service

	ginkgo.BeforeEach(func() {
		serviceExportPolicyName = fmt.Sprintf("%s-%s-policy", serviceExportResource, rand.String(RandomStrLength))
		serviceImportPolicyName = fmt.Sprintf("%s-%s-policy", serviceImportResource, rand.String(RandomStrLength))

		serviceExportPolicy = testhelper.NewClusterPropagationPolicy(serviceExportPolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
				Kind:       util.CRDKind,
				Name:       fmt.Sprintf("%s.%s", serviceExportResource, mcsv1alpha1.GroupName),
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})
		serviceImportPolicy = testhelper.NewClusterPropagationPolicy(serviceImportPolicyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
				Kind:       util.CRDKind,
				Name:       fmt.Sprintf("%s.%s", serviceImportResource, mcsv1alpha1.GroupName),
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})

		serviceExport, serviceImport, exportPolicy, importPolicy, demoDeployment, demoService = getPrepareInfo()
	})

	ginkgo.BeforeEach(func() {
		framework.CreateClusterPropagationPolicy(karmadaClient, serviceExportPolicy)
		framework.CreateClusterPropagationPolicy(karmadaClient, serviceImportPolicy)
		framework.WaitCRDPresentOnClusters(karmadaClient, framework.ClusterNames(), mcsv1alpha1.GroupVersion.String(), util.ServiceExportKind)
		framework.WaitCRDPresentOnClusters(karmadaClient, framework.ClusterNames(), mcsv1alpha1.GroupVersion.String(), util.ServiceImportKind)
		ginkgo.DeferCleanup(func() {
			framework.RemoveClusterPropagationPolicy(karmadaClient, serviceImportPolicy.Name)
			framework.RemoveClusterPropagationPolicy(karmadaClient, serviceExportPolicy.Name)

			// Now the deletion of ClusterPropagationPolicy will not cause the deletion of related binding and workload on member clusters,
			// so we do not need to wait the disappearance of ServiceExport CRD and ServiceImport CRD
		})
	})

	ginkgo.BeforeEach(func() {
		exportClusterClient := framework.GetClusterClient(serviceExportClusterName)
		gomega.Expect(exportClusterClient).ShouldNot(gomega.BeNil())

		klog.Infof("Create Deployment(%s/%s) in %s cluster", demoDeployment.Namespace, demoDeployment.Name, serviceExportClusterName)
		framework.CreateDeployment(exportClusterClient, &demoDeployment)

		klog.Infof("Create Service(%s/%s) in %s cluster", demoService.Namespace, demoService.Name, serviceExportClusterName)
		framework.CreateService(exportClusterClient, &demoService)

		ginkgo.By(fmt.Sprintf("Wait Service(%s/%s)'s EndpointSlice exist in %s cluster", demoService.Namespace, demoService.Name, serviceExportClusterName), func() {
			gomega.Eventually(func(g gomega.Gomega) (int, error) {
				endpointSlices, err := exportClusterClient.DiscoveryV1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labels.Set{discoveryv1.LabelServiceName: demoService.Name}.AsSelector().String(),
				})
				g.Expect(err).NotTo(gomega.HaveOccurred())

				epNums := 0
				for _, slice := range endpointSlices.Items {
					epNums += len(slice.Endpoints)
				}
				return epNums, nil
			}, pollTimeout, pollInterval).Should(gomega.Equal(1))
		})

		ginkgo.DeferCleanup(func() {
			klog.Infof("Delete Deployment(%s/%s) in %s cluster", demoDeployment.Namespace, demoDeployment.Name, serviceExportClusterName)
			framework.RemoveDeployment(exportClusterClient, demoDeployment.Namespace, demoDeployment.Name)

			klog.Infof("Delete Service(%s/%s) in %s cluster", demoService.Namespace, demoService.Name, serviceExportClusterName)
			framework.RemoveService(exportClusterClient, demoService.Namespace, demoService.Name)
		})
	})

	ginkgo.AfterEach(func() {
		ginkgo.By("Cleanup", func() {
			err := controlPlaneClient.Delete(context.TODO(), &serviceExport)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			err = controlPlaneClient.Delete(context.TODO(), &serviceImport)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		framework.RemovePropagationPolicy(karmadaClient, exportPolicy.Namespace, exportPolicy.Name)
		framework.RemovePropagationPolicy(karmadaClient, importPolicy.Namespace, importPolicy.Name)
	})

	ginkgo.Context("Connectivity testing", func() {
		ginkgo.It("Export Service from source-clusters, import Service to destination-clusters", func() {
			importClusterClient := framework.GetClusterClient(serviceImportClusterName)
			gomega.Expect(importClusterClient).ShouldNot(gomega.BeNil())

			ginkgo.By(fmt.Sprintf("Create ServiceExport(%s/%s)", serviceExport.Namespace, serviceExport.Name), func() {
				err := controlPlaneClient.Create(context.TODO(), &serviceExport)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			framework.CreatePropagationPolicy(karmadaClient, exportPolicy)

			ginkgo.By(fmt.Sprintf("Wait EndpointSlices collected to namespace(%s) in controller-plane", demoService.Namespace), func() {
				gomega.Eventually(func(g gomega.Gomega) (int, error) {
					endpointSlices, err := kubeClient.DiscoveryV1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set{discoveryv1.LabelServiceName: names.GenerateDerivedServiceName(demoService.Name)}.AsSelector().String(),
					})
					g.Expect(err).NotTo(gomega.HaveOccurred())

					epNums := 0
					for _, slice := range endpointSlices.Items {
						epNums += len(slice.Endpoints)
					}
					return epNums, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(1))
			})

			ginkgo.By(fmt.Sprintf("Create ServiceImport(%s/%s)", serviceImport.Namespace, serviceImport.Name), func() {
				err := controlPlaneClient.Create(context.TODO(), &serviceImport)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			framework.CreatePropagationPolicy(karmadaClient, importPolicy)

			ginkgo.By(fmt.Sprintf("Wait derived-service(%s/%s) exist in %s cluster", demoService.Namespace, names.GenerateDerivedServiceName(demoService.Name), serviceImportClusterName), func() {
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					_, err = importClusterClient.CoreV1().Services(demoService.Namespace).Get(context.TODO(), names.GenerateDerivedServiceName(demoService.Name), metav1.GetOptions{})
					if err != nil {
						if apierrors.IsNotFound(err) {
							return false, nil
						}
						return false, err
					}
					return true, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Wait EndpointSlices have been imported to %s cluster", serviceImportClusterName), func() {
				gomega.Eventually(func(g gomega.Gomega) (int, error) {
					endpointSlices, err := importClusterClient.DiscoveryV1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set{discoveryv1.LabelServiceName: names.GenerateDerivedServiceName(demoService.Name)}.AsSelector().String(),
					})
					g.Expect(err).NotTo(gomega.HaveOccurred())

					epNums := 0
					for _, slice := range endpointSlices.Items {
						epNums += len(slice.Endpoints)
					}
					return epNums, nil
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
		})
	})

	ginkgo.Context("EndpointSlices change testing", func() {
		ginkgo.It("Update Deployment's replicas", func() {
			exportClusterClient := framework.GetClusterClient(serviceExportClusterName)
			gomega.Expect(exportClusterClient).ShouldNot(gomega.BeNil())
			importClusterClient := framework.GetClusterClient(serviceImportClusterName)
			gomega.Expect(importClusterClient).ShouldNot(gomega.BeNil())

			ginkgo.By(fmt.Sprintf("Create ServiceExport(%s/%s)", serviceExport.Namespace, serviceExport.Name), func() {
				err := controlPlaneClient.Create(context.TODO(), &serviceExport)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			framework.CreatePropagationPolicy(karmadaClient, exportPolicy)

			ginkgo.By(fmt.Sprintf("Wait EndpointSlices collected to namespace(%s) in controller-plane", demoService.Namespace), func() {
				gomega.Eventually(func(g gomega.Gomega) (int, error) {
					endpointSlices, err := kubeClient.DiscoveryV1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set{discoveryv1.LabelServiceName: names.GenerateDerivedServiceName(demoService.Name)}.AsSelector().String(),
					})
					g.Expect(err).NotTo(gomega.HaveOccurred())

					epNums := 0
					for _, slice := range endpointSlices.Items {
						epNums += len(slice.Endpoints)
					}
					return epNums, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(1))
			})

			ginkgo.By(fmt.Sprintf("Create ServiceImport(%s/%s)", serviceImport.Namespace, serviceImport.Name), func() {
				err := controlPlaneClient.Create(context.TODO(), &serviceImport)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			framework.CreatePropagationPolicy(karmadaClient, importPolicy)

			ginkgo.By(fmt.Sprintf("Wait EndpointSlice exist in %s cluster", serviceImportClusterName), func() {
				gomega.Eventually(func(g gomega.Gomega) (int, error) {
					endpointSlices, err := importClusterClient.DiscoveryV1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set{discoveryv1.LabelServiceName: names.GenerateDerivedServiceName(demoService.Name)}.AsSelector().String(),
					})
					g.Expect(err).NotTo(gomega.HaveOccurred())

					epNums := 0
					for _, slice := range endpointSlices.Items {
						epNums += len(slice.Endpoints)
					}
					return epNums, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(1))
			})

			klog.Infof("Update Deployment's replicas in %s cluster", serviceExportClusterName)
			framework.UpdateDeploymentReplicas(exportClusterClient, &demoDeployment, 2)

			ginkgo.By(fmt.Sprintf("Wait EndpointSlice update in %s cluster", serviceImportClusterName), func() {
				gomega.Eventually(func(g gomega.Gomega) (int, error) {
					endpointSlices, err := importClusterClient.DiscoveryV1().EndpointSlices(demoService.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set{discoveryv1.LabelServiceName: names.GenerateDerivedServiceName(demoService.Name)}.AsSelector().String(),
					})
					g.Expect(err).NotTo(gomega.HaveOccurred())

					epNums := 0
					for _, slice := range endpointSlices.Items {
						epNums += len(slice.Endpoints)
					}
					return epNums, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(2))
			})
		})
	})
})
