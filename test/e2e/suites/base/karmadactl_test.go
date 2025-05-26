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

package base

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

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
)

const (
	karmadactlTimeout        = time.Second * 10
	PropagationPolicyPattern = `propagationpolicy\.policy\.karmada\.io\/([a-zA-Z0-9\-]+)`
)

var _ = ginkgo.Describe("Karmadactl promote testing", func() {
	var member1 string
	var member1Client kubernetes.Interface

	ginkgo.BeforeEach(func() {
		member1 = framework.ClusterNames()[0]
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
			clusterName = "member-e2e-" + rand.String(RandomStrLength)
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
				err := framework.AddClusterTaint(controlPlaneClient, clusterName, *framework.NotReadyTaintTemplate)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
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
		clusterName = "member-e2e-" + rand.String(RandomStrLength)
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
		policy = helper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, pod.Namespace, karmadactlTimeout, "exec", pod.Name, "--operation-scope", "members", "--cluster", clusterName, "--", "echo", "hello")
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		}
	})
})

var _ = ginkgo.Describe("Karmadactl top testing", func() {
	ginkgo.Context("Karmadactl top pod which does not exist", func() {
		ginkgo.It("Karmadactl top pod which does not exist", func() {
			podName := podNamePrefix + rand.String(RandomStrLength)
			for _, clusterName := range framework.ClusterNames() {
				gomega.Eventually(func() bool {
					cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "top", "pod", podName, "-n", testNamespace, "-C", clusterName)
					_, err := cmd.ExecOrDie()
					fmt.Printf("Should receive a NotFound error, and actually received: %+v\n", err)
					return err != nil && strings.Contains(err.Error(), fmt.Sprintf("pods \"%s\" not found", podName))
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
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
			policy = helper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
			ginkgo.By("Karmadactl top existing pod", func() {
				for _, clusterName := range framework.ClusterNames() {
					cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, pod.Namespace, karmadactlTimeout, "top", "pod", pod.Name, "-n", pod.Namespace, "-C", clusterName)
					_, err := cmd.ExecOrDie()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By("Karmadactl top existing pod without setting cluster flag", func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, pod.Namespace, karmadactlTimeout, "top", "pod", pod.Name, "-n", pod.Namespace)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("Karmadactl top pod without specific podName", func() {
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
})

var _ = ginkgo.Describe("Karmadactl logs testing", func() {
	var member1 string
	var member1Client kubernetes.Interface

	ginkgo.BeforeEach(func() {
		member1 = framework.ClusterNames()[0]
		member1Client = framework.GetClusterClient(member1)
	})

	waitForPodReady := func(namespace, podName string) {
		framework.WaitPodPresentOnClusterFitWith(member1, namespace, podName, func(pod *corev1.Pod) bool {
			return pod.Status.Phase == corev1.PodRunning
		})
	}

	ginkgo.Context("Test karmadactl logs for existing pod", func() {
		var (
			namespace, podName string
			ns                 *corev1.Namespace
			pod                *corev1.Pod
		)

		ginkgo.BeforeEach(func() {
			namespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			podName = podNamePrefix + rand.String(RandomStrLength)
			pod = helper.NewPod(namespace, podName)
			ns = helper.NewNamespace(namespace)

			// Create the namespace and pod in the member cluster.
			_, err := member1Client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			_, err = member1Client.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			waitForPodReady(namespace, podName)
		})

		ginkgo.AfterEach(func() {
			err := member1Client.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("should get logs of the existing pod successfully", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "logs", podName, "-C", member1)
			output, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(output).ShouldNot(gomega.BeEmpty())
		})

		ginkgo.It("should get logs from a specific container in the pod", func() {
			containerName := pod.Spec.Containers[0].Name
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "logs", podName, "-c", containerName, "-C", member1)
			output, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(output).ShouldNot(gomega.BeEmpty())
		})

		ginkgo.It("should return error for non-existing container in an existing pod", func() {
			nonExistentContainer := "non-existent-container"
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "logs", podName, "-c", nonExistentContainer, "-C", member1)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("container %s is not valid for pod %s", nonExistentContainer, podName))).Should(gomega.BeTrue())
		})
	})

	ginkgo.Context("Test karmadactl logs for non-existing pod", func() {
		var namespace, podName string
		var ns *corev1.Namespace

		ginkgo.BeforeEach(func() {
			namespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			podName = podNamePrefix + rand.String(RandomStrLength)
			ns = helper.NewNamespace(namespace)

			// Create the namespace in the member cluster.
			_, err := member1Client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.AfterEach(func() {
			err := member1Client.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("should return not found error for non-existing pod", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "logs", podName, "-C", member1)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("pods \"%s\" not found", podName))).Should(gomega.BeTrue())
		})

		ginkgo.It("should return not found error for non-existing container in non-existing pod", func() {
			nonExistentContainer := "non-existent-container"
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "logs", podName, "-c", nonExistentContainer, "-C", member1)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("pods \"%s\" not found", podName))).Should(gomega.BeTrue())
		})
	})

	ginkgo.Context("Test karmadactl logs for non-existing pod in non-existing cluster", func() {
		var nonExistentCluster string
		var namespace, podName string

		ginkgo.BeforeEach(func() {
			namespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			podName = podNamePrefix + rand.String(RandomStrLength)
			nonExistentCluster = "non-existent-cluster"
		})

		ginkgo.It("should return error for non-existing cluster", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "logs", podName, "-C", nonExistentCluster)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("clusters.cluster.karmada.io \"%s\" not found", nonExistentCluster))).Should(gomega.BeTrue())
		})
	})

	ginkgo.Context("Test karmadactl logs with invalid input", func() {
		var (
			namespace, podName string
			ns                 *corev1.Namespace
			pod                *corev1.Pod
		)

		ginkgo.BeforeEach(func() {
			namespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			podName = podNamePrefix + rand.String(RandomStrLength)
			ns = helper.NewNamespace(namespace)

			// Create the namespace and pod in the member cluster.
			_, err := member1Client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			pod = helper.NewPod(namespace, podName)
			_, err = member1Client.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			waitForPodReady(namespace, podName)
		})

		ginkgo.AfterEach(func() {
			err := member1Client.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("should return error for invalid flag", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "logs", podName, "--invalidflag", "-C", member1)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), "unknown flag: --invalidflag")).Should(gomega.BeTrue())
		})
	})
})

