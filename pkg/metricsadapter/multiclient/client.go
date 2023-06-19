package multiclient

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/informers"
	listcorev1 "k8s.io/client-go/listers/core/v1"

	clusterV1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

// MultiClusterDiscoveryInterface provides DiscoveryClient for multiple clusters.
type MultiClusterDiscoveryInterface interface {
	Get(clusterName string) *discovery.DiscoveryClient
	Set(clusterName string) error
	Remove(clusterName string)
}

// MultiClusterDiscovery provides DiscoveryClient for multiple clusters.
type MultiClusterDiscovery struct {
	sync.RWMutex
	clients       map[string]*discovery.DiscoveryClient
	secretLister  listcorev1.SecretLister
	clusterLister clusterlister.ClusterLister
}

// NewMultiClusterDiscoveryClient returns a new MultiClusterDiscovery
func NewMultiClusterDiscoveryClient(clusterLister clusterlister.ClusterLister, KubeFactory informers.SharedInformerFactory) MultiClusterDiscoveryInterface {
	return &MultiClusterDiscovery{
		clusterLister: clusterLister,
		secretLister:  KubeFactory.Core().V1().Secrets().Lister(),
		clients:       map[string]*discovery.DiscoveryClient{},
	}
}

// Get returns a DiscoveryClient for the provided clusterName.
func (m *MultiClusterDiscovery) Get(clusterName string) *discovery.DiscoveryClient {
	m.RLock()
	defer m.RUnlock()
	return m.clients[clusterName]
}

// Set a DiscoveryClient for the provided clusterName.
func (m *MultiClusterDiscovery) Set(clusterName string) error {
	clusterGetter := func(cluster string) (*clusterV1alpha1.Cluster, error) {
		return m.clusterLister.Get(cluster)
	}
	secretGetter := func(namespace string, name string) (*corev1.Secret, error) {
		return m.secretLister.Secrets(namespace).Get(name)
	}
	clusterConfig, err := util.BuildClusterConfig(clusterName, clusterGetter, secretGetter)
	if err != nil {
		return err
	}
	m.Lock()
	defer m.Unlock()
	m.clients[clusterName] = discovery.NewDiscoveryClientForConfigOrDie(clusterConfig)
	return nil
}

// Remove a DiscoveryClient for the provided clusterName.
func (m *MultiClusterDiscovery) Remove(clusterName string) {
	m.Lock()
	defer m.Unlock()
	delete(m.clients, clusterName)
}
