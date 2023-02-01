package e2e

import (
	"context"
	"fmt"
	"math"
	"reflect"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	controllercluster "github.com/karmada-io/karmada/pkg/controllers/cluster"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[resource-status collection] resource status collection testing", func() {
	var policyNamespace, policyName string
	var policy *policyv1alpha1.PropagationPolicy

	ginkgo.JustBeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy)
		ginkgo.DeferCleanup(func() {
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	ginkgo.Context("DeploymentStatus collection testing", func() {
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment

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
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment status collection testing", func() {
			ginkgo.By("check whether the deployment status can be correctly collected", func() {
				wantedReplicas := *deployment.Spec.Replicas * int32(len(framework.Clusters()))

				klog.Infof("Waiting for deployment(%s/%s) collecting correctly status", deploymentNamespace, deploymentName)
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					currentDeployment, err := kubeClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					klog.Infof("deployment(%s/%s) readyReplicas: %d, wanted replicas: %d", deploymentNamespace, deploymentName, currentDeployment.Status.ReadyReplicas, wantedReplicas)
					if currentDeployment.Status.ReadyReplicas == wantedReplicas &&
						currentDeployment.Status.AvailableReplicas == wantedReplicas &&
						currentDeployment.Status.UpdatedReplicas == wantedReplicas &&
						currentDeployment.Status.Replicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)

			ginkgo.By("check if deployment status has been update with new collection", func() {
				wantedReplicas := updateDeploymentReplicas * int32(len(framework.Clusters()))

				klog.Infof("Waiting for deployment(%s/%s) collecting correctly status", deploymentNamespace, deploymentName)
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					currentDeployment, err := kubeClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					if currentDeployment.Status.ReadyReplicas == wantedReplicas &&
						currentDeployment.Status.AvailableReplicas == wantedReplicas &&
						currentDeployment.Status.UpdatedReplicas == wantedReplicas &&
						currentDeployment.Status.Replicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})

	ginkgo.Context("ServiceStatus collection testing", func() {
		var serviceNamespace, serviceName string
		var service *corev1.Service

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = serviceNamePrefix + rand.String(RandomStrLength)
			serviceNamespace = testNamespace
			serviceName = policyName

			service = testhelper.NewService(serviceNamespace, serviceName, corev1.ServiceTypeLoadBalancer)
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
			framework.CreateService(kubeClient, service)
			ginkgo.DeferCleanup(func() {
				framework.RemoveService(kubeClient, serviceNamespace, serviceName)
			})
		})

		ginkgo.It("service status collection testing", func() {
			svcLoadBalancer := corev1.LoadBalancerStatus{}

			// simulate the update of the service status in member clusters.
			ginkgo.By("Update service status in member clusters", func() {
				for index, clusterName := range framework.ClusterNames() {
					clusterClient := framework.GetClusterClient(clusterName)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					ingresses := []corev1.LoadBalancerIngress{{IP: fmt.Sprintf("172.19.1.%d", index+6)}}
					for _, ingress := range ingresses {
						svcLoadBalancer.Ingress = append(svcLoadBalancer.Ingress, corev1.LoadBalancerIngress{
							IP:       ingress.IP,
							Hostname: clusterName,
						})
					}

					gomega.Eventually(func(g gomega.Gomega) {
						memberSvc, err := clusterClient.CoreV1().Services(serviceNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())

						memberSvc.Status.LoadBalancer = corev1.LoadBalancerStatus{Ingress: ingresses}
						_, err = clusterClient.CoreV1().Services(serviceNamespace).UpdateStatus(context.TODO(), memberSvc, metav1.UpdateOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())
					}, pollTimeout, pollInterval).Should(gomega.Succeed())
				}
			})
			klog.Infof("svcLoadBalancer: %v", svcLoadBalancer)

			ginkgo.By("check if service status has been update with collection", func() {
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					latestSvc, err := kubeClient.CoreV1().Services(serviceNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())

					klog.Infof("the latest serviceStatus loadBalancer: %v", latestSvc.Status.LoadBalancer)
					return reflect.DeepEqual(latestSvc.Status.LoadBalancer, svcLoadBalancer), nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})

	ginkgo.Context("NodePort Service collection testing", func() {
		var serviceNamespace, serviceName string
		var service *corev1.Service

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = serviceNamePrefix + rand.String(RandomStrLength)
			serviceNamespace = testNamespace
			serviceName = policyName

			service = testhelper.NewService(serviceNamespace, serviceName, corev1.ServiceTypeNodePort)
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
			framework.CreateService(kubeClient, service)
			ginkgo.DeferCleanup(func() {
				framework.RemoveService(kubeClient, serviceNamespace, serviceName)
			})
		})

		ginkgo.It("NodePort service apply status collection testing", func() {
			nodePorts := sets.NewInt32()
			// collect the NodePort of the service in member clusters.
			ginkgo.By("Update service status in member clusters", func() {

				for _, clusterName := range framework.ClusterNames() {
					clusterClient := framework.GetClusterClient(clusterName)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
					gomega.Eventually(func(g gomega.Gomega) {
						memberSvc, err := clusterClient.CoreV1().Services(serviceNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())
						for _, servicePort := range memberSvc.Spec.Ports {
							nodePorts.Insert(servicePort.NodePort)
						}
					}, pollTimeout, pollInterval).Should(gomega.Succeed())
				}
				// check service nodePort
				gomega.Expect(nodePorts.Len() == 1).Should(gomega.BeTrue())
			})
			klog.Infof("svcNodePort: %v", nodePorts.List()[0])

			ginkgo.By("check service ResourceBindings apply status ", func() {
				gomega.Eventually(func(g gomega.Gomega) (metav1.ConditionStatus, error) {
					resourceBindingName := names.GenerateBindingName(service.Kind, service.Name)
					resourceBinding, err := karmadaClient.WorkV1alpha2().ResourceBindings(serviceNamespace).Get(context.TODO(), resourceBindingName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())
					var fullyAppliedStatus metav1.ConditionStatus
					for _, condition := range resourceBinding.Status.Conditions {
						if condition.Type == workv1alpha2.FullyApplied {
							fullyAppliedStatus = condition.Status
						}
					}
					return fullyAppliedStatus, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(metav1.ConditionTrue))
			})
		})
	})

	ginkgo.Context("IngressStatus collection testing", func() {
		var ingNamespace, ingName string
		var ingress *networkingv1.Ingress

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = ingressNamePrefix + rand.String(RandomStrLength)
			ingNamespace = testNamespace
			ingName = policyName

			ingress = testhelper.NewIngress(ingNamespace, ingName)
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: ingress.APIVersion,
					Kind:       ingress.Kind,
					Name:       ingress.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateIngress(kubeClient, ingress)
			ginkgo.DeferCleanup(func() {
				framework.RemoveIngress(kubeClient, ingNamespace, ingName)
			})
		})

		ginkgo.It("ingress status collection testing", func() {
			ingLoadBalancer := networkingv1.IngressLoadBalancerStatus{}

			// simulate the update of the ingress status in member clusters.
			ginkgo.By("Update ingress status in member clusters", func() {
				for index, clusterName := range framework.ClusterNames() {
					clusterClient := framework.GetClusterClient(clusterName)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					ingresses := []networkingv1.IngressLoadBalancerIngress{{IP: fmt.Sprintf("172.19.2.%d", index+6)}}
					for _, ingress := range ingresses {
						ingLoadBalancer.Ingress = append(ingLoadBalancer.Ingress, networkingv1.IngressLoadBalancerIngress{
							IP:       ingress.IP,
							Hostname: clusterName,
						})
					}

					gomega.Eventually(func(g gomega.Gomega) {
						memberIng, err := clusterClient.NetworkingV1().Ingresses(ingNamespace).Get(context.TODO(), ingName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())

						memberIng.Status.LoadBalancer = networkingv1.IngressLoadBalancerStatus{Ingress: ingresses}
						_, err = clusterClient.NetworkingV1().Ingresses(ingNamespace).UpdateStatus(context.TODO(), memberIng, metav1.UpdateOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())
					}, pollTimeout, pollInterval).Should(gomega.Succeed())
				}
			})
			klog.Infof("ingLoadBalancer: %v", ingLoadBalancer)

			ginkgo.By("check if ingress status has been update with collection", func() {
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					latestIng, err := kubeClient.NetworkingV1().Ingresses(ingNamespace).Get(context.TODO(), ingName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())

					klog.Infof("the latest ingressStatus loadBalancer: %v", latestIng.Status.LoadBalancer)
					return reflect.DeepEqual(latestIng.Status.LoadBalancer, ingLoadBalancer), nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})

	ginkgo.Context("JobStatus collection testing", func() {
		var jobNamespace, jobName string
		var job *batchv1.Job

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
			framework.CreateJob(kubeClient, job)
			ginkgo.DeferCleanup(func() {
				framework.RemoveJob(kubeClient, jobNamespace, jobName)
			})
		})

		ginkgo.It("job status collection testing", func() {
			ginkgo.By("check whether the job status can be correctly collected", func() {
				wantedSucceedPods := int32(len(framework.Clusters()))

				klog.Infof("Waiting for job(%s/%s) collecting correctly status", jobNamespace, jobName)
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					currentJob, err := kubeClient.BatchV1().Jobs(jobNamespace).Get(context.TODO(), jobName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					klog.Infof("job(%s/%s) succeedPods: %d, wanted succeedPods: %d", jobNamespace, jobName, currentJob.Status.Succeeded, wantedSucceedPods)
					if currentJob.Status.Succeeded == wantedSucceedPods {
						return true, nil
					}

					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})

	ginkgo.Context("DaemonSetStatus collection testing", func() {
		var daemonSetNamespace, daemonSetName string
		var daemonSet *appsv1.DaemonSet
		var patch []map[string]interface{}

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = daemonSetNamePrefix + rand.String(RandomStrLength)
			daemonSetNamespace = testNamespace
			daemonSetName = policyName

			daemonSet = testhelper.NewDaemonSet(daemonSetNamespace, daemonSetName)
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: daemonSet.APIVersion,
					Kind:       daemonSet.Kind,
					Name:       daemonSet.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})

			patch = []map[string]interface{}{
				{
					"op":    "replace",
					"path":  "/spec/placement/clusterAffinity/clusterNames",
					"value": framework.ClusterNames()[0 : len(framework.ClusterNames())-1],
				},
			}
		})

		ginkgo.BeforeEach(func() {
			framework.CreateDaemonSet(kubeClient, daemonSet)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDaemonSet(kubeClient, daemonSetNamespace, daemonSetName)
			})
		})

		ginkgo.It("daemonSet status collection testing", func() {
			ginkgo.By("check whether the daemonSet status can be correctly collected", func() {
				wantedReplicas := int32(len(framework.Clusters()))

				klog.Infof("Waiting for daemonSet(%s/%s) collecting correctly status", daemonSetNamespace, daemonSetName)
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					currentDaemonSet, err := kubeClient.AppsV1().DaemonSets(daemonSetNamespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					klog.Infof("daemonSet(%s/%s) replicas: %d, wanted replicas: %d", daemonSetNamespace, daemonSetName, currentDaemonSet.Status.NumberReady, wantedReplicas)
					if currentDaemonSet.Status.NumberReady == wantedReplicas &&
						currentDaemonSet.Status.CurrentNumberScheduled == wantedReplicas &&
						currentDaemonSet.Status.DesiredNumberScheduled == wantedReplicas &&
						currentDaemonSet.Status.UpdatedNumberScheduled == wantedReplicas &&
						currentDaemonSet.Status.NumberAvailable == wantedReplicas {
						return true, nil
					}

					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			framework.PatchPropagationPolicy(karmadaClient, policy.Namespace, policyName, patch, types.JSONPatchType)

			ginkgo.By("check if daemonSet status has been update with new collection", func() {
				wantedReplicas := int32(len(framework.Clusters()) - 1)

				klog.Infof("Waiting for daemonSet(%s/%s) collecting correctly status", daemonSetNamespace, daemonSetName)
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					currentDaemonSet, err := kubeClient.AppsV1().DaemonSets(daemonSetNamespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					if currentDaemonSet.Status.NumberReady == wantedReplicas &&
						currentDaemonSet.Status.CurrentNumberScheduled == wantedReplicas &&
						currentDaemonSet.Status.DesiredNumberScheduled == wantedReplicas &&
						currentDaemonSet.Status.UpdatedNumberScheduled == wantedReplicas &&
						currentDaemonSet.Status.NumberAvailable == wantedReplicas {
						return true, nil
					}

					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})

	ginkgo.Context("StatefulSetStatus collection testing", func() {
		var statefulSetNamespace, statefulSetName string
		var statefulSet *appsv1.StatefulSet

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = statefulSetNamePrefix + rand.String(RandomStrLength)
			statefulSetNamespace = testNamespace
			statefulSetName = policyName

			statefulSet = testhelper.NewStatefulSet(statefulSetNamespace, statefulSetName)
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: statefulSet.APIVersion,
					Kind:       statefulSet.Kind,
					Name:       statefulSet.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateStatefulSet(kubeClient, statefulSet)
			ginkgo.DeferCleanup(func() {
				framework.RemoveStatefulSet(kubeClient, statefulSetNamespace, statefulSetName)
			})
		})

		ginkgo.It("statefulSet status collection testing", func() {
			ginkgo.By("check whether the statefulSet status can be correctly collected", func() {
				wantedReplicas := *statefulSet.Spec.Replicas * int32(len(framework.Clusters()))
				klog.Infof("Waiting for statefulSet(%s/%s) collecting correctly status", statefulSetNamespace, statefulSetName)
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					currentStatefulSet, err := kubeClient.AppsV1().StatefulSets(statefulSetNamespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					klog.Infof("statefulSet(%s/%s) replicas: %d, wanted replicas: %d", statefulSetNamespace, statefulSetName, currentStatefulSet.Status.Replicas, wantedReplicas)
					if currentStatefulSet.Status.Replicas == wantedReplicas &&
						currentStatefulSet.Status.ReadyReplicas == wantedReplicas &&
						currentStatefulSet.Status.CurrentReplicas == wantedReplicas &&
						currentStatefulSet.Status.UpdatedReplicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			framework.UpdateStatefulSetReplicas(kubeClient, statefulSet, updateStatefulSetReplicas)

			ginkgo.By("check if statefulSet status has been update with new collection", func() {
				wantedReplicas := updateStatefulSetReplicas * int32(len(framework.Clusters()))

				klog.Infof("Waiting for statefulSet(%s/%s) collecting correctly status", statefulSetNamespace, statefulSetName)
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					currentStatefulSet, err := kubeClient.AppsV1().StatefulSets(statefulSetNamespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					if currentStatefulSet.Status.Replicas == wantedReplicas &&
						currentStatefulSet.Status.ReadyReplicas == wantedReplicas &&
						currentStatefulSet.Status.CurrentReplicas == wantedReplicas &&
						currentStatefulSet.Status.UpdatedReplicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})

	ginkgo.Context("PodDisruptionBudget collection testing", func() {
		var pdbNamespace, pdbName string
		var pdb *policyv1.PodDisruptionBudget
		var deployment *appsv1.Deployment

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = podDisruptionBudgetNamePrefix + rand.String(RandomStrLength)
			pdbNamespace = testNamespace
			pdbName = policyName
			deploymentName := policyName

			deployment = testhelper.NewDeployment(pdbNamespace, deploymentName)
			pdb = testhelper.NewPodDisruptionBudget(pdbNamespace, pdbName, intstr.FromString("50%"))
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: pdb.APIVersion,
					Kind:       pdb.Kind,
					Name:       pdb.Name,
				},
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
			framework.CreateDeployment(kubeClient, deployment)
			framework.CreatePodDisruptionBudget(kubeClient, pdb)

			ginkgo.DeferCleanup(func() {
				framework.RemovePodDisruptionBudget(kubeClient, pdbNamespace, pdbName)
			})
		})

		ginkgo.It("pdb status collection testing", func() {
			ginkgo.By("check whether the pdb status can be correctly collected", func() {
				klog.Infof("Waiting for PodDisruptionBudget(%s/%s) collecting correctly status", pdbNamespace, pdbName)
				maxUnavailable := 0.5 // 50%
				numOfClusters := int32(len(framework.Clusters()))
				wantedExpectedPods := *deployment.Spec.Replicas * numOfClusters
				wantedDisruptionAllowed := int32(math.Ceil(float64(*deployment.Spec.Replicas)*maxUnavailable)) * numOfClusters

				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					currentPodDisruptionBudget, err := kubeClient.PolicyV1().PodDisruptionBudgets(pdbNamespace).Get(context.TODO(), pdbName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					klog.Infof("PodDisruptionBudget(%s/%s) Disruption Allowed: %d, wanted: %d", pdbNamespace, pdbName, currentPodDisruptionBudget.Status.DisruptionsAllowed, wantedDisruptionAllowed)
					klog.Infof("PodDisruptionBudget(%s/%s) Expected Pods: %d, wanted: %d", pdbNamespace, pdbName, currentPodDisruptionBudget.Status.ExpectedPods, wantedExpectedPods)
					if currentPodDisruptionBudget.Status.DisruptionsAllowed == wantedDisruptionAllowed &&
						currentPodDisruptionBudget.Status.ExpectedPods == wantedExpectedPods {
						return true, nil
					}

					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})
})

var _ = framework.SerialDescribe("workload status synchronization testing", func() {
	ginkgo.Context("Deployment status synchronization when cluster failed and recovered soon", func() {
		var policyNamespace, policyName string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var policy *policyv1alpha1.PropagationPolicy
		var originalReplicas, numOfFailedClusters int

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = policyName
			deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
			numOfFailedClusters = 1
			originalReplicas = 3

			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						// only test push mode clusters
						// because pull mode clusters cannot be disabled by changing APIEndpoint
						MatchLabels: pushModeClusterLabels,
					},
				},
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
		})

		ginkgo.It("deployment status synchronization testing", func() {
			var disabledClusters []string
			targetClusterNames := framework.ExtractTargetClustersFrom(controlPlaneClient, deployment)

			ginkgo.By("set one cluster condition status to false", func() {
				temp := numOfFailedClusters
				for _, targetClusterName := range targetClusterNames {
					if temp > 0 {
						klog.Infof("Set cluster %s to disable.", targetClusterName)
						err := disableCluster(controlPlaneClient, targetClusterName)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						// wait for the current cluster status changing to false
						framework.WaitClusterFitWith(controlPlaneClient, targetClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
							return helper.TaintExists(cluster.Spec.Taints, controllercluster.NotReadyTaintTemplate)
						})
						disabledClusters = append(disabledClusters, targetClusterName)
						temp--
					}
				}
			})

			ginkgo.By("recover not ready cluster", func() {
				for _, disabledCluster := range disabledClusters {
					fmt.Printf("cluster %s is waiting for recovering\n", disabledCluster)
					originalAPIEndpoint := getClusterAPIEndpoint(disabledCluster)

					err := recoverCluster(controlPlaneClient, disabledCluster, originalAPIEndpoint)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					// wait for the disabled cluster recovered
					gomega.Eventually(func(g gomega.Gomega) (bool, error) {
						currentCluster, err := util.GetCluster(controlPlaneClient, disabledCluster)
						g.Expect(err).ShouldNot(gomega.HaveOccurred())

						if !helper.TaintExists(currentCluster.Spec.Taints, controllercluster.NotReadyTaintTemplate) {
							fmt.Printf("cluster %s recovered\n", disabledCluster)
							return true, nil
						}
						return false, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(true))
				}
			})

			ginkgo.By("edit deployment in disabled cluster", func() {
				for _, disabledCluster := range disabledClusters {
					clusterClient := framework.GetClusterClient(disabledCluster)
					framework.UpdateDeploymentReplicas(clusterClient, deployment, updateDeploymentReplicas)
					// wait for the status synchronization
					gomega.Eventually(func(g gomega.Gomega) (bool, error) {
						currentDeployment, err := clusterClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						g.Expect(err).ShouldNot(gomega.HaveOccurred())

						if *currentDeployment.Spec.Replicas == int32(originalReplicas) {
							return true, nil
						}
						return false, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(true))
				}
			})
		})
	})
})
