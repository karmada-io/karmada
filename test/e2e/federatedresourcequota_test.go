package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[FederatedResourceQuota] auto-provision testing", func() {
	ginkgo.Context("create a federatedResourceQuota", func() {
		frqNamespace := testNamespace
		frqName := federatedResourceQuotaPrefix + rand.String(RandomStrLength)
		federatedResourceQuota := helper.NewFederatedResourceQuota(frqNamespace, frqName)

		ginkgo.AfterEach(func() {
			framework.RemoveFederatedResourceQuota(karmadaClient, frqNamespace, frqName)
		})

		ginkgo.It("federatedResourceQuota should be propagated to member clusters", func() {
			framework.CreateFederatedResourceQuota(karmadaClient, federatedResourceQuota)
			framework.WaitResourceQuotaPresentOnClusters(framework.ClusterNames(), frqNamespace, frqName)
		})
	})

	ginkgo.Context("delete a federatedResourceQuota", func() {
		frqNamespace := testNamespace
		frqName := federatedResourceQuotaPrefix + rand.String(RandomStrLength)
		federatedResourceQuota := helper.NewFederatedResourceQuota(frqNamespace, frqName)

		ginkgo.BeforeEach(func() {
			framework.CreateFederatedResourceQuota(karmadaClient, federatedResourceQuota)
			framework.WaitResourceQuotaPresentOnClusters(framework.ClusterNames(), frqNamespace, frqName)
		})

		ginkgo.It("federatedResourceQuota should be removed from member clusters", func() {
			framework.RemoveFederatedResourceQuota(karmadaClient, frqNamespace, frqName)
			framework.WaitResourceQuotaDisappearOnClusters(framework.ClusterNames(), frqNamespace, frqName)
		})
	})

	ginkgo.Context("join new cluster", func() {
		frqNamespace := testNamespace
		frqName := federatedResourceQuotaPrefix + rand.String(RandomStrLength)
		federatedResourceQuota := helper.NewFederatedResourceQuota(frqNamespace, frqName)

		clusterName := "member-e2e-" + rand.String(3)
		homeDir := os.Getenv("HOME")
		kubeConfigPath := fmt.Sprintf("%s/.kube/%s.config", homeDir, clusterName)
		controlPlane := fmt.Sprintf("%s-control-plane", clusterName)
		clusterContext := fmt.Sprintf("kind-%s", clusterName)

		ginkgo.BeforeEach(func() {
			framework.CreateFederatedResourceQuota(karmadaClient, federatedResourceQuota)
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Creating cluster: %s", clusterName), func() {
				err := createCluster(clusterName, kubeConfigPath, controlPlane, clusterContext)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("Unjoinning cluster: %s", clusterName), func() {
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
			framework.RemoveFederatedResourceQuota(karmadaClient, frqNamespace, frqName)
		})

		ginkgo.It("federatedResourceQuota should be propagated to new joined clusters", func() {
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

			ginkgo.By(fmt.Sprintf("waiting federatedResourceQuota(%s/%s) present on cluster: %s", frqNamespace, frqName, clusterName), func() {
				clusterClient, err := util.NewClusterClientSet(clusterName, controlPlaneClient, nil)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					_, err := clusterClient.KubeClient.CoreV1().ResourceQuotas(frqNamespace).Get(context.TODO(), frqName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())
					return true, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})
})

var _ = ginkgo.Describe("[FederatedResourceQuota] status collection testing", func() {
	ginkgo.Context("collect federatedResourceQuota status", func() {
		frqNamespace := testNamespace
		frqName := federatedResourceQuotaPrefix + rand.String(RandomStrLength)
		federatedResourceQuota := helper.NewFederatedResourceQuota(frqNamespace, frqName)

		ginkgo.AfterEach(func() {
			framework.RemoveFederatedResourceQuota(karmadaClient, frqNamespace, frqName)
		})

		ginkgo.It("federatedResourceQuota status should be collect correctly", func() {
			framework.CreateFederatedResourceQuota(karmadaClient, federatedResourceQuota)
			framework.WaitFederatedResourceQuotaCollectStatus(karmadaClient, frqNamespace, frqName)

			patch := []map[string]interface{}{
				{
					"op":    "replace",
					"path":  "/spec/staticAssignments/0/hard/cpu",
					"value": "2",
				},
				{
					"op":    "replace",
					"path":  "/spec/staticAssignments/1/hard/memory",
					"value": "4Gi",
				},
			}
			framework.UpdateFederatedResourceQuotaWithPatch(karmadaClient, frqNamespace, frqName, patch, types.JSONPatchType)
			framework.WaitFederatedResourceQuotaCollectStatus(karmadaClient, frqNamespace, frqName)
		})
	})
})