var _ = ginkgo.Describe("Karmadactl version testing", func() {
	ginkgo.Context("Test karmadactl version command", func() {
		ginkgo.It("should return the version information successfully", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, "", karmadactlPath, "", karmadactlTimeout, "version")
			output, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(output, "karmadactl version")).Should(gomega.BeTrue())
			gomega.Expect(strings.Contains(output, "GitVersion")).Should(gomega.BeTrue())
			gomega.Expect(strings.Contains(output, "GoVersion")).Should(gomega.BeTrue())
			gomega.Expect(strings.Contains(output, "Platform")).Should(gomega.BeTrue())
		})

		ginkgo.It("should return error for invalid flag", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, "", karmadactlPath, "", karmadactlTimeout, "version", "--invalidflag")
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), "unknown flag: --invalidflag")).Should(gomega.BeTrue())
		})
	})
})

var _ = ginkgo.Describe("Karmadactl get testing", func() {
	var member1 string
	var member1Client kubernetes.Interface

	ginkgo.BeforeEach(func() {
		member1 = framework.ClusterNames()[0]
		member1Client = framework.GetClusterClient(member1)
	})

	ginkgo.Context("Test operation scope switching", func() {
		var deployment *appsv1.Deployment
		var propagationPolicy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			deployment = helper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
			propagationPolicy = helper.NewPropagationPolicy(deployment.Namespace, ppNamePrefix+rand.String(RandomStrLength), []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: framework.ClusterNames()},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateDeployment(kubeClient, deployment)
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)

			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			})
		})

		ginkgo.It("should switch operation scope successfully", func() {
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(*appsv1.Deployment) bool {
					return true
				})

			ginkgo.By("should display the deployment in Karmada control plane", func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, deployment.Namespace, karmadactlTimeout, "get", "deployment", deployment.Name)
				output, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(strings.Contains(output, "Karmada")).Should(gomega.BeTrue())
				gomega.Expect(strings.Contains(output, member1)).ShouldNot(gomega.BeTrue())
			})

			ginkgo.By("should display the deployment in Karmada control plane and member1", func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, deployment.Namespace, karmadactlTimeout, "get", "deployment", deployment.Name, "--operation-scope", "all", "-C", member1)
				output, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(strings.Contains(output, "Karmada")).Should(gomega.BeTrue())
				gomega.Expect(strings.Contains(output, member1)).Should(gomega.BeTrue())
				gomega.Expect(strings.Contains(output, framework.ClusterNames()[1])).ShouldNot(gomega.BeTrue())
			})

			ginkgo.By("should display the deployment in Karmada control plane and all member clusters", func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, deployment.Namespace, karmadactlTimeout, "get", "deployment", deployment.Name, "--operation-scope", "all")
				output, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(strings.Contains(output, "Karmada")).Should(gomega.BeTrue())
				gomega.Expect(strings.Contains(output, member1)).Should(gomega.BeTrue())
				gomega.Expect(strings.Contains(output, framework.ClusterNames()[1])).Should(gomega.BeTrue())
			})

			ginkgo.By("should ignore resources in Karmada control plane", func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, deployment.Namespace, karmadactlTimeout, "get", "deployment", deployment.Name, "--operation-scope", "members")
				output, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(strings.Contains(output, "Karmada")).ShouldNot(gomega.BeTrue())
			})
		})
	})

	ginkgo.Context("Test karmadactl get for existing resource", func() {
		var (
			namespace, podName string
			ns                 *corev1.Namespace
			pod                *corev1.Pod
		)

		ginkgo.BeforeEach(func() {
			namespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			podName = podNamePrefix + rand.String(RandomStrLength)
			pod = helper.NewPod(namespace, podName)
			ns = helper.NewNamespace(namespace)

			// Create the namespace and pod in the member cluster.
			_, err := member1Client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			_, err = member1Client.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.AfterEach(func() {
			err := member1Client.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("should get the existing pod successfully", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "get", "pods", podName, "--operation-scope", "members", "-C", member1)
			output, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(output, podName)).Should(gomega.BeTrue())
		})
	})

	ginkgo.Context("Test karmadactl get for non-existing resource", func() {
		var (
			namespace, podName string
			ns                 *corev1.Namespace
		)

		ginkgo.BeforeEach(func() {
			namespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			podName = podNamePrefix + rand.String(RandomStrLength)
			ns = helper.NewNamespace(namespace)

			// Create the namespace in the member cluster.
			_, err := member1Client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.AfterEach(func() {
			err := member1Client.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("should return not found error for non-existing pod", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "get", "pods", podName, "--operation-scope", "members", "-C", member1)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("pods \"%s\" not found", podName))).Should(gomega.BeTrue())
		})
	})

	ginkgo.Context("Test karmadactl get for non-existing resource in non-existing namespace", func() {
		var namespace, podName string

		ginkgo.BeforeEach(func() {
			namespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			podName = podNamePrefix + rand.String(RandomStrLength)
		})

		ginkgo.It("should return not found error for non-existing namespace", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "get", "pods", podName, "--operation-scope", "members", "-C", member1)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("namespaces \"%s\" not found", namespace))).Should(gomega.BeTrue())
		})
	})

	ginkgo.Context("Test karmadactl get with invalid input", func() {
		var namespace string

		ginkgo.BeforeEach(func() {
			namespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
		})

		ginkgo.It("should return error for invalid resource type", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "get", "invalidresource", "invalidname")
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), "the server doesn't have a resource type \"invalidresource\"")).Should(gomega.BeTrue())
		})
	})
})

