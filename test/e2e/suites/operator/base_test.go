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

package e2e

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	operatorutil "github.com/karmada-io/karmada/operator/pkg/util"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

const karmadactlTimeout = time.Second * 60

var karmadaContext = fmt.Sprintf("%s@%s", constants.UserName, constants.ClusterName)

var _ = ginkgo.Describe("Base E2E: deploy a karmada instance and do a propagation testing", func() {
	var karmadaName string
	var controlPlaneConfig client.Client
	var kubeClient kubernetes.Interface
	var karmadaClient karmada.Interface
	var karmadaConfigFilePath string

	var pushModeClusterName string
	var pushModeKubeConfigPath string
	var pushModeClusterClient kubernetes.Interface
	var targetClusters []string

	var deploymentNamespace string
	var deploymentName string
	var policyName string

	ginkgo.Context("Karmada Operator base testing", func() {
		ginkgo.BeforeEach(func() {
			karmadaName = KarmadaInstanceNamePrefix + rand.String(RandomStrLength)
			InitializeKarmadaInstance(operatorClient, testNamespace, karmadaName, func(karmada *operatorv1alpha1.Karmada) {
				karmada.Spec.Components.KarmadaAPIServer.ServiceType = corev1.ServiceTypeNodePort
			})
		})

		ginkgo.BeforeEach(func() {
			homeDir := os.Getenv("HOME")
			karmadaConfigFilePath = fmt.Sprintf("%s/.kube/karmada-%s.config", homeDir, karmadaName)
			secret, err := hostClient.CoreV1().Secrets(testNamespace).Get(context.Background(), operatorutil.AdminKarmadaConfigSecretName(karmadaName), metav1.GetOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			karmadaConfigBytes := secret.Data["karmada.config"]
			err = writeKubeConfigToFile(karmadaConfigBytes, karmadaConfigFilePath)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			karmadaRestConfig, err := framework.LoadRESTClientConfig(karmadaConfigFilePath, karmadaContext)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			controlPlaneConfig = gclient.NewForConfigOrDie(karmadaRestConfig)
			kubeClient = kubernetes.NewForConfigOrDie(karmadaRestConfig)
			karmadaClient = karmada.NewForConfigOrDie(karmadaRestConfig)

			pushModeClusterName = os.Getenv("PUSH_MODE_CLUSTER_NAME")
			pushModeKubeConfigPath = os.Getenv("PUSH_MODE_KUBECONFIG_PATH")
			pushModeClusterRestConfig, err := framework.LoadRESTClientConfig(pushModeKubeConfigPath, pushModeClusterName)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			pushModeClusterClient = kubernetes.NewForConfigOrDie(pushModeClusterRestConfig)

			targetClusters = []string{pushModeClusterName}
		})

		ginkgo.AfterEach(func() {
			// Clean up resources
			framework.RemoveDeployment(kubeClient, deploymentNamespace, deploymentName)
			framework.RemovePropagationPolicy(karmadaClient, deploymentNamespace, policyName)
			framework.RemoveNamespace(kubeClient, deploymentNamespace)

			// Unjoin the push mode cluster
			cmd := framework.NewKarmadactlCommand(karmadaConfigFilePath, karmadaContext, karmadactlPath, "", karmadactlTimeout,
				"unjoin", "--cluster-kubeconfig", pushModeKubeConfigPath, "--cluster-context", pushModeClusterName, "--cluster-namespace", "karmada-cluster",
				"--v", "4", pushModeClusterName)
			_, err := cmd.ExecOrDie()
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			// Delete the karmada instance
			err = operatorClient.OperatorV1alpha1().Karmadas(testNamespace).Delete(context.Background(), karmadaName, metav1.DeleteOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			os.Remove(karmadaConfigFilePath)
		})

		ginkgo.It("Deploy a karmada instance and do a propagation testing", func() {
			ginkgo.By("join a push mode cluster", func() {
				cmd := framework.NewKarmadactlCommand(karmadaConfigFilePath, karmadaContext, karmadactlPath, "", karmadactlTimeout, "join",
					"--cluster-kubeconfig", pushModeKubeConfigPath, "--cluster-context", pushModeClusterName, "--cluster-namespace", "karmada-cluster",
					"--v", "4", pushModeClusterName)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("Wait for the new cluster to be ready", func() {
				framework.WaitClusterFitWith(controlPlaneConfig, pushModeClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
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
				framework.CreatePropagationPolicy(karmadaClient, policy)
				framework.WaitDeploymentFitWith(pushModeClusterClient, deployment.Namespace, deployment.Name,
					func(*appsv1.Deployment) bool {
						return true
					})
			})
		})
	})
})

func writeKubeConfigToFile(config []byte, filePath string) error {
	// Write the config to the specified file path
	err := os.WriteFile(filePath, config, 0600)
	if err != nil {
		return fmt.Errorf("failed to write kubeconfig to file: %v", err)
	}
	return nil
}
