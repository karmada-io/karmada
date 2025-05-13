/*
Copyright 2021 The Karmada Authors.

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
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/karmadactl/join"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/unjoin"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

const waitTimeout = 3 * time.Second

var _ = framework.SerialDescribe("FederatedResourceQuota auto-provision testing", func() {
	var frqNamespace, frqName string
	var federatedResourceQuota *policyv1alpha1.FederatedResourceQuota
	var f cmdutil.Factory

	ginkgo.BeforeEach(func() {
		frqNamespace = testNamespace
		frqName = federatedResourceQuotaPrefix + rand.String(RandomStrLength)

		defaultConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)
		defaultConfigFlags.Context = &karmadaContext
		f = cmdutil.NewFactory(defaultConfigFlags)
	})

	ginkgo.It("CURD a federatedResourceQuota", func() {
		var clusters = framework.ClusterNames()
		federatedResourceQuota = helper.NewFederatedResourceQuota(frqNamespace, frqName, framework.ClusterNames())
		ginkgo.By("[Create] federatedResourceQuota should be propagated to member clusters", func() {
			framework.CreateFederatedResourceQuota(karmadaClient, federatedResourceQuota)
			framework.WaitResourceQuotaPresentOnClusters(clusters, frqNamespace, frqName)
		})

		ginkgo.By("[Update] federatedResourceQuota should be propagated to member clusters according to the new staticAssignments", func() {
			clusters = []string{framework.ClusterNames()[0]}
			federatedResourceQuota = helper.NewFederatedResourceQuota(frqNamespace, frqName, clusters)
			patch := []map[string]interface{}{
				{
					"op":    "replace",
					"path":  "/spec/staticAssignments",
					"value": federatedResourceQuota.Spec.StaticAssignments,
				},
			}
			framework.UpdateFederatedResourceQuotaWithPatch(karmadaClient, frqNamespace, frqName, patch, types.JSONPatchType)
			framework.WaitResourceQuotaPresentOnClusters(clusters, frqNamespace, frqName)
			framework.WaitResourceQuotaDisappearOnClusters(framework.ClusterNames()[1:], frqNamespace, frqName)
		})

		ginkgo.By("[Delete] federatedResourceQuota should be removed from member clusters", func() {
			framework.RemoveFederatedResourceQuota(karmadaClient, frqNamespace, frqName)
			framework.WaitResourceQuotaDisappearOnClusters(clusters, frqNamespace, frqName)
		})
	})

	framework.SerialContext("join new cluster", ginkgo.Labels{NeedCreateCluster}, func() {
		var clusterNameInStaticAssignment string
		var clusterNameNotInStaticAssignment string
		var homeDir string
		var kubeConfigPathInStaticAssignment string
		var kubeConfigPathNotInStaticAssignment string
		var controlPlaneInStaticAssignment string
		var controlPlaneNotInStaticAssignment string
		var clusterContextInStaticAssignment string
		var clusterContextNotInStaticAssignment string

		ginkgo.BeforeEach(func() {
			clusterNameInStaticAssignment = "member-e2e-" + rand.String(RandomStrLength)
			clusterNameNotInStaticAssignment = "member-e2e-" + rand.String(RandomStrLength)
			homeDir = os.Getenv("HOME")
			kubeConfigPathInStaticAssignment = fmt.Sprintf("%s/.kube/%s.config", homeDir, clusterNameInStaticAssignment)
			kubeConfigPathNotInStaticAssignment = fmt.Sprintf("%s/.kube/%s.config", homeDir, clusterNameNotInStaticAssignment)
			controlPlaneInStaticAssignment = fmt.Sprintf("%s-control-plane", clusterNameInStaticAssignment)
			controlPlaneNotInStaticAssignment = fmt.Sprintf("%s-control-plane", clusterNameNotInStaticAssignment)
			clusterContextInStaticAssignment = fmt.Sprintf("kind-%s", clusterNameInStaticAssignment)
			clusterContextNotInStaticAssignment = fmt.Sprintf("kind-%s", clusterNameNotInStaticAssignment)
		})

		ginkgo.BeforeEach(func() {
			clusterNames := append(framework.ClusterNames(), clusterNameInStaticAssignment)
			federatedResourceQuota = helper.NewFederatedResourceQuota(frqNamespace, frqName, clusterNames)
			framework.CreateFederatedResourceQuota(karmadaClient, federatedResourceQuota)
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Creating cluster: %s", clusterNameInStaticAssignment), func() {
				err := createCluster(clusterNameInStaticAssignment, kubeConfigPathInStaticAssignment, controlPlaneInStaticAssignment, clusterContextInStaticAssignment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.By(fmt.Sprintf("Creating cluster: %s", clusterNameNotInStaticAssignment), func() {
				err := createCluster(clusterNameNotInStaticAssignment, kubeConfigPathNotInStaticAssignment, controlPlaneNotInStaticAssignment, clusterContextNotInStaticAssignment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("Unjoinning cluster: %s", clusterNameInStaticAssignment), func() {
				opts := unjoin.CommandUnjoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterNameInStaticAssignment,
					ClusterContext:    clusterContextInStaticAssignment,
					ClusterKubeConfig: kubeConfigPathInStaticAssignment,
					Wait:              5 * options.DefaultKarmadactlCommandDuration,
				}
				err := opts.Run(f)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.By(fmt.Sprintf("Unjoinning cluster: %s", clusterNameNotInStaticAssignment), func() {
				opts := unjoin.CommandUnjoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterNameNotInStaticAssignment,
					ClusterContext:    clusterContextNotInStaticAssignment,
					ClusterKubeConfig: kubeConfigPathNotInStaticAssignment,
					Wait:              5 * options.DefaultKarmadactlCommandDuration,
				}
				err := opts.Run(f)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("Deleting clusters: %s", clusterNameInStaticAssignment), func() {
				err := deleteCluster(clusterNameInStaticAssignment, kubeConfigPathInStaticAssignment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				_ = os.Remove(kubeConfigPathInStaticAssignment)
			})
			ginkgo.By(fmt.Sprintf("Deleting clusters: %s", clusterNameNotInStaticAssignment), func() {
				err := deleteCluster(clusterNameNotInStaticAssignment, kubeConfigPathNotInStaticAssignment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				_ = os.Remove(kubeConfigPathNotInStaticAssignment)
			})
		})

		ginkgo.AfterEach(func() {
			framework.RemoveFederatedResourceQuota(karmadaClient, frqNamespace, frqName)
		})

		ginkgo.It("federatedResourceQuota should only be propagated to newly joined cluster if the new cluster is declared in the StaticAssignment", func() {
			ginkgo.By(fmt.Sprintf("Joining cluster: %s", clusterNameInStaticAssignment), func() {
				opts := join.CommandJoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterNameInStaticAssignment,
					ClusterContext:    clusterContextInStaticAssignment,
					ClusterKubeConfig: kubeConfigPathInStaticAssignment,
				}
				err := opts.Run(f)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("waiting federatedResourceQuota(%s/%s) present on cluster: %s", frqNamespace, frqName, clusterNameInStaticAssignment), func() {
				clusterClient, err := util.NewClusterClientSet(clusterNameInStaticAssignment, controlPlaneClient, nil)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					_, err := clusterClient.KubeClient.CoreV1().ResourceQuotas(frqNamespace).Get(context.TODO(), frqName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())
					return true, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By(fmt.Sprintf("Joining cluster: %s", clusterNameNotInStaticAssignment), func() {
				opts := join.CommandJoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterNameNotInStaticAssignment,
					ClusterContext:    clusterContextNotInStaticAssignment,
					ClusterKubeConfig: kubeConfigPathNotInStaticAssignment,
				}
				err := opts.Run(f)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("check if federatedResourceQuota(%s/%s) present on cluster: %s", frqNamespace, frqName, clusterNameNotInStaticAssignment), func() {
				clusterClient, err := util.NewClusterClientSet(clusterNameNotInStaticAssignment, controlPlaneClient, nil)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				time.Sleep(waitTimeout)
				_, err = clusterClient.KubeClient.CoreV1().ResourceQuotas(frqNamespace).Get(context.TODO(), frqName, metav1.GetOptions{})
				gomega.Expect(apierrors.IsNotFound(err)).Should(gomega.Equal(true))
			})
		})
	})
})

var _ = framework.SerialDescribe("[FederatedResourceQuota] status collection testing", func() {
	var frqNamespace, frqName string
	var federatedResourceQuota *policyv1alpha1.FederatedResourceQuota

	ginkgo.BeforeEach(func() {
		frqNamespace = testNamespace
		frqName = federatedResourceQuotaPrefix + rand.String(RandomStrLength)
		federatedResourceQuota = helper.NewFederatedResourceQuota(frqNamespace, frqName, framework.ClusterNames())
	})

	ginkgo.Context("collect federatedResourceQuota status", func() {
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
