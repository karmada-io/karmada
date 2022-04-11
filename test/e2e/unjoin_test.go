package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("test unjoin testing", func() {
	ginkgo.Context(" unjoining not ready cluster", func() {
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

		ginkgo.It("Test unjoining not ready cluster", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)

			ginkgo.By(fmt.Sprintf("create cluster: %s", clusterName), func() {
				err := createCluster(clusterName, kubeConfigPath, controlPlane, clusterContext)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

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
				err := karmadactl.RunJoin(os.Stdout, karmadaConfig, opts)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.By("wait for deployment have been propagated to the member cluster.", func() {
				klog.Infof("Waiting for deployment(%s/%s) synced on cluster(%s)", deploymentNamespace, deploymentName, clusterName)
				gomega.Eventually(func() bool {
					_, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					return err == nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By(fmt.Sprintf("disable cluster: %s", clusterName), func() {
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
				err := karmadactl.RunUnjoin(os.Stdout, karmadaConfig, opts)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("Deleting clusters: %s", clusterName), func() {
				err := deleteCluster(clusterName, kubeConfigPath)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				_ = os.Remove(kubeConfigPath)
			})
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})

	})
})
