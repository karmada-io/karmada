package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	khelper "github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Karmadactl promote testing", func() {
	var karmadaConfig karmadactl.KarmadaConfig
	var member1 string
	var member1Client kubernetes.Interface

	ginkgo.BeforeEach(func() {
		karmadaConfig = karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
		member1 = "member1"
		member1Client = framework.GetClusterClient(member1)
	})

	ginkgo.Context("Test promoting namespaced resource: deployment", func() {
		var deployment *appsv1.Deployment
		var deploymentNamespace, deploymentName string
		var deploymentOpts, namespaceOpts karmadactl.CommandPromoteOption

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
				namespaceOpts = karmadactl.CommandPromoteOption{
					Cluster:          member1,
					ClusterNamespace: options.DefaultKarmadaClusterNamespace,
				}
				args := []string{"namespace", deploymentNamespace}
				// init args: place namespace name to CommandPromoteOption.name
				err := namespaceOpts.Complete(args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				// use karmadactl to promote a namespace from member1
				err = karmadactl.RunPromote(karmadaConfig, namespaceOpts, args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				framework.WaitNamespacePresentOnClusterByClient(kubeClient, deploymentNamespace)
			})

			// Step 3,  promote deployment from cluster member1 to karmada
			ginkgo.By(fmt.Sprintf("Promoting deployment %s from member: %s to karmada", deploymentName, member1), func() {
				deploymentOpts = karmadactl.CommandPromoteOption{
					Namespace:        deploymentNamespace,
					Cluster:          member1,
					ClusterNamespace: options.DefaultKarmadaClusterNamespace,
				}
				args := []string{"deployment", deploymentName}
				// init args: place deployment name to CommandPromoteOption.name
				err := deploymentOpts.Complete(args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				// use karmadactl to promote a deployment from member1
				err = karmadactl.RunPromote(karmadaConfig, deploymentOpts, args)
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

				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentDeployment, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					return framework.CheckDeploymentReadyStatus(currentDeployment, wantedReplicas), nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})

	ginkgo.Context("Test promoting cluster resources: clusterrole and clusterrolebing", func() {

		var clusterRoleName, clusterRoleBindingName string
		var clusterRole *rbacv1.ClusterRole
		var clusterRoleBinding *rbacv1.ClusterRoleBinding
		var clusterRoleOpts, clusterRoleBindingOpts karmadactl.CommandPromoteOption

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
				clusterRoleOpts = karmadactl.CommandPromoteOption{
					Cluster:          member1,
					ClusterNamespace: options.DefaultKarmadaClusterNamespace,
				}

				args := []string{"clusterrole", clusterRoleName}
				// init args: place clusterrole name to CommandPromoteOption.name
				err := clusterRoleOpts.Complete(args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				// use karmadactl to promote clusterrole from member1
				err = karmadactl.RunPromote(karmadaConfig, clusterRoleOpts, args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				clusterRoleBindingOpts = karmadactl.CommandPromoteOption{
					Cluster:          member1,
					ClusterNamespace: options.DefaultKarmadaClusterNamespace,
				}

				args = []string{"clusterrolebinding", clusterRoleBindingName}
				// init args: place clusterrolebinding name to CommandPromoteOption.name
				err = clusterRoleBindingOpts.Complete(args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				// use karmadactl to promote clusterrolebinding from member1
				err = karmadactl.RunPromote(karmadaConfig, clusterRoleBindingOpts, args)
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
			esName, _ = names.GenerateExecutionSpaceName(member1)
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
				opts := karmadactl.CommandPromoteOption{
					Cluster:          member1,
					ClusterNamespace: options.DefaultKarmadaClusterNamespace,
				}
				args := []string{"namespace", serviceNamespace}
				err := opts.Complete(args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = karmadactl.RunPromote(karmadaConfig, opts, args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				framework.WaitNamespacePresentOnClusterByClient(kubeClient, serviceNamespace)
			})

			ginkgo.By(fmt.Sprintf("Promoting service %s from member: %s to karmada control plane", serviceName, member1), func() {
				opts := karmadactl.CommandPromoteOption{
					Namespace:        serviceNamespace,
					Cluster:          member1,
					ClusterNamespace: options.DefaultKarmadaClusterNamespace,
				}
				args := []string{"service", serviceName}
				err := opts.Complete(args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = karmadactl.RunPromote(karmadaConfig, opts, args)
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

var _ = framework.SerialDescribe("Karmadactl unjoin testing", ginkgo.Labels{NeedCreateCluster}, func() {
	ginkgo.Context("unjoining not ready cluster", func() {
		var clusterName string
		var homeDir string
		var kubeConfigPath string
		var clusterContext string
		var controlPlane string
		var deploymentName, deploymentNamespace string
		var policyName, policyNamespace string
		var deployment *appsv1.Deployment
		var policy *policyv1alpha1.PropagationPolicy
		var karmadaConfig karmadactl.KarmadaConfig

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
			karmadaConfig = karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
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

		ginkgo.It("Test unjoining not ready cluster", func() {
			ginkgo.By(fmt.Sprintf("Joinning cluster: %s", clusterName), func() {
				opts := karmadactl.CommandJoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterName,
					ClusterContext:    clusterContext,
					ClusterKubeConfig: kubeConfigPath,
				}
				err := karmadactl.RunJoin(karmadaConfig, opts)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
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
				opts := karmadactl.CommandUnjoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterName,
					ClusterContext:    clusterContext,
					ClusterKubeConfig: kubeConfigPath,
					Wait:              5 * options.DefaultKarmadactlCommandDuration,
				}
				err := karmadactl.RunUnjoin(karmadaConfig, opts)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})
})

var _ = framework.SerialDescribe("Karmadactl cordon/uncordon testing", ginkgo.Labels{NeedCreateCluster}, func() {
	var member1 string
	var controlPlane string
	var clusterName string
	var homeDir string
	var kubeConfigPath string
	var clusterContext string
	var deploymentName, deploymentNamespace string
	var policyName, policyNamespace string
	var deployment *appsv1.Deployment
	var policy *policyv1alpha1.PropagationPolicy
	var karmadaConfig karmadactl.KarmadaConfig

	ginkgo.BeforeEach(func() {
		member1 = "member1"
		clusterName = "member-e2e-" + rand.String(3)
		homeDir = os.Getenv("HOME")
		kubeConfigPath = fmt.Sprintf("%s/.kube/%s.config", homeDir, clusterName)
		controlPlane = fmt.Sprintf("%s-control-plane", clusterName)
		clusterContext = fmt.Sprintf("kind-%s", clusterName)
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
				ClusterNames: []string{member1, clusterName},
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			},
		})
		karmadaConfig = karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By(fmt.Sprintf("Creating cluster: %s", clusterName), func() {
			err := createCluster(clusterName, kubeConfigPath, controlPlane, clusterContext)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})
		ginkgo.By(fmt.Sprintf("Joinning cluster: %s", clusterName), func() {
			karmadaConfig := karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
			opts := karmadactl.CommandJoinOption{
				DryRun:            false,
				ClusterNamespace:  "karmada-cluster",
				ClusterName:       clusterName,
				ClusterContext:    clusterContext,
				ClusterKubeConfig: kubeConfigPath,
			}
			err := karmadactl.RunJoin(karmadaConfig, opts)
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
				opts := karmadactl.CommandUnjoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterName,
					ClusterContext:    clusterContext,
					ClusterKubeConfig: kubeConfigPath,
					Wait:              5 * options.DefaultKarmadactlCommandDuration,
				}
				err := karmadactl.RunUnjoin(karmadaConfig, opts)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.By(fmt.Sprintf("Deleting clusters: %s", clusterName), func() {
				err := deleteCluster(clusterName, kubeConfigPath)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				_ = os.Remove(kubeConfigPath)
			})
		})
	})
	ginkgo.Context("cordon cluster", func() {
		ginkgo.BeforeEach(func() {
			opts := karmadactl.CommandCordonOption{
				DryRun:      false,
				ClusterName: clusterName,
			}
			err := karmadactl.RunCordonOrUncordon(karmadactl.DesiredCordon, karmadaConfig, opts)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			ginkgo.By("cluster should have a taint", func() {
				clusterObj := &clusterv1alpha1.Cluster{}
				err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				taints := clusterObj.Spec.Taints
				var unschedulable corev1.Taint
				for index := range taints {
					if taints[index].Key == clusterv1alpha1.TaintClusterUnscheduler {
						unschedulable = taints[index]
						break
					}
				}
				gomega.Expect(unschedulable).ShouldNot(gomega.BeNil())
				gomega.Expect(unschedulable.Effect).Should(gomega.Equal(corev1.TaintEffectNoSchedule))
			})
		})

		ginkgo.It(fmt.Sprintf("deploy deployment should not schedule to cordon cluster %s", clusterName), func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
			targetClusters := framework.ExtractTargetClustersFrom(controlPlaneClient, deployment)
			gomega.Expect(targetClusters).ShouldNot(gomega.ContainElement(clusterName))
		})

		ginkgo.It("uncordon cluster", func() {
			opts := karmadactl.CommandCordonOption{
				DryRun:      false,
				ClusterName: clusterName,
			}
			err := karmadactl.RunCordonOrUncordon(karmadactl.DesiredUnCordon, karmadaConfig, opts)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			ginkgo.By(fmt.Sprintf("cluster %s taint will be removed", clusterName), func() {
				clusterObj := &clusterv1alpha1.Cluster{}
				err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(clusterObj.Spec.Taints).Should(gomega.BeEmpty())
			})

			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
			ginkgo.By(fmt.Sprintf("deployment will schedule to cluster %s", clusterName), func() {
				targetClusters := framework.ExtractTargetClustersFrom(controlPlaneClient, deployment)
				gomega.Expect(targetClusters).Should(gomega.ContainElement(clusterName))
			})
		})
	})
})