var _ = ginkgo.Describe("Karmadactl describe testing", func() {
	ginkgo.Context("Test karmadactl describe for existing resource", func() {
		var deployment *appsv1.Deployment
		var propagationPolicy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			deployment = helper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
			propagationPolicy = helper.NewPropagationPolicy(deployment.Namespace, ppNamePrefix+rand.String(RandomStrLength), []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: framework.ClusterNames()},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateDeployment(kubeClient, deployment)
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)

			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			})
		})

		ginkgo.It("should describe the existing pod successfully", func() {
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(*appsv1.Deployment) bool {
					return true
				})
			ginkgo.By("should describe resources in Karmada control plane", func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, testNamespace, karmadactlTimeout, "describe", "deployments", deployment.Name)
				output, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(strings.Contains(output, deployment.Name)).Should(gomega.BeTrue())
			})
			ginkgo.By("should describe resources in member1", func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, testNamespace, karmadactlTimeout, "describe", "deployments", deployment.Name, "--operation-scope", "members", "--cluster", framework.ClusterNames()[0])
				output, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(strings.Contains(output, deployment.Name)).Should(gomega.BeTrue())
			})
		})
	})

	ginkgo.Context("Test karmadactl describe for non-existing resource", func() {
		var podName string

		ginkgo.BeforeEach(func() {
			podName = podNamePrefix + rand.String(RandomStrLength)
		})

		ginkgo.It("should return not found error for non-existing pod", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, testNamespace, karmadactlTimeout, "describe", "pods", podName)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("pods \"%s\" not found", podName))).Should(gomega.BeTrue())
		})
	})

	ginkgo.Context("Test karmadactl describe for non-existing resource in non-existing namespace", func() {
		var namespace, podName string

		ginkgo.BeforeEach(func() {
			namespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			podName = podNamePrefix + rand.String(RandomStrLength)
		})

		ginkgo.It("should return not found error for non-existing namespace", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "describe", "pods", podName)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("namespaces \"%s\" not found", namespace))).Should(gomega.BeTrue())
		})
	})

	ginkgo.Context("Test karmadactl describe with invalid input", func() {
		var namespace string

		ginkgo.BeforeEach(func() {
			namespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
		})

		ginkgo.It("should return error for invalid resource type", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, namespace, karmadactlTimeout, "describe", "invalidresource", "invalidname")
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), "the server doesn't have a resource type \"invalidresource\"")).Should(gomega.BeTrue())
		})
	})
})

