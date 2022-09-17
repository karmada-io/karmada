package client

import (
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	estimatorservice "github.com/karmada-io/karmada/pkg/estimator/service"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// SchedulerEstimatorCache is a cache that stores gRPC clients of all the cluster accurate scheduler estimator.
type SchedulerEstimatorCache struct {
	lock      sync.RWMutex
	estimator map[string]*clientWrapper
}

// NewSchedulerEstimatorCache returns an accurate scheduler estimator cache.
func NewSchedulerEstimatorCache() *SchedulerEstimatorCache {
	return &SchedulerEstimatorCache{
		estimator: make(map[string]*clientWrapper),
	}
}

type clientWrapper struct {
	connection *grpc.ClientConn
	client     estimatorservice.EstimatorClient
}

// IsEstimatorExist checks whether the cluster estimator exists in the cache.
func (c *SchedulerEstimatorCache) IsEstimatorExist(name string) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	_, exist := c.estimator[name]
	return exist
}

// AddCluster adds a grpc connection and associated client into the cache.
func (c *SchedulerEstimatorCache) AddCluster(name string, connection *grpc.ClientConn, client estimatorservice.EstimatorClient) {
	c.lock.Lock()
	defer c.lock.Unlock()
	// If more than one worker have established connections at the same time,
	// only the first one will be used. The ladder ones would be abandoned.
	if _, exist := c.estimator[name]; exist {
		_ = connection.Close()
		return
	}
	c.estimator[name] = &clientWrapper{
		connection: connection,
		client:     client,
	}
}

// DeleteCluster closes the connection and deletes it from the cache.
func (c *SchedulerEstimatorCache) DeleteCluster(name string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	es, exist := c.estimator[name]
	if !exist {
		return
	}
	_ = es.connection.Close()
	delete(c.estimator, name)
}

// GetClient returns the gRPC client of a cluster accurate replica estimator.
func (c *SchedulerEstimatorCache) GetClient(name string) (estimatorservice.EstimatorClient, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	es, exist := c.estimator[name]
	if !exist {
		return nil, fmt.Errorf("cluster %s does not exist in estimator cache", name)
	}
	return es.client, nil
}

// EstablishConnection establishes a new gRPC connection with the specified cluster scheduler estimator.
func EstablishConnection(kubeClient kubernetes.Interface, name string, estimatorCache *SchedulerEstimatorCache, estimatorServicePrefix string, port int) error {
	if estimatorCache.IsEstimatorExist(name) {
		return nil
	}

	serverAddr, err := resolveCluster(kubeClient, util.NamespaceKarmadaSystem,
		names.GenerateEstimatorServiceName(estimatorServicePrefix, name), int32(port))
	if err != nil {
		return err
	}

	klog.Infof("Start dialing estimator server(%s) of cluster(%s).", serverAddr, name)
	cc, err := util.Dial(serverAddr, 5*time.Second)
	if err != nil {
		klog.Errorf("Failed to dial cluster(%s): %v.", name, err)
		return err
	}
	c := estimatorservice.NewEstimatorClient(cc)
	estimatorCache.AddCluster(name, cc, c)
	klog.Infof("Connection with estimator server(%s) of cluster(%s) has been established.", serverAddr, name)
	return nil
}
