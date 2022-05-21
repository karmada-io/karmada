package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
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

		ginkgo.BeforeEach(func() {
			deploymentNamespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveDeployment(kubeClient, deploymentNamespace, deploymentName)
			framework.RemovePropagationPolicy(karmadaClient, deploymentNamespace, deploymentName+"-propagation")
			framework.RemoveNamespace(kubeClient, deploymentNamespace)
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
				opts := karmadactl.CommandPromoteOption{
					Cluster:          member1,
					ClusterNamespace: options.DefaultKarmadaClusterNamespace,
				}
				args := []string{"namespace", deploymentNamespace}
				// init args: place namespace name to CommandPromoteOption.name
				err := opts.Complete(args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				// use karmadactl to promote a namespace from member1
				err = karmadactl.RunPromote(karmadaConfig, opts, args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				framework.WaitNamespacePresentOnClusterByClient(kubeClient, deploymentNamespace)
			})

			// Step 3,  promote deployment from cluster member1 to karmada
			ginkgo.By(fmt.Sprintf("Promoting deployment %s from member: %s to karmada", deploymentName, member1), func() {
				opts := karmadactl.CommandPromoteOption{
					Namespace:        deploymentNamespace,
					Cluster:          member1,
					ClusterNamespace: options.DefaultKarmadaClusterNamespace,
				}
				args := []string{"deployment", deploymentName}
				// init args: place deployment name to CommandPromoteOption.name
				err := opts.Complete(args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				// use karmadactl to promote a deployment from member1
				err = karmadactl.RunPromote(karmadaConfig, opts, args)
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
			framework.RemoveClusterRole(kubeClient, clusterRoleName)
			framework.RemoveClusterPropagationPolicy(karmadaClient, clusterRoleName+"-propagation")

			framework.RemoveClusterRoleBinding(kubeClient, clusterRoleBindingName)
			framework.RemoveClusterPropagationPolicy(karmadaClient, clusterRoleBindingName+"-propagation")
		})

		ginkgo.It("Test promoting clusterrole and clusterrolebindings", func() {

			// Step1, create clusterrole and clusterrolebinding on member1
			ginkgo.By(fmt.Sprintf("Creating clusterrole and clusterrolebinding in member: %s", member1), func() {
				framework.CreateClusterRole(member1Client, clusterRole)
				framework.CreateClusterRoleBinding(member1Client, clusterRoleBinding)
			})

			// Step2, promote clusterrole and clusterrolebinding from member1
			ginkgo.By(fmt.Sprintf("Promoting clusterrole %s and clusterrolebindings %s from member to karmada", clusterRoleName, clusterRoleBindingName), func() {
				opts := karmadactl.CommandPromoteOption{
					Cluster:          member1,
					ClusterNamespace: options.DefaultKarmadaClusterNamespace,
				}

				args := []string{"clusterrole", clusterRoleName}
				// init args: place clusterrole name to CommandPromoteOption.name
				err := opts.Complete(args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				// use karmadactl to promote clusterrole from member1
				err = karmadactl.RunPromote(karmadaConfig, opts, args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				args = []string{"clusterrolebinding", clusterRoleBindingName}
				// init args: place clusterrolebinding name to CommandPromoteOption.name
				err = opts.Complete(args)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				// use karmadactl to promote clusterrolebinding from member1
				err = karmadactl.RunPromote(karmadaConfig, opts, args)
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
					ClusterNames: []string{deploymentName},
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
					GlobalCommandOptions: options.GlobalCommandOptions{
						DryRun: false,
					},
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
				gomega.Eventually(func() bool {
					_, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
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
					GlobalCommandOptions: options.GlobalCommandOptions{
						DryRun: false,
					},
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterName,
					ClusterContext:    clusterContext,
					ClusterKubeConfig: kubeConfigPath,
					Wait:              options.DefaultKarmadactlCommandDuration,
				}
				err := karmadactl.RunUnjoin(karmadaConfig, opts)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})
})
