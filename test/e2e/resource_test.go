package e2e

import (
	"context"
	"fmt"
	"reflect"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
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

			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentDeployment, err := kubeClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					klog.Infof("deployment(%s/%s) readyReplicas: %d, wanted replicas: %d", deploymentNamespace, deploymentName, currentDeployment.Status.ReadyReplicas, wantedReplicas)
					if currentDeployment.Status.ReadyReplicas == wantedReplicas &&
						currentDeployment.Status.AvailableReplicas == wantedReplicas &&
						currentDeployment.Status.UpdatedReplicas == wantedReplicas &&
						currentDeployment.Status.Replicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)

			ginkgo.By("check if deployment status has been update with new collection", func() {
				wantedReplicas := updateDeploymentReplicas * int32(len(framework.Clusters()))

				klog.Infof("Waiting for deployment(%s/%s) collecting correctly status", deploymentNamespace, deploymentName)
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentDeployment, err := kubeClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					if currentDeployment.Status.ReadyReplicas == wantedReplicas &&
						currentDeployment.Status.AvailableReplicas == wantedReplicas &&
						currentDeployment.Status.UpdatedReplicas == wantedReplicas &&
						currentDeployment.Status.Replicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
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

			service = helper.NewService(serviceNamespace, serviceName)
			service.Spec.Type = corev1.ServiceTypeLoadBalancer
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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

			service = helper.NewService(serviceNamespace, serviceName)
			service.Spec.Type = corev1.ServiceTypeNodePort
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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

			ingress = helper.NewIngress(ingNamespace, ingName)
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
			ingLoadBalancer := corev1.LoadBalancerStatus{}

			// simulate the update of the ingress status in member clusters.
			ginkgo.By("Update ingress status in member clusters", func() {
				for index, clusterName := range framework.ClusterNames() {
					clusterClient := framework.GetClusterClient(clusterName)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					ingresses := []corev1.LoadBalancerIngress{{IP: fmt.Sprintf("172.19.2.%d", index+6)}}
					for _, ingress := range ingresses {
						ingLoadBalancer.Ingress = append(ingLoadBalancer.Ingress, corev1.LoadBalancerIngress{
							IP:       ingress.IP,
							Hostname: clusterName,
						})
					}

					gomega.Eventually(func(g gomega.Gomega) {
						memberIng, err := clusterClient.NetworkingV1().Ingresses(ingNamespace).Get(context.TODO(), ingName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())

						memberIng.Status.LoadBalancer = corev1.LoadBalancerStatus{Ingress: ingresses}
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
		var patch []map[string]interface{}

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = jobNamePrefix + rand.String(RandomStrLength)
			jobNamespace = testNamespace
			jobName = policyName

			job = helper.NewJob(jobNamespace, jobName)
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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

			patch = []map[string]interface{}{
				{
					"op":    "replace",
					"path":  "/spec/placement/clusterAffinity/clusterNames",
					"value": framework.ClusterNames()[0 : len(framework.ClusterNames())-1],
				},
			}
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
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentJob, err := kubeClient.BatchV1().Jobs(jobNamespace).Get(context.TODO(), jobName, metav1.GetOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					klog.Infof("job(%s/%s) succeedPods: %d, wanted succeedPods: %d", jobNamespace, jobName, currentJob.Status.Succeeded, wantedSucceedPods)
					if currentJob.Status.Succeeded == wantedSucceedPods {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			framework.PatchPropagationPolicy(karmadaClient, policy.Namespace, policyName, patch, types.JSONPatchType)

			ginkgo.By("check if job status has been update with new collection", func() {
				wantedSucceedPods := int32(len(framework.Clusters()) - 1)

				klog.Infof("Waiting for job(%s/%s) collecting correctly status", jobNamespace, jobName)
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentJob, err := kubeClient.BatchV1().Jobs(jobNamespace).Get(context.TODO(), jobName, metav1.GetOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					if currentJob.Status.Succeeded == wantedSucceedPods {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
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

			daemonSet = helper.NewDaemonSet(daemonSetNamespace, daemonSetName)
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentDaemonSet, err := kubeClient.AppsV1().DaemonSets(daemonSetNamespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					klog.Infof("daemonSet(%s/%s) replicas: %d, wanted replicas: %d", daemonSetNamespace, daemonSetName, currentDaemonSet.Status.NumberReady, wantedReplicas)
					if currentDaemonSet.Status.NumberReady == wantedReplicas &&
						currentDaemonSet.Status.CurrentNumberScheduled == wantedReplicas &&
						currentDaemonSet.Status.DesiredNumberScheduled == wantedReplicas &&
						currentDaemonSet.Status.UpdatedNumberScheduled == wantedReplicas &&
						currentDaemonSet.Status.NumberAvailable == wantedReplicas {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			framework.PatchPropagationPolicy(karmadaClient, policy.Namespace, policyName, patch, types.JSONPatchType)

			ginkgo.By("check if daemonSet status has been update with new collection", func() {
				wantedReplicas := int32(len(framework.Clusters()) - 1)

				klog.Infof("Waiting for daemonSet(%s/%s) collecting correctly status", daemonSetNamespace, daemonSetName)
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentDaemonSet, err := kubeClient.AppsV1().DaemonSets(daemonSetNamespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					if currentDaemonSet.Status.NumberReady == wantedReplicas &&
						currentDaemonSet.Status.CurrentNumberScheduled == wantedReplicas &&
						currentDaemonSet.Status.DesiredNumberScheduled == wantedReplicas &&
						currentDaemonSet.Status.UpdatedNumberScheduled == wantedReplicas &&
						currentDaemonSet.Status.NumberAvailable == wantedReplicas {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
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

			statefulSet = helper.NewStatefulSet(statefulSetNamespace, statefulSetName)
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentStatefulSet, err := kubeClient.AppsV1().StatefulSets(statefulSetNamespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					klog.Infof("statefulSet(%s/%s) replicas: %d, wanted replicas: %d", statefulSetNamespace, statefulSetName, currentStatefulSet.Status.Replicas, wantedReplicas)
					if currentStatefulSet.Status.Replicas == wantedReplicas &&
						currentStatefulSet.Status.ReadyReplicas == wantedReplicas &&
						currentStatefulSet.Status.CurrentReplicas == wantedReplicas &&
						currentStatefulSet.Status.UpdatedReplicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			framework.UpdateStatefulSetReplicas(kubeClient, statefulSet, updateStatefulSetReplicas)

			ginkgo.By("check if statefulSet status has been update with new collection", func() {
				wantedReplicas := updateStatefulSetReplicas * int32(len(framework.Clusters()))

				klog.Infof("Waiting for statefulSet(%s/%s) collecting correctly status", statefulSetNamespace, statefulSetName)
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentStatefulSet, err := kubeClient.AppsV1().StatefulSets(statefulSetNamespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					if currentStatefulSet.Status.Replicas == wantedReplicas &&
						currentStatefulSet.Status.ReadyReplicas == wantedReplicas &&
						currentStatefulSet.Status.CurrentReplicas == wantedReplicas &&
						currentStatefulSet.Status.UpdatedReplicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})
})
