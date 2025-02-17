/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package base

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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var (
	serviceExportClusterName, serviceImportClusterName string

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
	exportPolicy = helper.NewPropagationPolicy(exportPolicyNamespace, exportPolicyName, []policyv1alpha1.ResourceSelector{
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
	importPolicy = helper.NewPropagationPolicy(importPolicyNamespace, importPolicyName, []policyv1alpha1.ResourceSelector{
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
		serviceExportClusterName = framework.ClusterNames()[0]
		serviceImportClusterName = framework.ClusterNames()[1]
		serviceExportPolicyName = fmt.Sprintf("%s-%s-policy", serviceExportResource, rand.String(RandomStrLength))
		serviceImportPolicyName = fmt.Sprintf("%s-%s-policy", serviceImportResource, rand.String(RandomStrLength))

		serviceExportPolicy = helper.NewClusterPropagationPolicy(serviceExportPolicyName, []policyv1alpha1.ResourceSelector{
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
		serviceImportPolicy = helper.NewClusterPropagationPolicy(serviceImportPolicyName, []policyv1alpha1.ResourceSelector{
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
				err := wait.PollUntilContextTimeout(context.TODO(), pollInterval, pollTimeout, true, func(ctx context.Context) (done bool, err error) {
					_, err = importClusterClient.CoreV1().Services(demoService.Namespace).Get(ctx, names.GenerateDerivedServiceName(demoService.Name), metav1.GetOptions{})
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

/*
MultiClusterService with CrossCluster type focus on syncing Services' EndpointSlice to other clusters.
Test Case Overview:

	case 1:
		ProviderClusters field is [member1], ConsumerClusters field is [member2]
		The endpointSlice in member1 will be synced to member1, with prefix member1-
	case 2:
		ProviderClusters field is [member1,member2], ConsumerClusters field is [member2]
		The endpointSlice in member1 will be synced to member1, with prefix member1-, and do not sync member2's endpointslice to member2 again
	case 3:
		fist, ProviderClusters field is [member1,member2], ConsumerClusters field is [member2]
		The endpointSlice in member1 will be synced to member1, with prefix member1-, and do not sync member2's endpointslice to member2 again
		then, ProviderClusters field changes to [member2], ConsumerClusters field is [member1]
		The endpointSlice in member2 will be synced to member1, with prefix member2-
	case 4:
		ProviderClusters field is empty, ConsumerClusters field is [member2]
		The endpointSlice in all member clusters(except member2) will be synced to member2
	case 5:
		ProviderClusters field is [member1], ConsumerClusters field is empty
		The endpointSlice in member1 will be synced to all member clusters(except member1)
	case 6:
		ProviderClusters field is empty, ConsumerClusters field is empty
		The endpointSlice in all member clusters will be synced to all member clusters
*/
var _ = ginkgo.Describe("CrossCluster MultiClusterService testing", func() {
	var mcsName, serviceName, policyName, deploymentName string
	var deployment *appsv1.Deployment
	var policy *policyv1alpha1.PropagationPolicy
	var service *corev1.Service
	var member1Name, member2Name string
	var member2Client kubernetes.Interface

	ginkgo.BeforeEach(func() {
		mcsName = mcsNamePrefix + rand.String(RandomStrLength)
		serviceName = mcsName
		policyName = mcsNamePrefix + rand.String(RandomStrLength)
		deploymentName = policyName
		member1Name = framework.ClusterNames()[0]
		member2Name = framework.ClusterNames()[1]

		member2Client = framework.GetClusterClient(member2Name)
		gomega.Expect(member2Client).ShouldNot(gomega.BeNil())

		service = helper.NewService(testNamespace, serviceName, corev1.ServiceTypeClusterIP)
		deployment = helper.NewDeployment(testNamespace, deploymentName)
		policy = helper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deploymentName,
			},
		}, policyv1alpha1.Placement{
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			},
		})
	})

	ginkgo.JustBeforeEach(func() {
		framework.CreateDeployment(kubeClient, deployment)
		framework.CreateService(kubeClient, service)
		ginkgo.DeferCleanup(func() {
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemoveService(kubeClient, service.Namespace, service.Name)
		})
	})

	// case 1: ProviderClusters field is [member1], ConsumerClusters field is [member2]
	ginkgo.Context(fmt.Sprintf("ProviderClusters field is [%s], ConsumerClusters field is [%s]", member1Name, member2Name), func() {
		var mcs *networkingv1alpha1.MultiClusterService

		ginkgo.BeforeEach(func() {
			mcs = helper.NewCrossClusterMultiClusterService(testNamespace, mcsName, []string{member1Name}, []string{member2Name})
			policy.Spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{member1Name},
			}
			framework.CreateMultiClusterService(karmadaClient, mcs)
			framework.CreatePropagationPolicy(karmadaClient, policy)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveMultiClusterService(karmadaClient, testNamespace, mcsName)
			framework.RemovePropagationPolicy(karmadaClient, testNamespace, policyName)
		})

		ginkgo.It("Test dispatch EndpointSlice from the provider clusters to the consumer clusters", func() {
			framework.WaitDeploymentPresentOnClustersFitWith([]string{member1Name}, testNamespace, deploymentName, func(deployment *appsv1.Deployment) bool {
				return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
			})

			framework.WaitMultiClusterServicePresentOnClustersFitWith(karmadaClient, testNamespace, mcsName, func(mcs *networkingv1alpha1.MultiClusterService) bool {
				return meta.IsStatusConditionTrue(mcs.Status.Conditions, networkingv1alpha1.EndpointSliceDispatched)
			})

			gomega.Eventually(func() bool {
				return checkEndpointSliceWithMultiClusterService(testNamespace, mcsName, mcs.Spec.ProviderClusters, mcs.Spec.ConsumerClusters)
			}, pollTimeout, pollInterval).Should(gomega.BeTrue())
		})
	})

	// case 2: ProviderClusters field is [member1,member2], ConsumerClusters field is [member2]
	ginkgo.Context(fmt.Sprintf("ProviderClusters field is [%s,%s], ConsumerClusters field is [%s]", member1Name, member2Name, member2Name), func() {
		var mcs *networkingv1alpha1.MultiClusterService

		ginkgo.BeforeEach(func() {
			mcs = helper.NewCrossClusterMultiClusterService(testNamespace, mcsName, []string{member1Name, member2Name}, []string{member2Name})
			policy.Spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{member1Name, member2Name},
			}
			framework.CreateMultiClusterService(karmadaClient, mcs)
			framework.CreatePropagationPolicy(karmadaClient, policy)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveMultiClusterService(karmadaClient, testNamespace, mcsName)
			framework.RemovePropagationPolicy(karmadaClient, testNamespace, policyName)
		})

		ginkgo.It("Test dispatch EndpointSlice from the provider clusters to the consumer clusters", func() {
			framework.WaitDeploymentPresentOnClustersFitWith([]string{member1Name, member2Name}, testNamespace, deploymentName, func(deployment *appsv1.Deployment) bool {
				return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
			})

			framework.WaitMultiClusterServicePresentOnClustersFitWith(karmadaClient, testNamespace, mcsName, func(mcs *networkingv1alpha1.MultiClusterService) bool {
				return meta.IsStatusConditionTrue(mcs.Status.Conditions, networkingv1alpha1.EndpointSliceDispatched)
			})

			gomega.Eventually(func() bool {
				return checkEndpointSliceWithMultiClusterService(testNamespace, mcsName, mcs.Spec.ProviderClusters, mcs.Spec.ConsumerClusters)
			}, pollTimeout, pollInterval).Should(gomega.BeTrue())
		})
	})

	// case 3:
	//	fist, ProviderClusters field is [member1,member2], ConsumerClusters field is [member2]
	//	then, ProviderClusters field changes to [member2], ConsumerClusters field is [member1]
	ginkgo.Context("Update ProviderClusters/ConsumerClusters field", func() {
		var mcs *networkingv1alpha1.MultiClusterService

		ginkgo.BeforeEach(func() {
			mcs = helper.NewCrossClusterMultiClusterService(testNamespace, mcsName, []string{member1Name, member2Name}, []string{member2Name})
			policy.Spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{member1Name, member2Name},
			}
			framework.CreateMultiClusterService(karmadaClient, mcs)
			framework.CreatePropagationPolicy(karmadaClient, policy)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveMultiClusterService(karmadaClient, testNamespace, mcsName)
			framework.RemovePropagationPolicy(karmadaClient, testNamespace, policyName)
		})

		ginkgo.It("Test dispatch EndpointSlice from the provider clusters to the consumer clusters", func() {
			framework.WaitDeploymentPresentOnClustersFitWith([]string{member1Name, member2Name}, testNamespace, deploymentName, func(deployment *appsv1.Deployment) bool {
				return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
			})

			framework.WaitMultiClusterServicePresentOnClustersFitWith(karmadaClient, testNamespace, mcsName, func(mcs *networkingv1alpha1.MultiClusterService) bool {
				return meta.IsStatusConditionTrue(mcs.Status.Conditions, networkingv1alpha1.EndpointSliceDispatched)
			})

			gomega.Eventually(func() bool {
				return checkEndpointSliceWithMultiClusterService(testNamespace, mcsName, mcs.Spec.ProviderClusters, mcs.Spec.ConsumerClusters)
			}, pollTimeout, pollInterval).Should(gomega.BeTrue())

			mcs.Spec.ProviderClusters = []networkingv1alpha1.ClusterSelector{
				{Name: member2Name},
			}
			mcs.Spec.ConsumerClusters = []networkingv1alpha1.ClusterSelector{
				{Name: member1Name},
			}
			framework.UpdateMultiClusterService(karmadaClient, mcs)

			framework.WaitMultiClusterServicePresentOnClustersFitWith(karmadaClient, testNamespace, mcsName, func(mcs *networkingv1alpha1.MultiClusterService) bool {
				return meta.IsStatusConditionTrue(mcs.Status.Conditions, networkingv1alpha1.EndpointSliceDispatched)
			})
		})
	})

	// case 4: ProviderClusters field is empty, ConsumerClusters field is [member2]
	ginkgo.Context(fmt.Sprintf("ProviderClusters field is empty, ConsumerClusters field is [%s]", member2Name), func() {
		var mcs *networkingv1alpha1.MultiClusterService

		ginkgo.BeforeEach(func() {
			mcs = helper.NewCrossClusterMultiClusterService(testNamespace, mcsName, []string{}, []string{member2Name})
			policy.Spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{member1Name},
			}
			framework.CreateMultiClusterService(karmadaClient, mcs)
			framework.CreatePropagationPolicy(karmadaClient, policy)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveMultiClusterService(karmadaClient, testNamespace, mcsName)
			framework.RemovePropagationPolicy(karmadaClient, testNamespace, policyName)
		})

		ginkgo.It("Test dispatch EndpointSlice from the provider clusters to the consumer clusters", func() {
			framework.WaitDeploymentPresentOnClustersFitWith([]string{member1Name}, testNamespace, deploymentName, func(deployment *appsv1.Deployment) bool {
				return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
			})

			framework.WaitMultiClusterServicePresentOnClustersFitWith(karmadaClient, testNamespace, mcsName, func(mcs *networkingv1alpha1.MultiClusterService) bool {
				return meta.IsStatusConditionTrue(mcs.Status.Conditions, networkingv1alpha1.EndpointSliceDispatched)
			})

			gomega.Eventually(func() bool {
				return checkEndpointSliceWithMultiClusterService(testNamespace, mcsName, mcs.Spec.ProviderClusters, mcs.Spec.ConsumerClusters)
			}, pollTimeout, pollInterval).Should(gomega.BeTrue())
		})
	})

	// case 5: ProviderClusters field is [member1], ConsumerClusters field is empty
	ginkgo.Context(fmt.Sprintf("ProviderClusters field is [%s], ConsumerClusters field is empty", member1Name), func() {
		var mcs *networkingv1alpha1.MultiClusterService

		ginkgo.BeforeEach(func() {
			mcs = helper.NewCrossClusterMultiClusterService(testNamespace, mcsName, []string{member1Name}, []string{})
			policy.Spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{member1Name},
			}
			framework.CreateMultiClusterService(karmadaClient, mcs)
			framework.CreatePropagationPolicy(karmadaClient, policy)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveMultiClusterService(karmadaClient, testNamespace, mcsName)
			framework.RemovePropagationPolicy(karmadaClient, testNamespace, policyName)
		})

		ginkgo.It("Test dispatch EndpointSlice from the provider clusters to the consumer clusters", func() {
			framework.WaitDeploymentPresentOnClustersFitWith([]string{member1Name}, testNamespace, deploymentName, func(deployment *appsv1.Deployment) bool {
				return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
			})

			framework.WaitMultiClusterServicePresentOnClustersFitWith(karmadaClient, testNamespace, mcsName, func(mcs *networkingv1alpha1.MultiClusterService) bool {
				return meta.IsStatusConditionTrue(mcs.Status.Conditions, networkingv1alpha1.EndpointSliceDispatched)
			})

			gomega.Eventually(func() bool {
				return checkEndpointSliceWithMultiClusterService(testNamespace, mcsName, mcs.Spec.ProviderClusters, mcs.Spec.ConsumerClusters)
			}, pollTimeout, pollInterval).Should(gomega.BeTrue())
		})
	})

	// case 6: ProviderClusters field is empty, ConsumerClusters field is empty
	ginkgo.Context("ProviderClusters field is empty, ConsumerClusters field is empty", func() {
		var mcs *networkingv1alpha1.MultiClusterService

		ginkgo.BeforeEach(func() {
			mcs = helper.NewCrossClusterMultiClusterService(testNamespace, mcsName, []string{}, []string{})
			policy.Spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{member1Name},
			}
			framework.CreateMultiClusterService(karmadaClient, mcs)
			framework.CreatePropagationPolicy(karmadaClient, policy)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveMultiClusterService(karmadaClient, testNamespace, mcsName)
			framework.RemovePropagationPolicy(karmadaClient, testNamespace, policyName)
		})

		ginkgo.It("Test dispatch EndpointSlice from the provider clusters to the consumer clusters", func() {
			framework.WaitDeploymentPresentOnClustersFitWith([]string{member1Name}, testNamespace, deploymentName, func(deployment *appsv1.Deployment) bool {
				return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
			})

			framework.WaitMultiClusterServicePresentOnClustersFitWith(karmadaClient, testNamespace, mcsName, func(mcs *networkingv1alpha1.MultiClusterService) bool {
				return meta.IsStatusConditionTrue(mcs.Status.Conditions, networkingv1alpha1.EndpointSliceDispatched)
			})

			gomega.Eventually(func() bool {
				return checkEndpointSliceWithMultiClusterService(testNamespace, mcsName, mcs.Spec.ProviderClusters, mcs.Spec.ConsumerClusters)
			}, pollTimeout, pollInterval).Should(gomega.BeTrue())
		})
	})
})

func checkEndpointSliceWithMultiClusterService(mcsNamespace, mcsName string, providerClusters, consumerClusters []networkingv1alpha1.ClusterSelector) bool {
	providerClusterNames := framework.ClusterNames()
	if len(providerClusters) != 0 {
		providerClusterNames = []string{}
		for _, providerCluster := range providerClusters {
			providerClusterNames = append(providerClusterNames, providerCluster.Name)
		}
	}

	consumerClusterNames := framework.ClusterNames()
	if len(consumerClusters) != 0 {
		consumerClusterNames = []string{}
		for _, consumerCluster := range consumerClusters {
			consumerClusterNames = append(consumerClusterNames, consumerCluster.Name)
		}
	}

	for _, clusterName := range providerClusterNames {
		providerClusterClient := framework.GetClusterClient(clusterName)
		gomega.Expect(providerClusterClient).ShouldNot(gomega.BeNil())

		providerEPSList, err := providerClusterClient.DiscoveryV1().EndpointSlices(mcsNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labels.Set{discoveryv1.LabelServiceName: mcsName}.AsSelector().String(),
		})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		for _, consumerCluster := range consumerClusterNames {
			consumerClusterClient := framework.GetClusterClient(consumerCluster)
			gomega.Expect(consumerClusterClient).ShouldNot(gomega.BeNil())

			consumerEPSList, err := consumerClusterClient.DiscoveryV1().EndpointSlices(mcsNamespace).List(context.TODO(), metav1.ListOptions{
				LabelSelector: labels.Set{discoveryv1.LabelServiceName: mcsName}.AsSelector().String(),
			})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			if !checkEndpointSliceSynced(providerEPSList, consumerEPSList, clusterName, consumerCluster) {
				return false
			}
		}
	}

	return true
}

func checkEndpointSliceSynced(providerEPSList, consumerEPSList *discoveryv1.EndpointSliceList, provisonCluster, consumerCluster string) bool {
	if provisonCluster == consumerCluster {
		return true
	}

	for _, item := range providerEPSList.Items {
		if item.GetLabels()[discoveryv1.LabelManagedBy] == util.EndpointSliceDispatchControllerLabelValue {
			continue
		}
		synced := false
		for _, consumerItem := range consumerEPSList.Items {
			if consumerItem.Name == provisonCluster+"-"+item.Name && len(consumerItem.Endpoints) == len(item.Endpoints) {
				synced = true
				break
			}
			synced = false
		}
		if !synced {
			return false
		}
	}

	return true
}
