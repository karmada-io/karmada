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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

// reschedule testing is used to test the rescheduling situation when some initially scheduled clusters are unjoined
var _ = ginkgo.Describe("reschedule testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := testNamespace
		deploymentName := policyName
		deployment := testhelper.NewDeployment(deploymentNamespace, deploymentName)
		maxGroups := 1
		minGroups := 1
		numOfUnjoinedClusters := 1

		// set MaxGroups=MinGroups=1, label is sync-mode=Push.
		policy := testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: pushModeClusterLabels,
				},
			},
			SpreadConstraints: []policyv1alpha1.SpreadConstraint{
				{
					SpreadByField: policyv1alpha1.SpreadByFieldCluster,
					MaxGroups:     maxGroups,
					MinGroups:     minGroups,
				},
			},
		})

		ginkgo.It("deployment reschedule testing", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)

			fmt.Printf("waiting for deployment %q ready\n", deploymentName)
			// wait for the deployment ready
			err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
				deploy, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				fmt.Printf("deployment %q readyReplicas: %d, expected replicas: %d\n", deploymentName, deploy.Status.ReadyReplicas, int32(maxGroups)**deploy.Spec.Replicas)
				return deploy.Status.ReadyReplicas == int32(maxGroups)**deploy.Spec.Replicas, nil
			})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			var unjoinedClusters []string
			targetClusterNames := framework.ExtractTargetClustersFrom(controlPlaneClient, deployment)

			ginkgo.By("unjoin target cluster", func() {
				count := numOfUnjoinedClusters
				for _, targetClusterName := range targetClusterNames {
					if count == 0 {
						break
					}
					count--
					klog.Infof("Unjoining cluster %q.", targetClusterName)
					karmadaConfig := karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
					opts := karmadactl.CommandUnjoinOption{
						GlobalCommandOptions: options.GlobalCommandOptions{
							KubeConfig:     fmt.Sprintf("%s/.kube/karmada.config", os.Getenv("HOME")),
							KarmadaContext: "karmada-apiserver",
						},
						ClusterNamespace: "karmada-cluster",
						ClusterName:      targetClusterName,
					}
					err := karmadactl.RunUnjoin(os.Stdout, karmadaConfig, opts)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					unjoinedClusters = append(unjoinedClusters, targetClusterName)
				}
			})

			ginkgo.By("check whether the deployment is rescheduled to other available clusters", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					totalNum := 0
					targetClusterNames = framework.ExtractTargetClustersFrom(controlPlaneClient, deployment)
					for _, targetClusterName := range targetClusterNames {
						// the target cluster should be overwritten to another available cluster
						g.Expect(isDisabled(targetClusterName, unjoinedClusters)).Should(gomega.BeFalse())

						framework.WaitDeploymentPresentOnClusterFitWith(targetClusterName, deployment.Namespace, deployment.Name,
							func(deployment *appsv1.Deployment) bool {
								return true
							})
						totalNum++
					}
					g.Expect(totalNum == maxGroups).Should(gomega.BeTrue())
				}, pollTimeout, pollInterval).Should(gomega.Succeed())
			})

			ginkgo.By("check if the scheduled condition is true", func() {
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
					rb, err := getResourceBinding(deployment)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					return meta.IsStatusConditionTrue(rb.Status.Conditions, workv1alpha2.Scheduled), nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("rejoin the unjoined clusters", func() {
				for _, unjoinedCluster := range unjoinedClusters {
					fmt.Printf("cluster %q is waiting for rejoining\n", unjoinedCluster)
					karmadaConfig := karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
					opts := karmadactl.CommandJoinOption{
						GlobalCommandOptions: options.GlobalCommandOptions{
							KubeConfig:     fmt.Sprintf("%s/.kube/karmada.config", os.Getenv("HOME")),
							KarmadaContext: "karmada-apiserver",
						},
						ClusterNamespace:  "karmada-cluster",
						ClusterName:       unjoinedCluster,
						ClusterContext:    unjoinedCluster,
						ClusterKubeConfig: fmt.Sprintf("%s/.kube/members.config", os.Getenv("HOME")),
					}
					err := karmadactl.RunJoin(os.Stdout, karmadaConfig, opts)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					fmt.Printf("waiting for cluster %q ready\n", unjoinedCluster)
					framework.WaitClusterFitWith(controlPlaneClient, unjoinedCluster, func(cluster *clusterv1alpha1.Cluster) bool {
						return util.IsClusterReady(&cluster.Status)
					})

					// set cluster label
					err = framework.SetClusterLabel(controlPlaneClient, unjoinedCluster)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})
})
