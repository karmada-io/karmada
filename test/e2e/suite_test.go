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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	// TestSuiteSetupTimeOut defines the time after which the suite setup times out.
	TestSuiteSetupTimeOut = 300 * time.Second
	// TestSuiteTeardownTimeOut defines the time after which the suite tear down times out.
	TestSuiteTeardownTimeOut = 300 * time.Second

	// MinimumMemberCluster represents the minimum number of member clusters to run E2E test.
	MinimumMemberCluster = 2
)

var (
	kubeconfig    string
	restConfig    *rest.Config
	kubeClient    kubernetes.Interface
	karmadaClient karmada.Interface
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

	meetRequirement, err := isMemberClusterMeetRequirements(karmadaClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(meetRequirement).Should(gomega.BeTrue())
}, TestSuiteSetupTimeOut.Seconds())

var _ = ginkgo.AfterSuite(func() {
	// suite tear down, such as cleanup karmada environment.
}, TestSuiteTeardownTimeOut.Seconds())

// isMemberClusterMeetRequirements checks if current environment meet the requirements of E2E.
func isMemberClusterMeetRequirements(client karmada.Interface) (bool, error) {
	// list all member cluster we have
	clusters, err := client.MemberclusterV1alpha1().MemberClusters().List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return false, err
	}

	// check if member cluster number meets requirements
	if len(clusters.Items) < MinimumMemberCluster {
		return false, fmt.Errorf("needs at lease %d member cluster to run, but got: %d", MinimumMemberCluster, len(clusters.Items))
	}

	// check if all member cluster status is ready
	for _, cluster := range clusters.Items {
		if !util.IsMemberClusterReady(&cluster) {
			return false, fmt.Errorf("cluster %s not ready", cluster.GetName())
		}
	}

	klog.Infof("Got %d member cluster and all in ready state.", len(clusters.Items))
	return true, nil
}
