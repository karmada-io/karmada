package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/helper"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
)

var _ = ginkgo.Describe("[namespace autoprovision] namespace autoprovision testing", func() {
	var err error
	clusterName := "member-e2e-" + rand.String(3)
	homeDir := os.Getenv("HOME")
	kubeConfigDir := fmt.Sprintf("%s/.kube", homeDir)
	kubeconfigName := fmt.Sprintf("%s.config", clusterName)
	kubeConfigPath := fmt.Sprintf("%s/%s", kubeConfigDir, kubeconfigName)
	controlPlane := fmt.Sprintf("%s-control-plane", clusterName)
	clusterContext := fmt.Sprintf("kind-%s", clusterName)

	ginkgo.BeforeEach(func() {
		// create a new member cluster which will be used by following tests.
		err := createCluster(clusterName, kubeConfigPath, controlPlane, clusterContext)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.AfterEach(func() {
		_ = os.Remove(kubeConfigPath)
	})

	// propagate single resource to two explicit clusters.
	ginkgo.Context("namespace autoprovision testing", func() {
		ginkgo.It("propagate namespace", func() {
			namespaceName := rand.String(6)
			namespace := helper.NewNamespace(namespaceName)

			ginkgo.By(fmt.Sprintf("creating namespace: %s", namespaceName), func() {
				_, err := util.CreateNamespace(kubeClient, namespace)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.By("check if namespace appear in member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					err = wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
						if err != nil {
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By("join new member cluster", func() {
				karmadaConfig := karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
				opts := karmadactl.CommandJoinOption{
					GlobalCommandOptions: options.GlobalCommandOptions{
						ClusterNamespace: "karmada-cluster",
						DryRun:           false,
					},
					ClusterName:       clusterName,
					ClusterContext:    clusterContext,
					ClusterKubeConfig: kubeConfigPath,
				}
				err = karmadactl.RunJoin(os.Stdout, karmadaConfig, opts)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if namespace appear in new member clusters", func() {
				clusterJoined, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				clusterClient, err := util.NewClusterClientSet(clusterJoined, kubeClient)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				err = wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
					_, err = clusterClient.KubeClient.CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					return true, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("deleting namespace: %s", namespaceName), func() {
				err := util.DeleteNamespace(kubeClient, namespaceName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.By("check if namespace disappear from member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					err = wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
								return true, nil
							}
						}
						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})
	})
})
