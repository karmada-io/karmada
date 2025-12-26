/*
Copyright 2025 The Karmada Authors.

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

package init

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

const karmadactlTimeout = time.Second * 120

var _ = ginkgo.Describe("Base E2E: deploy a karmada instance by cmd init and do a propagation testing", func() {
	var controlPlaneConfig client.Client
	var kubeClient kubernetes.Interface
	var karmadaClient karmada.Interface
	var karmadaConfigFilePath string

	var pushModeClusterName string
	var pushModeKubeConfigPath string
	var pushModeClusterClient kubernetes.Interface

	var pullModeClusterName string
	var pullModeKubeConfigPath string
	var pullModeClusterClient kubernetes.Interface

	var targetClusters []string

	var deploymentNamespace string
	var deploymentName string
	var policyName string

	var tempPki string

	var etcdDataPath string

	ginkgo.Context("Karmadactl init base testing", func() {
		ginkgo.BeforeEach(func() {
			tempPki = filepath.Join(karmadaDataPath, "pki")
			etcdDataPath = filepath.Join(karmadaDataPath, "etcd-data")
		})

		ginkgo.BeforeEach(func() {
			pushModeClusterName = os.Getenv("PUSH_MODE_CLUSTER_NAME")
			pushModeKubeConfigPath = os.Getenv("PUSH_MODE_KUBECONFIG_PATH")
			pushModeClusterRestConfig, err := framework.LoadRESTClientConfig(pushModeKubeConfigPath, pushModeClusterName)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			pushModeClusterClient = kubernetes.NewForConfigOrDie(pushModeClusterRestConfig)

			pullModeClusterName = os.Getenv("PULL_MODE_CLUSTER_NAME")
			pullModeKubeConfigPath = os.Getenv("PULL_MODE_KUBECONFIG_PATH")
			pullModeClusterRestConfig, err := framework.LoadRESTClientConfig(pullModeKubeConfigPath, pullModeClusterName)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			pullModeClusterClient = kubernetes.NewForConfigOrDie(pullModeClusterRestConfig)

			targetClusters = []string{pushModeClusterName, pullModeClusterName}
		})

		ginkgo.It("Deploy a karmada instance and do a propagation testing", func() {
			ginkgo.By("Execute the command karmadactl init", func() {
				args := []string{"init",
					"--karmada-data", karmadaDataPath,
					"--karmada-pki", tempPki,
					"--crds", crdsPath,
					"--karmada-aggregated-apiserver-image", karmadaAggregatedAPIServerImage,
					"--karmada-controller-manager-image", karmadaControllerManagerImage,
					"--karmada-scheduler-image", karmadaSchedulerImage,
					"--karmada-webhook-image", karmadaWebhookImage,
					"--port", karmadaAPIServerNodePort,
					"--etcd-data", etcdDataPath,
					"--v", "4",
				}

				cmd := framework.NewKarmadactlCommand(
					kubeconfig,
					"",
					karmadactlPath,
					testNamespace,
					KarmadactlInitTimeOut,
					args...,
				)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				karmadaConfigFilePath = filepath.Join(karmadaDataPath, "karmada-apiserver.config")
				karmadaRestConfig, err := framework.LoadRESTClientConfig(karmadaConfigFilePath, "")
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				controlPlaneConfig = gclient.NewForConfigOrDie(karmadaRestConfig)
				kubeClient = kubernetes.NewForConfigOrDie(karmadaRestConfig)
				karmadaClient = karmada.NewForConfigOrDie(karmadaRestConfig)
			})

			ginkgo.By("join a push mode cluster", func() {
				cmd := framework.NewKarmadactlCommand(karmadaConfigFilePath, "", karmadactlPath, "", karmadactlTimeout, "join",
					"--cluster-kubeconfig", pushModeKubeConfigPath, "--cluster-context", pushModeClusterName, "--cluster-namespace", "karmada-cluster",
					"--v", "4", pushModeClusterName)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				ginkgo.DeferCleanup(func() {
					// Unjoin the push mode cluster
					cmd := framework.NewKarmadactlCommand(karmadaConfigFilePath, "", karmadactlPath, "", karmadactlTimeout,
						"unjoin", "--cluster-kubeconfig", pushModeKubeConfigPath, "--cluster-context", pushModeClusterName, "--cluster-namespace", "karmada-cluster",
						"--v", "4", pushModeClusterName)
					_, err := cmd.ExecOrDie()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})
			})

			ginkgo.By("register a pull mode cluster", func() {
				cmd := framework.NewKarmadactlCommand(
					karmadaConfigFilePath, "", karmadactlPath, "", karmadactlTimeout,
					"token", "create", "--print-register-command="+"true",
				)
				output, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				// Extract the endpoint for Karmada APIServer.
				endpointRegex := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d{1,5})`)
				karmadaAPIEndpoint := endpointRegex.FindString(output)

				// Extract token.
				tokenRegex := regexp.MustCompile(`--token\s+(\S+)`)
				tokenMatches := tokenRegex.FindStringSubmatch(output)
				gomega.Expect(len(tokenMatches)).Should(gomega.BeNumerically(">", 1))
				token := tokenMatches[1]

				// Extract discovery token CA cert hash.
				hashRegex := regexp.MustCompile(`--discovery-token-ca-cert-hash\s+(\S+)`)
				hashMatches := hashRegex.FindStringSubmatch(output)
				gomega.Expect(len(hashMatches)).Should(gomega.BeNumerically(">", 1))
				discoveryTokenCACertHash := hashMatches[1]

				cmd = framework.NewKarmadactlCommand(
					"", "", karmadactlPath, "", karmadactlTimeout, "register", karmadaAPIEndpoint, "--token", token,
					"--kubeconfig="+pullModeKubeConfigPath,
					"--cluster-name", pullModeClusterName, "--context", pullModeClusterName, "--karmada-agent-image", "docker.io/karmada/karmada-agent:latest",
					"--discovery-token-ca-cert-hash", discoveryTokenCACertHash, "--namespace", testNamespace,
				)
				_, err = cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				ginkgo.DeferCleanup(func() {
					ginkgo.By(fmt.Sprintf("Unregistering cluster: %s", pullModeClusterName), func() {
						cmd := framework.NewKarmadactlCommand(
							kubeconfig, "", karmadactlPath, "", karmadactlTimeout,
							"unregister", pullModeClusterName, "--cluster-kubeconfig", pullModeKubeConfigPath,
							"--cluster-context", pullModeClusterName, "--namespace", testNamespace,
						)
						_, err := cmd.ExecOrDie()
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					})
				})
			})

			ginkgo.By("Wait for the new cluster to be ready", func() {
				framework.WaitClusterFitWith(controlPlaneConfig, pushModeClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
					return meta.IsStatusConditionPresentAndEqual(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
				})

				framework.WaitClusterFitWith(controlPlaneConfig, pullModeClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
					return meta.IsStatusConditionPresentAndEqual(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
				})
			})

			ginkgo.By("Do a simple propagation testing", func() {
				deploymentNamespace = "deployment-" + rand.String(RandomStrLength)
				framework.CreateNamespace(kubeClient, testhelper.NewNamespace(deploymentNamespace))

				policyName = "deployment" + rand.String(RandomStrLength)
				deploymentName = policyName

				deployment := testhelper.NewDeployment(deploymentNamespace, deploymentName)
				policy := testhelper.NewPropagationPolicy(deploymentNamespace, policyName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: targetClusters,
					},
				})

				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, deploymentNamespace, deploymentName)
				})
				framework.CreatePropagationPolicy(karmadaClient, policy)
				ginkgo.DeferCleanup(func() {
					framework.RemovePropagationPolicy(karmadaClient, deploymentNamespace, policyName)
				})
				framework.WaitDeploymentFitWith(pushModeClusterClient, deployment.Namespace, deployment.Name,
					func(*appsv1.Deployment) bool {
						return true
					})
				framework.WaitDeploymentFitWith(pullModeClusterClient, deployment.Namespace, deployment.Name,
					func(*appsv1.Deployment) bool {
						return true
					})
			})
		})
	})
})