var _ = ginkgo.Describe("Karmadactl token testing", func() {
	ginkgo.Context("Test creating a bootstrap token", func() {
		var (
			token string
			err   error
		)

		ginkgo.AfterEach(func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "delete", token)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("should create a token successfully and validate its format", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "create")
			token, err = cmd.ExecOrDie()
			token = strings.TrimSpace(token)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			// Validate the token format.
			tokenID, tokenSecret := extractTokenIDAndSecret(token)
			gomega.Expect(validateTokenFormat(tokenID, tokenSecret)).Should(gomega.BeTrue(), "Token format is invalid")
		})
	})

	ginkgo.Context("Test deleting a bootstrap token", func() {
		var tokenID, tokenSecret string

		ginkgo.BeforeEach(func() {
			// Create a token to be deleted in the test.
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "create")
			token, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			tokenID, tokenSecret = extractTokenIDAndSecret(token)
		})

		ginkgo.It("should delete a token successfully by ID", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "delete", tokenID)
			output, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(output, fmt.Sprintf("bootstrap token %q deleted", tokenID))).Should(gomega.BeTrue())
		})

		ginkgo.It("should delete a token successfully by both ID and Secret", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "delete", fmt.Sprintf("%s.%s", tokenID, tokenSecret))
			output, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(output, fmt.Sprintf("bootstrap token %q deleted", tokenID))).Should(gomega.BeTrue())
		})

		ginkgo.It("Invalid delete operation", func() {
			ginkgo.By("should fail to delete a token by secret only", func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "delete", tokenSecret)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(strings.Contains(err.Error(), "given token didn't match pattern")).Should(gomega.BeTrue())
			})

			ginkgo.By("should fail to delete a valid token already deleted before", func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "delete", tokenID)
				output, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(strings.Contains(output, fmt.Sprintf("bootstrap token %q deleted", tokenID))).Should(gomega.BeTrue())

				cmd = framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "delete", tokenID)
				_, err = cmd.ExecOrDie()
				gomega.Expect(err).Should(gomega.HaveOccurred())
				gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("err: secrets \"bootstrap-token-%s\" not found", tokenID))).Should(gomega.BeTrue())
			})
		})
	})

	ginkgo.Context("Test listing bootstrap tokens", func() {
		var (
			token string
			err   error
		)

		ginkgo.BeforeEach(func() {
			// Create a token to be deleted in the test.
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "create")
			token, err = cmd.ExecOrDie()
			token = strings.TrimSpace(token)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.AfterEach(func() {
			// Check if the previous test case failed.
			if ginkgo.CurrentSpecReport().Failed() {
				// Attempt to delete the token to ensure clean state for subsequent tests,
				// regardless of whether it exists or not.
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "delete", token)
				_, _ = cmd.ExecOrDie()
			}
		})

		ginkgo.It("list operation test", func() {
			ginkgo.By("Listing tokens successfully", func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "list")
				output, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(strings.Contains(output, token)).Should(gomega.BeTrue())
			})

			ginkgo.By("Deleting the token and verifying the token is not listed", func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "delete", token)
				_, err = cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				cmd = framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "list")
				output, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(strings.Contains(output, token)).Should(gomega.BeFalse())
			})
		})
	})

	ginkgo.Context("Test invalid flags", func() {
		ginkgo.It("should return error for invalid flag in create", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "create", "--invalidflag")
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), "unknown flag: --invalidflag")).Should(gomega.BeTrue())
		})

		ginkgo.It("should return error for invalid flag in delete", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "delete", "--invalidflag")
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), "unknown flag: --invalidflag")).Should(gomega.BeTrue())
		})

		ginkgo.It("should return error for invalid flag in list", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "token", "list", "--invalidflag")
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), "unknown flag: --invalidflag")).Should(gomega.BeTrue())
		})
	})
})

var _ = ginkgo.Describe("Karmadactl options testing", func() {
	ginkgo.Context("Test karmadactl options command", func() {
		ginkgo.It("should list available options successfully", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, "", karmadactlPath, "", karmadactlTimeout, "options")
			output, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(output, "The following options can be passed to any command")).Should(gomega.BeTrue())
		})

		ginkgo.It("should return error for invalid flag in options", func() {
			cmd := framework.NewKarmadactlCommand(kubeconfig, "", karmadactlPath, "", karmadactlTimeout, "options", "--invalidflag")
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(strings.Contains(err.Error(), "unknown flag: --invalidflag")).Should(gomega.BeTrue())
		})
	})
})

