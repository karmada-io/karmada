package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[namespace auto-provision] namespace auto-provision testing", func() {

	ginkgo.When("create a namespace in karmada-apiserver", func() {
		namespaceName := "karmada-e2e-ns-" + rand.String(3)
		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Creating namespace: %s", namespaceName), func() {
				namespace := helper.NewNamespace(namespaceName)
				_, err := util.CreateNamespace(kubeClient, namespace)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("Deleting namespace: %s", namespaceName), func() {
				err := util.DeleteNamespace(kubeClient, namespaceName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("namespace should be propagated to member clusters", func() {
			ginkgo.By("check if namespace appear in member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})
	})

	ginkgo.When("delete a namespace from karmada apiserver", func() {
		namespaceName := "karmada-e2e-ns-" + rand.String(3)

		ginkgo.It("namespace should be propagated to member clusters", func() {
			ginkgo.By(fmt.Sprintf("Creating namespace: %s", namespaceName), func() {
				namespace := helper.NewNamespace(namespaceName)
				_, err := util.CreateNamespace(kubeClient, namespace)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if namespace appear in member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})

		ginkgo.It("namespace should be removed from member clusters", func() {

			ginkgo.By(fmt.Sprintf("Deleting namespace: %s", namespaceName), func() {
				err := util.DeleteNamespace(kubeClient, namespaceName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("namespace(%s) shoud be disappeared", namespaceName), func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
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

	ginkgo.When("joining new cluster", func() {
		clusterName := "member-e2e-" + rand.String(3)
		homeDir := os.Getenv("HOME")
		kubeConfigPath := fmt.Sprintf("%s/.kube/%s.config", homeDir, clusterName)
		controlPlane := fmt.Sprintf("%s-control-plane", clusterName)
		clusterContext := fmt.Sprintf("kind-%s", clusterName)

		namespaceName := "karmada-e2e-ns-" + rand.String(3)
		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Creating namespace: %s", namespaceName), func() {
				namespace := helper.NewNamespace(namespaceName)
				_, err := util.CreateNamespace(kubeClient, namespace)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
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
						ClusterNamespace: "karmada-cluster",
						DryRun:           false,
					},
					ClusterName:       clusterName,
					ClusterContext:    clusterContext,
					ClusterKubeConfig: kubeConfigPath,
				}
				err := karmadactl.RunJoin(os.Stdout, karmadaConfig, opts)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("Unjoinning cluster: %s", clusterName), func() {
				karmadaConfig := karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
				opts := karmadactl.CommandUnjoinOption{
					GlobalCommandOptions: options.GlobalCommandOptions{
						ClusterNamespace: "karmada-cluster",
						DryRun:           false,
					},
					ClusterName:       clusterName,
					ClusterContext:    clusterContext,
					ClusterKubeConfig: kubeConfigPath,
				}
				err := karmadactl.RunUnjoin(os.Stdout, karmadaConfig, opts)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("Deleting clusters: %s", clusterName), func() {
				err := deleteCluster(clusterName, kubeConfigPath)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				_ = os.Remove(kubeConfigPath)
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("Deleting namespace: %s", namespaceName), func() {
				err := util.DeleteNamespace(kubeClient, namespaceName)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("namespace should be propagated to new created clusters", func() {
			ginkgo.By(fmt.Sprintf("waiting namespace(%s) present on cluster: %s", namespaceName, clusterName), func() {
				clusterJoined, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				clusterClient, err := util.NewClusterClientSet(clusterJoined, controlPlaneClient, nil)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				err = wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					_, err = clusterClient.KubeClient.CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
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
		})
	})
})
