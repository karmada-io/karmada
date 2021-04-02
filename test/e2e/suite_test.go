package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/exec"

	clusterapi "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
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
	pollTimeout = 60 * time.Second

	// MinimumCluster represents the minimum number of member clusters to run E2E test.
	MinimumCluster = 2

	// RandomStrLength represents the random string length to combine names.
	RandomStrLength = 3
)

const (
	deploymentNamePrefix = "deploy-"
	serviceNamePrefix    = "service-"
	podNamePrefix        = "pod-"
	crdNamePrefix        = "cr-"

	updateDeploymentReplicas = 6
	updateServicePort        = 81
	updatePodImage           = "nginx:latest"
	updateCRnamespace        = "e2e-test"
)

var (
	kubeconfig            string
	restConfig            *rest.Config
	kubeClient            kubernetes.Interface
	karmadaClient         karmada.Interface
	dynamicClient         dynamic.Interface
	controlPlaneClient    client.Client
	clusters              []*clusterapi.Cluster
	clusterNames          []string
	clusterClients        []*util.ClusterClient
	clusterDynamicClients []*util.DynamicClusterClient
	testNamespace         = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
	clusterProvider       *cluster.Provider
	pullModeClusters      map[string]string
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

	controlPlaneClient, err = client.New(restConfig, client.Options{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	pullModeClusters, err = fetchPullBasedClusters()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	clusters, err = fetchClusters(karmadaClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	var meetRequirement bool
	meetRequirement, err = isClusterMeetRequirements(clusters)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(meetRequirement).Should(gomega.BeTrue())

	for _, cluster := range clusters {
		clusterClient, clusterDynamicClient, err := newClusterClientSet(cluster)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		clusterNames = append(clusterNames, cluster.Name)
		clusterClients = append(clusterClients, clusterClient)
		clusterDynamicClients = append(clusterDynamicClients, clusterDynamicClient)
	}

	gomega.Expect(clusterNames).Should(gomega.HaveLen(len(clusters)))

	err = setupTestNamespace(testNamespace, kubeClient, clusterClients)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}, TestSuiteSetupTimeOut.Seconds())

var _ = ginkgo.AfterSuite(func() {
	// cleanup all namespaces we created both in control plane and member clusters.
	// It will not return error even if there is no such namespace in there that may happen in case setup failed.
	err := cleanupTestNamespace(testNamespace, kubeClient, clusterClients)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}, TestSuiteTeardownTimeOut.Seconds())

func fetchPullBasedClusters() (map[string]string, error) {
	pullBasedClusters := os.Getenv("PULL_BASED_CLUSTERS")
	if pullBasedClusters == "" {
		return nil, nil
	}

	pullBasedClustersMap := make(map[string]string)
	pullBasedClusters = strings.TrimSuffix(pullBasedClusters, ";")
	clusterInfo := strings.Split(pullBasedClusters, ";")
	for _, cluster := range clusterInfo {
		clusterNameAndConfigPath := strings.Split(cluster, ":")
		if len(clusterNameAndConfigPath) != 2 {
			return nil, fmt.Errorf("failed to parse config path for cluster: %s", cluster)
		}
		pullBasedClustersMap[clusterNameAndConfigPath[0]] = clusterNameAndConfigPath[1]
	}
	return pullBasedClustersMap, nil
}

// fetchClusters will fetch all member clusters we have.
func fetchClusters(client karmada.Interface) ([]*clusterapi.Cluster, error) {
	clusterList, err := client.ClusterV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	clusters := make([]*clusterapi.Cluster, 0, len(clusterList.Items))
	for _, cluster := range clusterList.Items {
		pinedCluster := cluster
		if pinedCluster.Spec.SyncMode == clusterapi.Pull {
			if _, exist := pullModeClusters[cluster.Name]; !exist {
				continue
			}
		}
		clusters = append(clusters, &pinedCluster)
	}

	return clusters, nil
}

// isClusterMeetRequirements checks if current environment meet the requirements of E2E.
func isClusterMeetRequirements(clusters []*clusterapi.Cluster) (bool, error) {
	// check if member cluster number meets requirements
	if len(clusters) < MinimumCluster {
		return false, fmt.Errorf("needs at lease %d member cluster to run, but got: %d", MinimumCluster, len(clusters))
	}

	// check if all member cluster status is ready
	for _, cluster := range clusters {
		if !util.IsClusterReady(&cluster.Status) {
			return false, fmt.Errorf("cluster %s not ready", cluster.GetName())
		}
	}

	klog.Infof("Got %d member cluster and all in ready state.", len(clusters))
	return true, nil
}

// setupTestNamespace will create a namespace in control plane and all member clusters, most of cases will run against it.
// The reason why we need a separated namespace is it will make it easier to cleanup resources deployed by the testing.
func setupTestNamespace(namespace string, kubeClient kubernetes.Interface, clusterClients []*util.ClusterClient) error {
	namespaceObj := helper.NewNamespace(namespace)
	_, err := util.CreateNamespace(kubeClient, namespaceObj)
	if err != nil {
		return err
	}

	for _, clusterClient := range clusterClients {
		_, err = util.CreateNamespace(clusterClient.KubeClient, namespaceObj)
		if err != nil {
			return err
		}
	}

	return nil
}

// cleanupTestNamespace will remove the namespace we setup before for the whole testing.
func cleanupTestNamespace(namespace string, kubeClient kubernetes.Interface, clusterClients []*util.ClusterClient) error {
	err := util.DeleteNamespace(kubeClient, namespace)
	if err != nil {
		return err
	}

	for _, clusterClient := range clusterClients {
		err = util.DeleteNamespace(clusterClient.KubeClient, namespace)
		if err != nil {
			return err
		}
	}

	return nil
}

func getClusterClient(clusterName string) kubernetes.Interface {
	for _, client := range clusterClients {
		if client.ClusterName == clusterName {
			return client.KubeClient
		}
	}

	return nil
}

func getClusterDynamicClient(clusterName string) dynamic.Interface {
	for _, client := range clusterDynamicClients {
		if client.ClusterName == clusterName {
			return client.DynamicClientSet
		}
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

func newClusterClientSet(c *clusterapi.Cluster) (*util.ClusterClient, *util.DynamicClusterClient, error) {
	if c.Spec.SyncMode == clusterapi.Push {
		clusterClient, err := util.NewClusterClientSet(c, controlPlaneClient)
		if err != nil {
			return nil, nil, err
		}
		clusterDynamicClient, err := util.NewClusterDynamicClientSet(c, controlPlaneClient)
		if err != nil {
			return nil, nil, err
		}
		return clusterClient, clusterDynamicClient, nil
	}

	clusterConfigPath := pullModeClusters[c.Name]
	clusterConfig, err := clientcmd.BuildConfigFromFlags("", clusterConfigPath)
	if err != nil {
		return nil, nil, err
	}

	clusterClientSet := util.ClusterClient{ClusterName: c.Name}
	clusterDynamicClientSet := util.DynamicClusterClient{ClusterName: c.Name}
	clusterClientSet.KubeClient = kubernetes.NewForConfigOrDie(clusterConfig)
	clusterDynamicClientSet.DynamicClientSet = dynamic.NewForConfigOrDie(clusterConfig)

	return &clusterClientSet, &clusterDynamicClientSet, nil
}