var _ = framework.SerialDescribe("Karmadactl taint testing", ginkgo.Labels{NeedCreateCluster}, func() {
	ginkgo.Context("Test karmadactl taint command with different effects", func() {
		var (
			newClusterName, clusterContext        string
			homeDir, kubeConfigPath, controlPlane string
			taintKey                              = "environment"
			taintProductionValue                  = "production"
			taintTestValue                        = "test"
		)

		ginkgo.BeforeEach(func() {
			newClusterName = "member-e2e-" + rand.String(RandomStrLength)
			homeDir = os.Getenv("HOME")
			kubeConfigPath = fmt.Sprintf("%s/.kube/%s.config", homeDir, newClusterName)
			controlPlane = fmt.Sprintf("%s-control-plane", newClusterName)
			clusterContext = fmt.Sprintf("kind-%s", newClusterName)
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Creating cluster: %s", newClusterName), func() {
				err := createCluster(newClusterName, kubeConfigPath, controlPlane, clusterContext)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Joining cluster: %s", newClusterName), func() {
				cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout,
					"join", "--cluster-kubeconfig", kubeConfigPath, "--cluster-context", clusterContext, "--cluster-namespace", "karmada-cluster", newClusterName)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("wait cluster %s ready", newClusterName), func() {
				framework.WaitClusterFitWith(controlPlaneClient, newClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
					return meta.IsStatusConditionPresentAndEqual(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
				})
			})

			ginkgo.DeferCleanup(func() {
				ginkgo.By(fmt.Sprintf("Unjoinning cluster: %s", newClusterName), func() {
					cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", 5*options.DefaultKarmadactlCommandDuration,
						"unjoin", "--cluster-kubeconfig", kubeConfigPath, "--cluster-context", clusterContext, "--cluster-namespace", "karmada-cluster", newClusterName)
					_, err := cmd.ExecOrDie()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				ginkgo.By(fmt.Sprintf("Deleting clusters: %s", newClusterName), func() {
					err := deleteCluster(newClusterName, kubeConfigPath)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					_ = os.Remove(kubeConfigPath)
				})
			})
		})

		ginkgo.It("should handle taint correctly", func() {
			// testTaintEffect is a reusable function for testing taints on a given effect.
			testTaintEffect := func(effect corev1.TaintEffect) {
				productionTaint := fmt.Sprintf("%s=%s:%s", taintKey, taintProductionValue, effect)
				testTaint := fmt.Sprintf("%s=%s:%s", taintKey, taintTestValue, effect)

				ginkgo.By(fmt.Sprintf("Verifying that %s taint is not applied during dry-run mode", productionTaint), func() {
					cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "taint", "clusters", newClusterName, productionTaint, "--dry-run")
					output, err := cmd.ExecOrDie()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(strings.Contains(output, fmt.Sprintf("cluster/%s tainted", newClusterName))).Should(gomega.BeTrue())
					framework.WaitClusterFitWith(controlPlaneClient, newClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
						for _, taint := range cluster.Spec.Taints {
							if taint.Key == taintKey && taint.Value == taintProductionValue && taint.Effect == effect {
								return false
							}
						}
						return true
					})
				})

				ginkgo.By(fmt.Sprintf("Applying %s taint to the cluster", productionTaint), func() {
					cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "taint", "clusters", newClusterName, productionTaint)
					output, err := cmd.ExecOrDie()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(strings.Contains(output, fmt.Sprintf("cluster/%s tainted", newClusterName))).Should(gomega.BeTrue())
					framework.WaitClusterFitWith(controlPlaneClient, newClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
						for _, taint := range cluster.Spec.Taints {
							if taint.Key == taintKey && taint.Value == taintProductionValue && taint.Effect == effect {
								return true
							}
						}
						return false
					})
				})

				ginkgo.By(fmt.Sprintf("Overwriting %s taint with %s taint without --overwrite flag", productionTaint, testTaint), func() {
					cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "taint", "clusters", newClusterName, testTaint)
					_, err := cmd.ExecOrDie()
					gomega.Expect(err).Should(gomega.HaveOccurred())
					gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("cluster %s already has environment taint(s) with same effect(s) and --overwrite is false", newClusterName))).Should(gomega.BeTrue())
					framework.WaitClusterFitWith(controlPlaneClient, newClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
						for _, taint := range cluster.Spec.Taints {
							if taint.Key == taintKey && taint.Value == taintProductionValue && taint.Effect == effect {
								return true
							}
						}
						return false
					})
				})

				ginkgo.By(fmt.Sprintf("Overwriting %s taint with %s taint using --overwrite flag", productionTaint, testTaint), func() {
					cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "taint", "clusters", newClusterName, testTaint, "--overwrite")
					output, err := cmd.ExecOrDie()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(strings.Contains(output, fmt.Sprintf("cluster/%s modified", newClusterName))).Should(gomega.BeTrue())
					framework.WaitClusterFitWith(controlPlaneClient, newClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
						for _, taint := range cluster.Spec.Taints {
							if taint.Key == taintKey && taint.Value == taintTestValue && taint.Effect == effect {
								return true
							}
						}
						return false
					})
				})

				ginkgo.By(fmt.Sprintf("Removing %s taint from the cluster", testTaint), func() {
					cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "taint", "clusters", newClusterName, fmt.Sprintf("%s-", testTaint))
					output, err := cmd.ExecOrDie()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(strings.Contains(output, "untainted")).Should(gomega.BeTrue())
					framework.WaitClusterFitWith(controlPlaneClient, newClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
						for _, taint := range cluster.Spec.Taints {
							if taint.Key == taintKey && taint.Value == taintTestValue && taint.Effect == effect {
								return false
							}
						}
						return true
					})
				})

				ginkgo.By("Returning an error for invalid flag in taint command", func() {
					cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "taint", "clusters", newClusterName, productionTaint, "--invalidflag")
					_, err := cmd.ExecOrDie()
					gomega.Expect(err).Should(gomega.HaveOccurred())
					gomega.Expect(strings.Contains(err.Error(), "unknown flag: --invalidflag")).Should(gomega.BeTrue())
				})

				ginkgo.By("Returning an error if cluster does not exist", func() {
					nonExistentCluster := "non-existent-cluster"
					cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "taint", "clusters", nonExistentCluster, productionTaint)
					_, err := cmd.ExecOrDie()
					gomega.Expect(err).Should(gomega.HaveOccurred())
					gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("clusters.cluster.karmada.io \"%s\" not found", nonExistentCluster))).Should(gomega.BeTrue())
				})
			}

			// Call testTaintEffect function for each taint effect: NoSchedule, NoExecute.
			testTaintEffect(corev1.TaintEffectNoSchedule)
			testTaintEffect(corev1.TaintEffectNoExecute)
		})
	})
})

