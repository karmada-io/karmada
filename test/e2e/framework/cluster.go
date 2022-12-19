package framework

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	// MinimumCluster represents the minimum number of member clusters to run E2E test.
	MinimumCluster = 2
)

var (
	clusters              []*clusterv1alpha1.Cluster
	clusterNames          []string
	clusterClients        []*util.ClusterClient
	clusterDynamicClients []*util.DynamicClusterClient
	pullModeClusters      map[string]string
)

// Clusters will return all member clusters we have.
func Clusters() []*clusterv1alpha1.Cluster {
	return clusters
}

// ClusterNames will return all member clusters' names we have.
func ClusterNames() []string {
	return clusterNames
}

// ClusterNamesWithSyncMode will return member clusters' names which matches the sync mode.
func ClusterNamesWithSyncMode(mode clusterv1alpha1.ClusterSyncMode) []string {
	res := make([]string, 0, len(clusterNames))
	for _, cluster := range clusters {
		if cluster.Spec.SyncMode == mode {
			res = append(res, cluster.Name)
		}
	}
	return res
}

// InitClusterInformation init the E2E test's cluster information.
func InitClusterInformation(karmadaClient karmada.Interface, controlPlaneClient client.Client) {
	var err error

	pullModeClusters, err = fetchPullBasedClusters()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	clusters, err = fetchClusters(karmadaClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	var meetRequirement bool
	meetRequirement, err = isClusterMeetRequirements(clusters)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(meetRequirement).Should(gomega.BeTrue())

	for _, cluster := range clusters {
		clusterClient, clusterDynamicClient, err := newClusterClientSet(controlPlaneClient, cluster)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		clusterNames = append(clusterNames, cluster.Name)
		clusterClients = append(clusterClients, clusterClient)
		clusterDynamicClients = append(clusterDynamicClients, clusterDynamicClient)

		err = setClusterLabel(controlPlaneClient, cluster.Name)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	}
	gomega.Expect(clusterNames).Should(gomega.HaveLen(len(clusters)))
}

// GetClusterClient get cluster client
func GetClusterClient(clusterName string) kubernetes.Interface {
	for _, clusterClient := range clusterClients {
		if clusterClient.ClusterName == clusterName {
			return clusterClient.KubeClient
		}
	}

	return nil
}

// GetClusterDynamicClient get cluster dynamicClient
func GetClusterDynamicClient(clusterName string) dynamic.Interface {
	for _, clusterClient := range clusterDynamicClients {
		if clusterClient.ClusterName == clusterName {
			return clusterClient.DynamicClientSet
		}
	}

	return nil
}

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
func fetchClusters(client karmada.Interface) ([]*clusterv1alpha1.Cluster, error) {
	clusterList, err := client.ClusterV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	clusters := make([]*clusterv1alpha1.Cluster, 0, len(clusterList.Items))
	for _, cluster := range clusterList.Items {
		pinedCluster := cluster
		if pinedCluster.Spec.SyncMode == clusterv1alpha1.Pull {
			if _, exist := pullModeClusters[cluster.Name]; !exist {
				continue
			}
		}
		clusters = append(clusters, &pinedCluster)
	}

	return clusters, nil
}

// FetchCluster will fetch member cluster by name.
func FetchCluster(client karmada.Interface, clusterName string) (*clusterv1alpha1.Cluster, error) {
	cluster, err := client.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return cluster, nil
}

// isClusterMeetRequirements checks if current environment meet the requirements of E2E.
func isClusterMeetRequirements(clusters []*clusterv1alpha1.Cluster) (bool, error) {
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

func newClusterClientSet(controlPlaneClient client.Client, c *clusterv1alpha1.Cluster) (*util.ClusterClient, *util.DynamicClusterClient, error) {
	if c.Spec.SyncMode == clusterv1alpha1.Push {
		clusterClient, err := util.NewClusterClientSet(c.Name, controlPlaneClient, nil)
		if err != nil {
			return nil, nil, err
		}
		clusterDynamicClient, err := util.NewClusterDynamicClientSet(c.Name, controlPlaneClient)
		if err != nil {
			return nil, nil, err
		}
		return clusterClient, clusterDynamicClient, nil
	}

	clusterConfigPath := pullModeClusters[c.Name]
	clusterConfig, err := LoadRESTClientConfig(clusterConfigPath, c.Name)
	if err != nil {
		return nil, nil, err
	}

	clusterClientSet := util.ClusterClient{ClusterName: c.Name}
	clusterDynamicClientSet := util.DynamicClusterClient{ClusterName: c.Name}
	clusterClientSet.KubeClient = kubernetes.NewForConfigOrDie(clusterConfig)
	clusterDynamicClientSet.DynamicClientSet = dynamic.NewForConfigOrDie(clusterConfig)

	return &clusterClientSet, &clusterDynamicClientSet, nil
}

// setClusterLabel set cluster label of E2E
func setClusterLabel(c client.Client, clusterName string) error {
	err := wait.PollImmediate(2*time.Second, 10*time.Second, func() (done bool, err error) {
		clusterObj := &clusterv1alpha1.Cluster{}
		if err := c.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		if clusterObj.Labels == nil {
			clusterObj.Labels = make(map[string]string)
		}
		clusterObj.Labels["location"] = "CHN"
		if clusterObj.Spec.SyncMode == clusterv1alpha1.Push {
			clusterObj.Labels["sync-mode"] = "Push"
		}
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

// UpdateClusterLabels updates cluster labels.
func UpdateClusterLabels(client karmada.Interface, clusterName string, labels map[string]string) {
	gomega.Eventually(func() (bool, error) {
		cluster, err := client.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if cluster.Labels == nil {
			cluster.Labels = map[string]string{}
		}
		for key, value := range labels {
			cluster.Labels[key] = value
		}
		_, err = client.ClusterV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
		return true, nil
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// DeleteClusterLabels deletes cluster labels if it exists.
func DeleteClusterLabels(client karmada.Interface, clusterName string, labels map[string]string) {
	gomega.Eventually(func() (bool, error) {
		cluster, err := client.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if cluster.Labels == nil {
			return true, nil
		}
		for key := range labels {
			delete(cluster.Labels, key)
		}
		_, err = client.ClusterV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
		return true, nil
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// GetClusterNamesFromClusters will get Clusters' names form Clusters Object.
func GetClusterNamesFromClusters(clusters []*clusterv1alpha1.Cluster) []string {
	clusterNames := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		clusterNames = append(clusterNames, cluster.Name)
	}
	return clusterNames
}

// WaitClusterFitWith wait cluster fit with fit func.
func WaitClusterFitWith(c client.Client, clusterName string, fit func(cluster *clusterv1alpha1.Cluster) bool) {
	gomega.Eventually(func() (bool, error) {
		currentCluster, err := util.GetCluster(c, clusterName)
		if err != nil {
			return false, err
		}
		return fit(currentCluster), nil
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// LoadRESTClientConfig creates a rest.Config using the passed kubeconfig. If context is empty, current context in kubeconfig will be used.
func LoadRESTClientConfig(kubeconfig string, context string) (*rest.Config, error) {
	loader := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	loadedConfig, err := loader.Load()
	if err != nil {
		return nil, err
	}

	if context == "" {
		context = loadedConfig.CurrentContext
	}
	klog.Infof("Use context %v", context)

	return clientcmd.NewNonInteractiveClientConfig(
		*loadedConfig,
		context,
		&clientcmd.ConfigOverrides{},
		loader,
	).ClientConfig()
}

// SetClusterRegion sets .Spec.Region field for Cluster object.
func SetClusterRegion(c client.Client, clusterName string, regionName string) error {
	return wait.PollImmediate(2*time.Second, 10*time.Second, func() (done bool, err error) {
		clusterObj := &clusterv1alpha1.Cluster{}
		if err := c.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}

		clusterObj.Spec.Region = regionName
		if err := c.Update(context.TODO(), clusterObj); err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}
