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

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/karmada-io/karmada/pkg/karmadactl/join"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/unjoin"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[namespace auto-provision] namespace auto-provision testing", func() {
	var namespaceName string
	var namespace *corev1.Namespace
	var f cmdutil.Factory

	ginkgo.BeforeEach(func() {
		namespaceName = "karmada-e2e-ns-" + rand.String(RandomStrLength)
		namespace = helper.NewNamespace(namespaceName)

		defaultConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)
		defaultConfigFlags.Context = &karmadaContext
		f = cmdutil.NewFactory(defaultConfigFlags)
	})

	ginkgo.When("create a namespace in karmada-apiserver", func() {
		ginkgo.BeforeEach(func() {
			framework.CreateNamespace(kubeClient, namespace)
		})
		ginkgo.AfterEach(func() {
			framework.RemoveNamespace(kubeClient, namespaceName)
		})

		ginkgo.It("namespace should be propagated to member clusters", func() {
			framework.WaitNamespacePresentOnClusters(framework.ClusterNames(), namespaceName)
		})
	})

	ginkgo.When("delete a namespace from karmada apiserver", func() {
		ginkgo.BeforeEach(func() {
			framework.CreateNamespace(kubeClient, namespace)
			framework.WaitNamespacePresentOnClusters(framework.ClusterNames(), namespaceName)
		})

		ginkgo.It("namespace should be removed from member clusters", func() {
			framework.RemoveNamespace(kubeClient, namespaceName)
			framework.WaitNamespaceDisappearOnClusters(framework.ClusterNames(), namespaceName)
		})
	})

	framework.SerialWhen("joining new cluster", ginkgo.Labels{NeedCreateCluster}, func() {
		var clusterName string
		var homeDir string
		var kubeConfigPath string
		var controlPlane string
		var clusterContext string

		ginkgo.BeforeEach(func() {
			clusterName = "member-e2e-" + rand.String(RandomStrLength)
			homeDir = os.Getenv("HOME")
			kubeConfigPath = fmt.Sprintf("%s/.kube/%s.config", homeDir, clusterName)
			controlPlane = fmt.Sprintf("%s-control-plane", clusterName)
			clusterContext = fmt.Sprintf("kind-%s", clusterName)
		})

		ginkgo.BeforeEach(func() {
			framework.CreateNamespace(kubeClient, namespace)
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Creating cluster: %s", clusterName), func() {
				err := createCluster(clusterName, kubeConfigPath, controlPlane, clusterContext)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Joining cluster: %s", clusterName), func() {
				opts := join.CommandJoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterName,
					ClusterContext:    clusterContext,
					ClusterKubeConfig: kubeConfigPath,
				}
				err := opts.Run(f)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("Unjoinning cluster: %s", clusterName), func() {
				opts := unjoin.CommandUnjoinOption{
					DryRun:            false,
					ClusterNamespace:  "karmada-cluster",
					ClusterName:       clusterName,
					ClusterContext:    clusterContext,
					ClusterKubeConfig: kubeConfigPath,
					Wait:              5 * options.DefaultKarmadactlCommandDuration,
				}
				err := opts.Run(f)
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
			framework.RemoveNamespace(kubeClient, namespaceName)
		})

		ginkgo.It("namespace should be propagated to new created clusters", func() {
			ginkgo.By(fmt.Sprintf("waiting namespace(%s) present on cluster: %s", namespaceName, clusterName), func() {
				clusterJoined, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				clusterClient, err := util.NewClusterClientSet(clusterJoined.Name, controlPlaneClient, nil)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				err = wait.PollUntilContextTimeout(context.TODO(), pollInterval, pollTimeout, true, func(ctx context.Context) (done bool, err error) {
					_, err = clusterClient.KubeClient.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
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