var _ = ginkgo.Describe("Karmadactl apply testing", func() {
	var (
		member1Name        string
		member1Client      kubernetes.Interface
		deploymentManifest string
		deployment         *appsv1.Deployment
	)

	ginkgo.BeforeEach(func() {
		member1Name = framework.ClusterNames()[0]
		member1Client = framework.GetClusterClient(member1Name)
	})

	ginkgo.BeforeEach(func() {
		deployment = helper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
		deploymentManifest = fmt.Sprintf("/tmp/%s.yaml", deployment.Name)
		err := WriteYamlToFile(deployment, deploymentManifest)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		ginkgo.DeferCleanup(func() {
			os.Remove(deploymentManifest)
		})
	})

	ginkgo.It("should apply configuration without propagating into member clusters", func() {
		// Apply configuration without propagation to member clusters.
		cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "apply", "-f", deploymentManifest)
		output, err := cmd.ExecOrDie()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("deployment.apps/%s created", deployment.Name)))
		defer framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)

		// Check that the deployment is created on karmada control plane.
		framework.WaitDeploymentFitWith(kubeClient, deployment.Namespace, deployment.Name, func(_ *appsv1.Deployment) bool {
			return true
		})
	})

	ginkgo.It("should apply configuration with propagation into specific member clusters", func() {
		// Apply configuration with propagation to member1 cluster.
		cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "apply", "-f", deploymentManifest, "--cluster", member1Name)
		output, err := cmd.ExecOrDie()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("deployment.apps/%s created", deployment.Name)))
		gomega.Expect(output).Should(gomega.MatchRegexp(PropagationPolicyPattern + ` created`))
		defer framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)

		// Extract propagation policy name.
		propagationPolicyName, err := extractPropagationPolicyName(output)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		defer framework.RemovePropagationPolicy(karmadaClient, testNamespace, propagationPolicyName)

		// Check that the propagation policy is created on the karmada control plane in the given namespace.
		framework.WaitPropagationPolicyFitWith(karmadaClient, testNamespace, propagationPolicyName, func(_ *policyv1alpha1.PropagationPolicy) bool {
			return true
		})

		// Check that the deployment is created on the karmada control plane.
		framework.WaitDeploymentFitWith(kubeClient, deployment.Namespace, deployment.Name, func(_ *appsv1.Deployment) bool {
			return true
		})

		// Check that the deployment is propagated to the specified member1 cluster.
		framework.WaitClusterFitWith(controlPlaneClient, member1Name, func(_ *clusterv1alpha1.Cluster) bool {
			_, err := member1Client.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
			return err == nil
		})
	})

	ginkgo.It("should apply the configuration and propagate to all member clusters", func() {
		// Apply configuration and propagate to all member clusters.
		cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "apply", "-f", deploymentManifest, "--all-clusters")
		output, err := cmd.ExecOrDie()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("deployment.apps/%s created", deployment.Name)))
		gomega.Expect(output).Should(gomega.MatchRegexp(PropagationPolicyPattern + ` created`))
		defer framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)

		// Extract propagation policy name.
		propagationPolicyName, err := extractPropagationPolicyName(output)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		defer framework.RemovePropagationPolicy(karmadaClient, testNamespace, propagationPolicyName)

		// Check that the propagation policy is created on the karmada control plane in the given namespace.
		framework.WaitPropagationPolicyFitWith(karmadaClient, testNamespace, propagationPolicyName, func(_ *policyv1alpha1.PropagationPolicy) bool {
			return true
		})

		// Check that the deployment is created on the karmada control plane.
		framework.WaitDeploymentFitWith(kubeClient, deployment.Namespace, deployment.Name, func(_ *appsv1.Deployment) bool {
			return true
		})

		// Check that the deployment is propagated to all member clusters.
		framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name, func(_ *appsv1.Deployment) bool {
			return true
		})
	})

	ginkgo.It("should run in dry-run mode without making any server requests", func() {
		// Apply configuration in dry-run mode.
		cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "apply", "-f", deploymentManifest, "--dry-run=client")
		output, err := cmd.ExecOrDie()
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("deployment.apps/%s", deployment.Name)))

		// Check that the deployment is not created on karmada control plane.
		gomega.Eventually(func() bool {
			_, err = kubeClient.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, pollTimeout, pollInterval).Should(gomega.Equal(true))
	})

	ginkgo.It("should return error for invalid flag", func() {
		// Apply configuration with invalid flag.
		cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "apply", "-f", deploymentManifest, "--invalidflag")
		_, err := cmd.ExecOrDie()
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(strings.Contains(err.Error(), "unknown flag: --invalidflag")).Should(gomega.BeTrue())
	})

	ginkgo.It("should return error if file does not exist", func() {
		// Apply configuration from non-existent file.
		nonExistentFile := "/tmp/non-existent.yaml"
		cmd := framework.NewKarmadactlCommand(kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout, "apply", "-f", nonExistentFile)
		_, err := cmd.ExecOrDie()
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(strings.Contains(err.Error(), fmt.Sprintf("error: the path \"%s\" does not exist", nonExistentFile))).Should(gomega.BeTrue())
	})
})

