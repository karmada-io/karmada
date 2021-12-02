package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/clientcmd"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("unjoin cluster testing", func() {

	ginkgo.When("unjoining not ready cluster", func() {
		clusterName := "member-e2e-" + rand.String(3)
		homeDir := os.Getenv("HOME")
		kubeConfigPath := fmt.Sprintf("%s/.kube/%s.config", homeDir, clusterName)
		controlPlane := fmt.Sprintf("%s-control-plane", clusterName)
		clusterContext := fmt.Sprintf("kind-%s", clusterName)
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := testNamespace
		deploymentName := policyName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		policy := helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Creating cluster: %s", clusterName), func() {
				err := createCluster(clusterName, kubeConfigPath, controlPlane, clusterContext)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Joinning cluster: %s", clusterName), func() {
				karmadaConfig := karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
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
		})
		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Creating Deployment workload in cluster: %s", clusterName), func() {
				framework.CreatePropagationPolicy(karmadaClient, policy)
				framework.CreateDeployment(kubeClient, deployment)
			})
		})
		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("diasble cluster %s ", clusterName), func() {
				err := disableCluster(controlPlaneClient, clusterName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
		ginkgo.It("Test unjoining not ready cluster", func() {
			karmadaConfig := karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
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
			err = deleteCluster(clusterName, kubeConfigPath)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			deployment, err := kubeClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(deployment).ShouldNot(gomega.BeNil())
		})
		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("Deleting cluster: %s and resouce", clusterName), func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
		})
	})
})
