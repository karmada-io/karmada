/*
Copyright 2022 The Karmada Authors.

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

package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/karmadactl/cordon"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util"
	khelper "github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

const (
	karmadactlTimeout = time.Second * 10
)

var _ = ginkgo.Describe("Karmadactl promote testing", func() {
	var member1 string
	var member1Client kubernetes.Interface

	ginkgo.BeforeEach(func() {
		member1 = "member1"
		member1Client = framework.GetClusterClient(member1)
		defaultConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)
		defaultConfigFlags.Context = &karmadaContext
	})

	ginkgo.Context("Test promoting namespaced resource: deployment", func() {
		var deployment *appsv1.Deployment
		var deploymentNamespace, deploymentName string

		ginkgo.BeforeEach(func() {
			deploymentNamespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
		})

		ginkgo.AfterEach(func() {
			deploymentGVK := schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			}
			namespaceGVK := schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Namespace",
			}
			ppName := names.GeneratePolicyName(deploymentNamespace, deploymentName, deploymentGVK.String())
			cppName := names.GeneratePolicyName("", deploymentNamespace, namespaceGVK.String())
			framework.RemoveDeployment(kubeClient, deploymentNamespace, deploymentName)
			framework.RemovePropagationPolicy(karmadaClient, deploymentNamespace, ppName)
			framework.RemoveNamespace(kubeClient, deploymentNamespace)
			framework.RemoveClusterPropagationPolicy(karmadaClient, cppName)
		})

		ginkgo.It("Test promoting a deployment from cluster member", func() {
			// Step 1,  create namespace and deployment on cluster member1
			ginkgo.By(fmt.Sprintf("Creating deployment %s with namespace %s not existed in karmada control plane", deploymentName, deploymentNamespace), func() {
				deploymentNamespaceObj := helper.NewNamespace(deploymentNamespace)
				framework.CreateNamespace(member1Client, deploymentNamespaceObj)
				framework.CreateDeployment(member1Client, deployment)
			})

			// Step 2, promote namespace used by the deployment from member1 to karmada
			ginkgo.By(fmt.Sprintf("Promoting namespace %s from member: %s to karmada control plane", deploymentNamespace, member1), func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "promote", "namespace", deploymentNamespace, "-C", member1)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				framework.WaitNamespacePresentOnClusterByClient(kubeClient, deploymentNamespace)
			})

			// Step 3,  promote deployment from cluster member1 to karmada
			ginkgo.By(fmt.Sprintf("Promoting deployment %s from member: %s to karmada", deploymentName, member1), func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, deploymentNamespace, karmadactlTimeout, "promote", "deployment", deploymentName, "-C", member1)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Waiting for deployment %s promoted to the karmada control plane", deploymentName), func() {
				gomega.Eventually(func() bool {
					_, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					return err == nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By(fmt.Sprintf("Waiting for deployment(%s)'s replicas is ready", deploymentName), func() {
				wantedReplicas := *deployment.Spec.Replicas

				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					currentDeployment, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					return framework.CheckDeploymentReadyStatus(currentDeployment, wantedReplicas), nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})

	ginkgo.Context("Test promoting cluster resources: clusterrole and clusterrolebinding", func() {
		var clusterRoleName, clusterRoleBindingName string
		var clusterRole *rbacv1.ClusterRole
		var clusterRoleBinding *rbacv1.ClusterRoleBinding

		ginkgo.BeforeEach(func() {
			var nameFlag = rand.String(RandomStrLength)
			clusterRoleName = fmt.Sprintf("test-%s-clusterrole", nameFlag)
			clusterRoleBindingName = fmt.Sprintf("test-%s-clusterrolebinding", nameFlag)
			clusterRole = helper.NewClusterRole(clusterRoleName, []rbacv1.PolicyRule{
				{
					APIGroups:     []string{"cluster.karmada.io"},
					Verbs:         []string{"*"},
					Resources:     []string{"clusters/proxy"},
					ResourceNames: []string{member1},
				},
			})
			clusterRoleBinding = helper.NewClusterRoleBinding(clusterRoleBindingName, clusterRoleName, []rbacv1.Subject{
				{APIGroup: "rbac.authorization.k8s.io", Kind: "User", Name: "test"},
			})
		})

		ginkgo.AfterEach(func() {
			clusterRoleGVK := schema.GroupVersionKind{
				Group:   "rbac.authorization.k8s.io",
				Version: "v1",
				Kind:    "ClusterRole",
			}
			clusterRoleBindingGVK := schema.GroupVersionKind{
				Group:   "rbac.authorization.k8s.io",
				Version: "v1",
				Kind:    "ClusterRoleBinding",
			}
			clusterRoleClusterPropagationPolicy := names.GeneratePolicyName("", clusterRoleName, clusterRoleGVK.String())
			clusterRoleBindingClusterPropagationPolicy := names.GeneratePolicyName("", clusterRoleBindingName, clusterRoleBindingGVK.String())
			framework.RemoveClusterRole(kubeClient, clusterRoleName)
			framework.RemoveClusterPropagationPolicy(karmadaClient, clusterRoleClusterPropagationPolicy)

			framework.RemoveClusterRoleBinding(kubeClient, clusterRoleBindingName)
			framework.RemoveClusterPropagationPolicy(karmadaClient, clusterRoleBindingClusterPropagationPolicy)
		})

		ginkgo.It("Test promoting clusterrole and clusterrolebindings", func() {
			// Step1, create clusterrole and clusterrolebinding on member1
			ginkgo.By(fmt.Sprintf("Creating clusterrole and clusterrolebinding in member: %s", member1), func() {
				framework.CreateClusterRole(member1Client, clusterRole)
				framework.CreateClusterRoleBinding(member1Client, clusterRoleBinding)
			})

			// Step2, promote clusterrole and clusterrolebinding from member1
			ginkgo.By(fmt.Sprintf("Promoting clusterrole %s and clusterrolebindings %s from member to karmada", clusterRoleName, clusterRoleBindingName), func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "promote", "clusterrole", clusterRoleName, "-C", member1)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				cmd = framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "promote", "clusterrolebinding", clusterRoleBindingName, "-C", member1)
				_, err = cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			// Step3, check clusterrole and clusterrolebing is promoted
			ginkgo.By(fmt.Sprintf("Waiting for clusterrole %s and clusterrolebinding %s promoted to the karmada control plane", clusterRoleName, clusterRoleBindingName), func() {
				gomega.Eventually(func() bool {
					_, err1 := kubeClient.RbacV1().ClusterRoles().Get(context.TODO(), clusterRoleName, metav1.GetOptions{})
					_, err2 := kubeClient.RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterRoleBindingName, metav1.GetOptions{})
					return err1 == nil && err2 == nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})

	})

	ginkgo.Context("Test promoting namespaced resource: service", func() {
		var service *corev1.Service
		var serviceNamespace, serviceName, workName, esName string

		ginkgo.BeforeEach(func() {
			serviceNamespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			serviceName = serviceNamePrefix + rand.String(RandomStrLength)
			service = helper.NewService(serviceNamespace, serviceName, corev1.ServiceTypeNodePort)
			workName = names.GenerateWorkName(util.ServiceKind, serviceName, serviceNamespace)
			esName = names.GenerateExecutionSpaceName(member1)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveService(kubeClient, serviceNamespace, serviceName)
			framework.RemoveNamespace(kubeClient, serviceNamespace)
		})

		ginkgo.It("Test promoting a service from cluster member", func() {
			ginkgo.By(fmt.Sprintf("Creating service %s with namespace %s not existed in karmada control plane", serviceName, serviceNamespace), func() {
				serviceNamespaceObj := helper.NewNamespace(serviceNamespace)
				framework.CreateNamespace(member1Client, serviceNamespaceObj)
				framework.CreateService(member1Client, service)
			})

			ginkgo.By(fmt.Sprintf("Promoting namespace %s from member: %s to karmada control plane", serviceNamespace, member1), func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "promote", "namespace", serviceNamespace, "-C", member1)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				framework.WaitNamespacePresentOnClusterByClient(kubeClient, serviceNamespace)
			})

			ginkgo.By(fmt.Sprintf("Promoting service %s from member: %s to karmada control plane", serviceName, member1), func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, serviceNamespace, karmadactlTimeout, "promote", "service", serviceName, "-C", member1)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Waiting for work of service %s existing in  the karmada control plane", serviceNamePrefix), func() {
				gomega.Eventually(func() bool {
					_, err := karmadaClient.WorkV1alpha1().Works(esName).Get(context.TODO(), workName, metav1.GetOptions{})
					return err == nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By(fmt.Sprintf("Check condition of the work generated by the service %s is `Applied`", serviceNamePrefix), func() {
				gomega.Eventually(func() bool {
					work, _ := karmadaClient.WorkV1alpha1().Works(esName).Get(context.TODO(), workName, metav1.GetOptions{})
					applied := khelper.IsResourceApplied(&work.Status)
					return applied
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})
})

var _ = framework.SerialDescribe("Karmadactl join/unjoin testing", ginkgo.Labels{NeedCreateCluster}, func() {
	ginkgo.Context("joining cluster and unjoining not ready cluster", func() {
		var clusterName string
		var homeDir string
		var kubeConfigPath string
		var clusterContext string
		var controlPlane string
		var deploymentName, deploymentNamespace string
		var policyName, policyNamespace string
		var deployment *appsv1.Deployment
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			clusterName = "member-e2e-" + rand.String(3)
			homeDir = os.Getenv("HOME")
			kubeConfigPath = fmt.Sprintf("%s/.kube/%s.config", homeDir, clusterName)
			clusterContext = fmt.Sprintf("kind-%s", clusterName)
			controlPlane = fmt.Sprintf("%s-control-plane", clusterName)
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			policyName = deploymentName
			policyNamespace = testNamespace

			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{clusterName},
				},
			})
			defaultConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)
			defaultConfigFlags.Context = &karmadaContext
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Create cluster: %s", clusterName), func() {
				err := createCluster(clusterName, kubeConfigPath, controlPlane, clusterContext)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.DeferCleanup(func() {
				ginkgo.By(fmt.Sprintf("Deleting clusters: %s", clusterName), func() {
					err := deleteCluster(clusterName, kubeConfigPath)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					_ = os.Remove(kubeConfigPath)
				})
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Joining cluster: %s", clusterName), func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "join",
					"--cluster-kubeconfig", kubeConfigPath, "--cluster-context", clusterContext, "--cluster-namespace", "karmada-cluster", clusterName)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
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

		ginkgo.It("Test unjoining not ready cluster", func() {
			ginkgo.By("Checking cluster status collection", func() {
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					cluster, err := framework.FetchCluster(karmadaClient, clusterName)
					g.Expect(err).ShouldNot(gomega.HaveOccurred())
					if cluster.Status.KubernetesVersion == "" {
						return false, nil
					}
					if len(cluster.Status.APIEnablements) == 0 {
						return false, nil
					}
					if len(cluster.Status.Conditions) == 0 {
						return false, nil
					}
					if cluster.Status.NodeSummary == nil {
						return false, nil
					}
					if cluster.Status.ResourceSummary == nil || len(cluster.Status.ResourceSummary.AllocatableModelings) == 0 {
						return false, nil
					}
					return true, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
			ginkgo.By("Waiting for deployment have been propagated to the member cluster.", func() {
				klog.Infof("Waiting for deployment(%s/%s) synced on cluster(%s)", deploymentNamespace, deploymentName, clusterName)

				clusterConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				clusterClient := kubernetes.NewForConfigOrDie(clusterConfig)

				gomega.Eventually(func() bool {
					_, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					return err == nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By(fmt.Sprintf("Disable cluster: %s", clusterName), func() {
				err := disableCluster(controlPlaneClient, clusterName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				framework.WaitClusterFitWith(controlPlaneClient, clusterName, func(cluster *clusterv1alpha1.Cluster) bool {
					return meta.IsStatusConditionPresentAndEqual(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)
				})
			})

			ginkgo.By(fmt.Sprintf("Unjoinning cluster: %s", clusterName), func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", 5*options.DefaultKarmadactlCommandDuration,
					"unjoin", "--cluster-kubeconfig", kubeConfigPath, "--cluster-context", clusterContext, "--cluster-namespace", "karmada-cluster", clusterName)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})
})

var _ = framework.SerialDescribe("Karmadactl cordon/uncordon testing", ginkgo.Labels{NeedCreateCluster}, func() {
	var controlPlane string
	var clusterName string
	var homeDir string
	var kubeConfigPath string
	var clusterContext string
	var f cmdutil.Factory

	ginkgo.BeforeEach(func() {
		clusterName = "member-e2e-" + rand.String(3)
		homeDir = os.Getenv("HOME")
		kubeConfigPath = fmt.Sprintf("%s/.kube/%s.config", homeDir, clusterName)
		controlPlane = fmt.Sprintf("%s-control-plane", clusterName)
		clusterContext = fmt.Sprintf("kind-%s", clusterName)

		defaultConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)
		defaultConfigFlags.Context = &karmadaContext
		f = cmdutil.NewFactory(defaultConfigFlags)
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By(fmt.Sprintf("Creating cluster: %s", clusterName), func() {
			err := createCluster(clusterName, kubeConfigPath, controlPlane, clusterContext)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})
		ginkgo.By(fmt.Sprintf("Joining cluster: %s", clusterName), func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout,
				"join", "--cluster-kubeconfig", kubeConfigPath, "--cluster-context", clusterContext, "--cluster-namespace", "karmada-cluster", clusterName)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})
		// When a newly joined cluster is unready at the beginning, the scheduler will ignore it.
		ginkgo.By(fmt.Sprintf("wait cluster %s ready", clusterName), func() {
			framework.WaitClusterFitWith(controlPlaneClient, clusterName, func(cluster *clusterv1alpha1.Cluster) bool {
				return meta.IsStatusConditionPresentAndEqual(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
			})
		})
		ginkgo.DeferCleanup(func() {
			ginkgo.By(fmt.Sprintf("Unjoinning cluster: %s", clusterName), func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", 5*options.DefaultKarmadactlCommandDuration,
					"unjoin", "--cluster-kubeconfig", kubeConfigPath, "--cluster-context", clusterContext, "--cluster-namespace", "karmada-cluster", clusterName)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.By(fmt.Sprintf("Deleting clusters: %s", clusterName), func() {
				err := deleteCluster(clusterName, kubeConfigPath)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				_ = os.Remove(kubeConfigPath)
			})
		})
	})

	ginkgo.Context("cordon/uncordon cluster taint check", func() {
		ginkgo.BeforeEach(func() {
			opts := cordon.CommandCordonOption{
				ClusterName: clusterName,
			}
			err := cordon.RunCordonOrUncordon(cordon.DesiredCordon, f, opts)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It(fmt.Sprintf("cluster %s should have unschedulable:NoSchedule taint", clusterName), func() {
			clusterObj := &clusterv1alpha1.Cluster{}
			err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(
				khelper.TaintExists(
					clusterObj.Spec.Taints,
					&corev1.Taint{
						Key:    clusterv1alpha1.TaintClusterUnscheduler,
						Effect: corev1.TaintEffectNoSchedule,
					})).
				Should(gomega.Equal(true))
		})

		ginkgo.It(fmt.Sprintf("cluster %s should not have unschedulable:NoSchedule taint", clusterName), func() {
			opts := cordon.CommandCordonOption{
				ClusterName: clusterName,
			}
			err := cordon.RunCordonOrUncordon(cordon.DesiredUnCordon, f, opts)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			ginkgo.By(fmt.Sprintf("cluster %s taint(unschedulable:NoSchedule) will be removed", clusterName), func() {
				clusterObj := &clusterv1alpha1.Cluster{}
				err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(
					khelper.TaintExists(
						clusterObj.Spec.Taints,
						&corev1.Taint{
							Key:    clusterv1alpha1.TaintClusterUnscheduler,
							Effect: corev1.TaintEffectNoSchedule,
						})).
					Should(gomega.Equal(false))
			})
		})
	})
})

var _ = ginkgo.Describe("Karmadactl exec testing", func() {
	var policyName string
	var pod *corev1.Pod
	var policy *policyv1alpha1.PropagationPolicy

	ginkgo.BeforeEach(func() {
		policyName = podNamePrefix + rand.String(RandomStrLength)
		pod = helper.NewPod(testNamespace, podNamePrefix+rand.String(RandomStrLength))
		policy = testhelper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policyName)
			framework.RemovePod(kubeClient, pod.Namespace, pod.Name)
		})
	})

	ginkgo.It("Test exec command", func() {
		framework.WaitPodPresentOnClustersFitWith(framework.ClusterNames(), pod.Namespace, pod.Name,
			func(pod *corev1.Pod) bool {
				return pod.Status.Phase == corev1.PodRunning
			})

		for _, clusterName := range framework.ClusterNames() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, pod.Namespace, karmadactlTimeout, "exec", pod.Name, "-C", clusterName, "--", "echo", "hello")
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		}
	})
})

var _ = ginkgo.Describe("Karmadactl top testing", ginkgo.Labels{NeedCreateCluster}, func() {
	ginkgo.Context("Karmadactl top pod which does not exist", func() {
		ginkgo.It("Karmadactl top pod which does not exist", func() {
			podName := podNamePrefix + rand.String(RandomStrLength)
			for _, clusterName := range framework.ClusterNames() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "top", "pod", podName, "-n", testNamespace, "-C", clusterName)
				_, err := cmd.ExecOrDie()
				gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("pods \"%s\" not found", podName))).To(gomega.BeTrue(), "should not found")
			}
		})
	})

	ginkgo.Context("Karmadactl top pod", func() {
		var policyName string
		var pod *corev1.Pod
		var policy *policyv1alpha1.PropagationPolicy
		ginkgo.BeforeEach(func() {
			// create a pod and a propagationPolicy
			policyName = podNamePrefix + rand.String(RandomStrLength)
			pod = helper.NewPod(testNamespace, podNamePrefix+rand.String(RandomStrLength))
			policy = testhelper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
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

			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreatePod(kubeClient, pod)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policyName)
				framework.RemovePod(kubeClient, pod.Namespace, pod.Name)
			})

			// wait for pod and metrics ready
			framework.WaitPodPresentOnClustersFitWith(framework.ClusterNames(), pod.Namespace, pod.Name,
				func(pod *corev1.Pod) bool {
					return pod.Status.Phase == corev1.PodRunning
				})
			for _, cluster := range framework.ClusterNames() {
				framework.WaitPodMetricsReady(kubeClient, karmadaClient, cluster, pod.Namespace, pod.Name)
			}
		})

		ginkgo.It("Karmadactl top existing pod", func() {
			for _, clusterName := range framework.ClusterNames() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, pod.Namespace, karmadactlTimeout, "top", "pod", pod.Name, "-n", pod.Namespace, "-C", clusterName)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			}
		})

		ginkgo.It("Karmadactl top existing pod without setting cluster flag", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, pod.Namespace, karmadactlTimeout, "top", "pod", pod.Name, "-n", pod.Namespace)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("Karmadactl top pod without specific podName", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, pod.Namespace, karmadactlTimeout, "top", "pod", "-A")
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			for _, clusterName := range framework.ClusterNames() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, pod.Namespace, karmadactlTimeout, "top", "pod", "-A", "-C", clusterName)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			}
		})
	})
})