// WriteYamlToFile writes the provided object as a YAML file to the specified path.
//
// Parameters:
// - obj: The object to be marshaled to YAML.
// - filePath: The path where the YAML file will be written.
//
// Returns:
// - error: An error if there was an issue during the process, otherwise nil.
func WriteYamlToFile(obj interface{}, filePath string) error {
	// Marshal the object to YAML.
	yamlData, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}

	// Create the file.
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the YAML data to the file.
	_, err = file.Write(yamlData)
	if err != nil {
		return err
	}

	return nil
}

var _ = framework.SerialDescribe("Karmadactl register testing", ginkgo.Ordered, ginkgo.Labels{NeedCreateCluster}, func() {
	var (
		newClusterName, clusterContext                 string
		homeDir, kubeConfigPath, controlPlane          string
		karmadaAPIEndpoint, karmadaAPIEndpointExpected string
		token, discoveryTokenCACertHash                string
	)

	ginkgo.BeforeAll(func() {
		ginkgo.By("Initialize dependencies", func() {
			// Initialize member cluster variables.
			newClusterName = "member-e2e-" + rand.String(RandomStrLength)
			homeDir = os.Getenv("HOME")
			kubeConfigPath = fmt.Sprintf("%s/.kube/%s.config", homeDir, newClusterName)
			controlPlane = fmt.Sprintf("%s-control-plane", newClusterName)
			clusterContext = fmt.Sprintf("kind-%s", newClusterName)
		})

		ginkgo.By(fmt.Sprintf("Creating cluster: %s", newClusterName), func() {
			err := createCluster(newClusterName, kubeConfigPath, controlPlane, clusterContext)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.By("Extract Karmada API server endpoint", func() {
			var err error
			karmadaAPIEndpointExpected, err = extractAPIServerEndpoint(kubeconfig, "karmada-apiserver")
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.By("Generate token and discovery token CA cert hash", func() {
			cmd := framework.NewKarmadactlCommand(
				kubeconfig, karmadaContext, karmadactlPath, "", karmadactlTimeout,
				"token", "create", "--print-register-command="+"true",
			)
			output, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			// Extract the endpoint for Karmada APIServer.
			endpointRegex := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d{1,5})`)
			karmadaAPIEndpoint = endpointRegex.FindString(output)
			gomega.Expect(karmadaAPIEndpoint).Should(gomega.Equal(karmadaAPIEndpointExpected))

			// Extract token.
			tokenRegex := regexp.MustCompile(`--token\s+(\S+)`)
			tokenMatches := tokenRegex.FindStringSubmatch(output)
			gomega.Expect(len(tokenMatches)).Should(gomega.BeNumerically(">", 1))
			token = tokenMatches[1]

			// Extract discovery token CA cert hash.
			hashRegex := regexp.MustCompile(`--discovery-token-ca-cert-hash\s+(\S+)`)
			hashMatches := hashRegex.FindStringSubmatch(output)
			gomega.Expect(len(hashMatches)).Should(gomega.BeNumerically(">", 1))
			discoveryTokenCACertHash = hashMatches[1]
		})
	})

	ginkgo.AfterEach(func() {
		ginkgo.By(fmt.Sprintf("Unregistering cluster: %s", newClusterName), func() {
			cmd := framework.NewKarmadactlCommand(
				kubeconfig, karmadaContext, karmadactlPath, "", 5*options.DefaultKarmadactlCommandDuration,
				"unregister", "--cluster-kubeconfig", kubeConfigPath, newClusterName,
			)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})
	})

	ginkgo.AfterAll(func() {
		ginkgo.By(fmt.Sprintf("Deleting clusters: %s", newClusterName), func() {
			err := deleteCluster(newClusterName, kubeConfigPath)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			_ = os.Remove(kubeConfigPath)
		})
	})

	ginkgo.It("should register a cluster without CA verification", func() {
		ginkgo.By("Register the new cluster in Karmada without CA verification", func() {
			cmd := framework.NewKarmadactlCommand(
				"", karmadaContext, karmadactlPath, "", karmadactlTimeout*5, "register", karmadaAPIEndpoint, "--token", token,
				"--discovery-token-unsafe-skip-ca-verification="+"true", "--kubeconfig="+kubeConfigPath,
				"--cluster-name", newClusterName, "--karmada-agent-image", "docker.io/karmada/karmada-agent:latest",
			)
			output, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("cluster(%s) is joined successfully", newClusterName)))
		})

		ginkgo.By("Wait for the new cluster to be ready", func() {
			framework.WaitClusterFitWith(controlPlaneClient, newClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
				return meta.IsStatusConditionPresentAndEqual(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
			})
		})
	})

	ginkgo.It("should register a cluster with CA verification", func() {
		ginkgo.By("Register the new cluster in Karmada with CA verification", func() {
			cmd := framework.NewKarmadactlCommand(
				"", karmadaContext, karmadactlPath, "", karmadactlTimeout*5, "register", karmadaAPIEndpoint,
				"--token", token, "--discovery-token-ca-cert-hash", discoveryTokenCACertHash,
				"--kubeconfig="+kubeConfigPath, "--cluster-name", newClusterName,
				"--karmada-agent-image", "docker.io/karmada/karmada-agent:latest",
			)
			output, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("cluster(%s) is joined successfully", newClusterName)))
		})

		ginkgo.By("Wait for the new cluster to be ready", func() {
			framework.WaitClusterFitWith(controlPlaneClient, newClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
				return meta.IsStatusConditionPresentAndEqual(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
			})
		})
	})
})

// extractAPIServerEndpoint extracts the given clusterName API endpoint from a kubeconfig file.
//
// Parameters:
// - kubeConfigPath: The file path to the kubeconfig file.
//
// Returns:
// - string: The extracted API server endpoint without the "https://" prefix.
// - error: An error if:
//   - the kubeconfig file cannot be read
//   - the YAML format is invalid
//   - the given clusterName is not found
func extractAPIServerEndpoint(kubeConfigPath string, clusterName string) (string, error) {
	config, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		return "", err
	}

	for name, cluster := range config.Clusters {
		if name == clusterName {
			endpointWithoutPrefix := strings.TrimPrefix(cluster.Server, "https://")
			return endpointWithoutPrefix, nil
		}
	}

	return "", fmt.Errorf("%s endpoint not found in kubeconfig", clusterName)
}

// extractPropagationPolicyName extracts the propagation policy name from the input string.
//
// Parameters:
// - input: The string containing the propagation policy information.
//
// Returns:
// - string: The extracted propagation policy name.
// - error: An error if the propagation policy name could not be found.
func extractPropagationPolicyName(input string) (string, error) {
	re := regexp.MustCompile(PropagationPolicyPattern)
	matches := re.FindStringSubmatch(input)
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("no match found")
}

// extractTokenIDAndSecret extracts the token ID and Secret from the output string.
// It assumes the output format is "tokenID.tokenSecret".
//
// Parameters:
// - output: The string containing the token information.
//
// Returns:
// - tokenID: The extracted token ID.
// - tokenSecret: The extracted token Secret.
// If the output format is incorrect, both return values will be empty strings.
func extractTokenIDAndSecret(output string) (string, string) {
	parts := strings.Split(output, ".")
	if len(parts) == 2 {
		tokenID := strings.TrimSpace(parts[0])
		tokenSecret := strings.TrimSpace(parts[1])
		return tokenID, tokenSecret
	}
	return "", ""
}

// validateTokenFormat validates the format of the token ID and Secret.
//
// Parameters:
// - tokenID: The token ID to be validated.
// - tokenSecret: The token Secret to be validated.
//
// Returns:
// - bool: True if both the token ID and token Secret match their respective patterns, otherwise false.
//
// The token ID should be a lowercase alphanumeric string of exactly 6 characters.
// The token Secret should be a lowercase alphanumeric string of exactly 16 characters.
func validateTokenFormat(tokenID, tokenSecret string) bool {
	token := fmt.Sprintf("%s.%s", tokenID, tokenSecret)
	bootstrapTokenRegex := regexp.MustCompile(bootstrapapi.BootstrapTokenPattern)
	return bootstrapTokenRegex.MatchString(token)
}
