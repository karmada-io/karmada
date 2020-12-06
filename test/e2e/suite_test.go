package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	clusterapi "github.com/karmada-io/karmada/pkg/apis/membercluster/v1alpha1"
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

	// MinimumMemberCluster represents the minimum number of member clusters to run E2E test.
	MinimumMemberCluster = 2
)

var (
	kubeconfig           string
	restConfig           *rest.Config
	kubeClient           kubernetes.Interface
	karmadaClient        karmada.Interface
	memberClusters       []*clusterapi.MemberCluster
	memberClusterNames   []string
	memberClusterClients []*util.ClusterClient
	testNamespace        = fmt.Sprintf("karmada-e2e-%s", rand.String(3))
)

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "E2E Suite")
}

var _ = ginkgo.BeforeSuite(func() {
	kubeconfig = os.Getenv("KUBECONFIG")
	gomega.Expect(kubeconfig).ShouldNot(gomega.BeEmpty())

	var err error
	restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	kubeClient, err = kubernetes.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	karmadaClient, err = karmada.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	memberClusters, err = fetchMemberClusters(karmadaClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	var meetRequirement bool
	meetRequirement, err = isMemberClusterMeetRequirements(memberClusters)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(meetRequirement).Should(gomega.BeTrue())

	for _, cluster := range memberClusters {
		memberClusterNames = append(memberClusterNames, cluster.Name)

		memberClusterClient, err := util.NewClusterClientSet(cluster, kubeClient)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		memberClusterClients = append(memberClusterClients, memberClusterClient)
	}
	gomega.Expect(memberClusterNames).Should(gomega.HaveLen(len(memberClusters)))

	err = setupTestNamespace(testNamespace, kubeClient, memberClusterClients)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}, TestSuiteSetupTimeOut.Seconds())

var _ = ginkgo.AfterSuite(func() {
	// cleanup all namespaces we created both in control plane and member clusters.
	// It will not return error even if there is no such namespace in there that may happen in case setup failed.
	err := cleanupTestNamespace(testNamespace, kubeClient, memberClusterClients)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}, TestSuiteTeardownTimeOut.Seconds())

// fetchMemberClusters will fetch all member clusters we have.
func fetchMemberClusters(client karmada.Interface) ([]*clusterapi.MemberCluster, error) {
	clusterList, err := client.MemberclusterV1alpha1().MemberClusters().List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	clusters := make([]*clusterapi.MemberCluster, 0, len(clusterList.Items))
	for _, cluster := range clusterList.Items {
		pinedCluster := cluster
		clusters = append(clusters, &pinedCluster)
	}

	return clusters, nil
}

// isMemberClusterMeetRequirements checks if current environment meet the requirements of E2E.
func isMemberClusterMeetRequirements(clusters []*clusterapi.MemberCluster) (bool, error) {
	// check if member cluster number meets requirements
	if len(clusters) < MinimumMemberCluster {
		return false, fmt.Errorf("needs at lease %d member cluster to run, but got: %d", MinimumMemberCluster, len(clusters))
	}

	// check if all member cluster status is ready
	for _, cluster := range clusters {
		if !util.IsMemberClusterReady(&cluster.Status) {
			return false, fmt.Errorf("cluster %s not ready", cluster.GetName())
		}
	}

	klog.Infof("Got %d member cluster and all in ready state.", len(clusters))
	return true, nil
}

// setupTestNamespace will create a namespace in control plane and all member clusters, most of cases will run against it.
// The reason why we need a separated namespace is it will make it easier to cleanup resources deployed by the testing.
func setupTestNamespace(namespace string, kubeClient kubernetes.Interface, memberClusterClients []*util.ClusterClient) error {
	namespaceObj := helper.NewNamespace(namespace)
	_, err := util.CreateNamespace(kubeClient, namespaceObj)
	if err != nil {
		return err
	}

	for _, clusterClient := range memberClusterClients {
		_, err = util.CreateNamespace(clusterClient.KubeClient, namespaceObj)
		if err != nil {
			return err
		}
	}

	return nil
}

// cleanupTestNamespace will remove the namespace we setup before for the whole testing.
func cleanupTestNamespace(namespace string, kubeClient kubernetes.Interface, memberClusterClients []*util.ClusterClient) error {
	err := util.DeleteNamespace(kubeClient, namespace)
	if err != nil {
		return err
	}

	for _, clusterClient := range memberClusterClients {
		err = util.DeleteNamespace(clusterClient.KubeClient, namespace)
		if err != nil {
			return err
		}
	}

	return nil
}

func getClusterClient(clusterName string) kubernetes.Interface {
	for _, client := range memberClusterClients {
		if client.ClusterName == clusterName {
			return client.KubeClient
		}
	}

	return nil
}
