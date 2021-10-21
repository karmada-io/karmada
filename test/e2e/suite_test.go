package e2e

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/exec"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

const (
	// TestSuiteSetupTimeOut defines the time after which the suite setup times out.
	TestSuiteSetupTimeOut = 300 * time.Second
	// TestSuiteTeardownTimeOut defines the time after which the suite tear down times out.
	TestSuiteTeardownTimeOut = 300 * time.Second

	// pollInterval defines the interval time for a poll operation.
	pollInterval = 5 * time.Second
	// pollTimeout defines the time after which the poll operation times out.
	pollTimeout = 300 * time.Second

	// RandomStrLength represents the random string length to combine names.
	RandomStrLength = 3
)

const (
	deploymentNamePrefix = "deploy-"
	serviceNamePrefix    = "service-"
	podNamePrefix        = "pod-"
	crdNamePrefix        = "cr-"
	jobNamePrefix        = "job-"

	updateDeploymentReplicas = 6
	updateServicePort        = 81
	updatePodImage           = "nginx:latest"
	updateCRnamespace        = "e2e-test"
	updateBackoffLimit       = 3
)

var (
	kubeconfig            string
	restConfig            *rest.Config
	kubeClient            kubernetes.Interface
	karmadaClient         karmada.Interface
	dynamicClient         dynamic.Interface
	controlPlaneClient    client.Client
	testNamespace         = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
	clusterProvider       *cluster.Provider
	clusterLabels         = map[string]string{"location": "CHN"}
	pushModeClusterLabels = map[string]string{"sync-mode": "Push"}
)

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "E2E Suite")
}

var _ = ginkgo.BeforeSuite(func() {
	kubeconfig = os.Getenv("KUBECONFIG")
	gomega.Expect(kubeconfig).ShouldNot(gomega.BeEmpty())

	clusterProvider = cluster.NewProvider()
	var err error
	restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	kubeClient, err = kubernetes.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	karmadaClient, err = karmada.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	dynamicClient, err = dynamic.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	controlPlaneClient = gclient.NewForConfigOrDie(restConfig)

	framework.InitClusterInformation(karmadaClient, controlPlaneClient)

	err = setupTestNamespace(testNamespace, kubeClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}, TestSuiteSetupTimeOut.Seconds())

var _ = ginkgo.AfterSuite(func() {
	// cleanup clusterLabels set by the E2E test
	for _, cluster := range framework.Clusters() {
		err := deleteClusterLabel(controlPlaneClient, cluster.Name)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	}

	// cleanup all namespaces we created both in control plane and member clusters.
	// It will not return error even if there is no such namespace in there that may happen in case setup failed.
	err := cleanupTestNamespace(testNamespace, kubeClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}, TestSuiteTeardownTimeOut.Seconds())

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

	cmd := exec.Command(
		"docker", "inspect",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		controlPlane,
	)
	lines, err := exec.OutputLines(cmd)
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
	err := wait.PollImmediate(2*time.Second, 10*time.Second, func() (done bool, err error) {
		clusterObj := &clusterv1alpha1.Cluster{}
		if err := c.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		delete(clusterObj.Labels, "location")
		if err := c.Update(context.TODO(), clusterObj); err != nil {
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
