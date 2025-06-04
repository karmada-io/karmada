/*
Copyright 2020 The Karmada Authors.

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
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	mapper "k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kind/pkg/cluster"
	kindexec "sigs.k8s.io/kind/pkg/exec"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

const (
	// RandomStrLength represents the random string length to combine names.
	RandomStrLength = 5
)

const (
	deploymentNamePrefix          = "deploy-"
	serviceNamePrefix             = "service-"
	podNamePrefix                 = "pod-"
	crdNamePrefix                 = "cr-"
	jobNamePrefix                 = "job-"
	workloadNamePrefix            = "workload-"
	federatedResourceQuotaPrefix  = "frq-"
	configMapNamePrefix           = "configmap-"
	secretNamePrefix              = "secret-"
	pvcNamePrefix                 = "pvc-"
	saNamePrefix                  = "sa-"
	ingressNamePrefix             = "ingress-"
	daemonSetNamePrefix           = "daemonset-"
	statefulSetNamePrefix         = "statefulset-"
	roleNamePrefix                = "role-"
	clusterRoleNamePrefix         = "clusterrole-"
	roleBindingNamePrefix         = "rolebinding-"
	clusterRoleBindingNamePrefix  = "clusterrolebinding-"
	podDisruptionBudgetNamePrefix = "poddisruptionbudget-"
	federatedHPANamePrefix        = "fhpa-"
	cronFedratedHPANamePrefix     = "cronfhpa-"
	resourceRegistryPrefix        = "rr-"
	mcsNamePrefix                 = "mcs-"
	ppNamePrefix                  = "pp-"
	cppNamePrefix                 = "cpp-"
	workloadRebalancerPrefix      = "rebalancer-"
	remedyNamePrefix              = "remedy-"
	clusterTaintPolicyNamePrefix  = "ctp-"

	updateDeploymentReplicas  = 2
	updateStatefulSetReplicas = 2
	updateServicePort         = 81
	updatePodImage            = "nginx:latest"
	updateCRnamespace         = "e2e-test"
	updateBackoffLimit        = 3
	updateParallelism         = 3
)

var (
	// pollInterval defines the interval time for a poll operation.
	pollInterval time.Duration
	// pollTimeout defines the time after which the poll operation times out.
	pollTimeout time.Duration
)

var (
	hostContext        string
	karmadaContext     string
	kubeconfig         string
	karmadactlPath     string
	restConfig         *rest.Config
	karmadaHost        string
	hostKubeClient     kubernetes.Interface
	kubeClient         kubernetes.Interface
	karmadaClient      karmada.Interface
	dynamicClient      dynamic.Interface
	discoveryClient    *discovery.DiscoveryClient
	restMapper         meta.RESTMapper
	controlPlaneClient client.Client
	// testNamespace is the main namespace for testing.
	// It is the default namespace where most test resources are created and validated.
	testNamespace         string
	clusterProvider       *cluster.Provider
	clusterLabels         = map[string]string{"location": "CHN"}
	pushModeClusterLabels = map[string]string{"sync-mode": "Push"}
)

func init() {
	// usage ginkgo -- --poll-interval=5s --poll-timeout=5m
	// eg. ginkgo -v --race --trace --fail-fast -p --randomize-all ./test/e2e/suites/base -- --poll-interval=5s --poll-timeout=5m
	flag.DurationVar(&pollInterval, "poll-interval", 5*time.Second, "poll-interval defines the interval time for a poll operation")
	flag.DurationVar(&pollTimeout, "poll-timeout", 300*time.Second, "poll-timeout defines the time which the poll operation times out")
	flag.StringVar(&hostContext, "host-context", "karmada-host", "Name of the host cluster context in control plane kubeconfig file.")
	flag.StringVar(&karmadaContext, "karmada-context", "karmada-apiserver", "Name of the karmada cluster context in control plane kubeconfig file.")
}

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "E2E Suite")
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	return nil
}, func([]byte) {
	kubeconfig = os.Getenv("KUBECONFIG")
	gomega.Expect(kubeconfig).ShouldNot(gomega.BeEmpty())

	goPathCmd := exec.Command("go", "env", "GOPATH")
	goPath, err := goPathCmd.CombinedOutput()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	formatGoPath := strings.Trim(string(goPath), "\n")
	karmadactlPath = formatGoPath + "/bin/karmadactl"
	gomega.Expect(karmadactlPath).ShouldNot(gomega.BeEmpty())

	clusterProvider = cluster.NewProvider()

	restConfig, err = framework.LoadRESTClientConfig(kubeconfig, hostContext)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	hostKubeClient, err = kubernetes.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	restConfig, err = framework.LoadRESTClientConfig(kubeconfig, karmadaContext)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	karmadaHost = restConfig.Host

	kubeClient, err = kubernetes.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	karmadaClient, err = karmada.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	dynamicClient, err = dynamic.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	discoveryClient, err = discovery.NewDiscoveryClientForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	groupResources, err := mapper.GetAPIGroupResources(discoveryClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	restMapper = mapper.NewDiscoveryRESTMapper(groupResources)

	controlPlaneClient = gclient.NewForConfigOrDie(restConfig)

	framework.InitClusterInformation(karmadaClient, controlPlaneClient)

	testNamespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
	err = setupTestNamespace(testNamespace, kubeClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	framework.WaitNamespacePresentOnClusters(framework.ClusterNames(), testNamespace)
})

var _ = ginkgo.JustAfterEach(func() {
	// check if the current test case failed
	if ginkgo.CurrentSpecReport().Failed() {
		printAllBindingAndRelatedObjects()
	}
})

var _ = ginkgo.SynchronizedAfterSuite(func() {
	// cleanup all namespaces we created both in control plane and member clusters.
	// It will not return error even if there is no such namespace in there that may happen in case setup failed.
	err := cleanupTestNamespace(testNamespace, kubeClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}, func() {
	// cleanup clusterLabels set by the E2E test
	for _, cluster := range framework.Clusters() {
		err := deleteClusterLabel(controlPlaneClient, cluster.Name)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	}
})

// setupTestNamespace will create a namespace in control plane and all member clusters, most of cases will run against it.
// The reason why we need a separated namespace is it will make it easier to cleanup resources deployed by the testing.
func setupTestNamespace(namespace string, kubeClient kubernetes.Interface) error {
	namespaceObj := helper.NewNamespace(namespace)
	_, err := util.CreateNamespace(kubeClient, namespaceObj)
	if err != nil {
		return err
	}

	return nil
}

// cleanupTestNamespace will remove the namespace we setup before for the whole testing.
func cleanupTestNamespace(namespace string, kubeClient kubernetes.Interface) error {
	err := util.DeleteNamespace(kubeClient, namespace)
	if err != nil {
		return err
	}

	return nil
}

func createCluster(clusterName, kubeConfigPath, controlPlane, clusterContext string) error {
	err := clusterProvider.Create(clusterName, cluster.CreateWithKubeconfigPath(kubeConfigPath))
	if err != nil {
		return err
	}

	cmd := kindexec.Command(
		"docker", "inspect",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		controlPlane,
	)
	lines, err := kindexec.OutputLines(cmd)
	if err != nil {
		return err
	}

	pathOptions := clientcmd.NewDefaultPathOptions()
	pathOptions.LoadingRules.ExplicitPath = kubeConfigPath
	pathOptions.EnvVar = ""
	config, err := pathOptions.GetStartingConfig()
	if err != nil {
		return err
	}

	serverIP := fmt.Sprintf("https://%s:6443", lines[0])
	config.Clusters[clusterContext].Server = serverIP
	err = clientcmd.ModifyConfig(pathOptions, *config, true)
	if err != nil {
		return err
	}
	return nil
}

func deleteCluster(clusterName, kubeConfigPath string) error {
	return clusterProvider.Delete(clusterName, kubeConfigPath)
}

// deleteClusterLabel delete cluster label of E2E
func deleteClusterLabel(c client.Client, clusterName string) error {
	err := wait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 10*time.Second, true, func(ctx context.Context) (done bool, err error) {
		clusterObj := &clusterv1alpha1.Cluster{}
		if err := c.Get(ctx, client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
			return false, err
		}
		delete(clusterObj.Labels, "location")
		if err := c.Update(ctx, clusterObj); err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	return err
}

func podLogs(ctx context.Context, k8s kubernetes.Interface, namespace, name string) (string, error) {
	logRequest := k8s.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{})
	logs, err := logRequest.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer logs.Close()
	data, err := io.ReadAll(logs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
