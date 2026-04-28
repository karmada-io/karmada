/*
Copyright 2021 The Karmada Authors.

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

package client

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	estimatorservice "github.com/karmada-io/karmada/pkg/estimator/service"
	"github.com/karmada-io/karmada/pkg/util/grpcconnection"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// SchedulerEstimatorCache is a cache that stores gRPC clients of all the cluster accurate scheduler estimator.
type SchedulerEstimatorCache struct {
	lock      sync.RWMutex
	estimator map[string]*clientWrapper
}

// SchedulerEstimatorServiceInfo contains information needed to discover and connect to a scheduler estimator service.
type SchedulerEstimatorServiceInfo struct {
	Name       string
	NamePrefix string
	Namespace  string
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
func EstablishConnection(kubeClient kubernetes.Interface, serviceInfo SchedulerEstimatorServiceInfo, estimatorCache *SchedulerEstimatorCache, grpcConfig *grpcconnection.ClientConfig) error {
	if estimatorCache.IsEstimatorExist(serviceInfo.Name) {
		return nil
	}

	serverAddrs, err := resolveCluster(kubeClient, serviceInfo.Namespace,
		names.GenerateEstimatorServiceName(serviceInfo.NamePrefix, serviceInfo.Name), int32(grpcConfig.TargetPort)) // #nosec G115: integer overflow conversion int -> int32
	if err != nil {
		return err
	}

	klog.Infof("Start dialing estimator server(%s) of cluster(%s).", strings.Join(serverAddrs, ","), serviceInfo.Name)
	cc, err := grpcConfig.DialWithTimeOut(serverAddrs, 5*time.Second)
	if err != nil {
		klog.Errorf("Failed to dial cluster(%s): %v.", serviceInfo.Name, err)
		return err
	}
	c := estimatorservice.NewEstimatorClient(cc)
	estimatorCache.AddCluster(serviceInfo.Name, cc, c)
	klog.Infof("Connection with estimator server(%s) of cluster(%s) has been established.", cc.Target(), serviceInfo.Name)
	return nil
}
