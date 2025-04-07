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

package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/kr/pretty"
	"google.golang.org/grpc/metadata"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	infov1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/scheduler-estimator/app/options"
	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	frameworkplugins "github.com/karmada-io/karmada/pkg/estimator/server/framework/plugins"
	frameworkruntime "github.com/karmada-io/karmada/pkg/estimator/server/framework/runtime"
	"github.com/karmada-io/karmada/pkg/estimator/server/metrics"
	"github.com/karmada-io/karmada/pkg/estimator/server/replica"
	estimatorservice "github.com/karmada-io/karmada/pkg/estimator/service"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/grpcconnection"
	"github.com/karmada-io/karmada/pkg/util/helper"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework/parallelize"
)

const (
	// Duration the scheduler will wait before expiring an assumed pod.
	durationToExpireAssumedPod time.Duration = 0
)

var (
	// TODO(Garrybest): make it as an option
	supportedGVRs = []schema.GroupVersionResource{
		appsv1.SchemeGroupVersion.WithResource("deployments"),
	}
)

// AccurateSchedulerEstimatorServer is the gRPC server of a cluster accurate scheduler estimator.
// Please see https://github.com/karmada-io/karmada/pull/580 (#580).
type AccurateSchedulerEstimatorServer struct {
	clusterName       string
	kubeClient        kubernetes.Interface
	restMapper        meta.RESTMapper
	informerFactory   informers.SharedInformerFactory
	nodeLister        listv1.NodeLister
	replicaLister     *replica.ListerWrapper
	informerManager   genericmanager.SingleClusterInformerManager
	parallelizer      parallelize.Parallelizer
	estimateFramework framework.Framework

	Cache schedcache.Cache

	GrpcConfig *grpcconnection.ServerConfig
}

// NewEstimatorServer creates an instance of AccurateSchedulerEstimatorServer.
func NewEstimatorServer(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	discoveryClient discovery.DiscoveryInterface,
	opts *options.Options,
) (*AccurateSchedulerEstimatorServer, error) {
	cachedDiscoClient := cacheddiscovery.NewMemCacheClient(discoveryClient)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoClient)
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	informerFactory.InformerFor(&corev1.Pod{}, newPodInformer)

	es := &AccurateSchedulerEstimatorServer{
		clusterName:     opts.ClusterName,
		kubeClient:      kubeClient,
		restMapper:      restMapper,
		informerFactory: informerFactory,
		nodeLister:      informerFactory.Core().V1().Nodes().Lister(),
		replicaLister: &replica.ListerWrapper{
			PodLister:        informerFactory.Core().V1().Pods().Lister(),
			ReplicaSetLister: informerFactory.Apps().V1().ReplicaSets().Lister(),
		},
		parallelizer: parallelize.NewParallelizer(opts.Parallelism),
		Cache:        schedcache.New(durationToExpireAssumedPod, ctx.Done()),
		GrpcConfig: &grpcconnection.ServerConfig{
			InsecureSkipClientVerify: opts.InsecureSkipGrpcClientVerify,
			ClientAuthCAFile:         opts.GrpcClientCaFile,
			CertFile:                 opts.GrpcAuthCertFile,
			KeyFile:                  opts.GrpcAuthKeyFile,
			ServerPort:               opts.ServerPort,
		},
	}
	// ignore the error here because the informers haven't been started
	_ = informerFactory.Core().V1().Nodes().Informer().SetTransform(fedinformer.StripUnusedFields)
	_ = informerFactory.Core().V1().Pods().Informer().SetTransform(fedinformer.StripUnusedFields)
	_ = informerFactory.Apps().V1().ReplicaSets().Informer().SetTransform(fedinformer.StripUnusedFields)

	es.informerManager = genericmanager.NewSingleClusterInformerManager(ctx, dynamicClient, 0)
	for _, gvr := range supportedGVRs {
		es.informerManager.Lister(gvr)
	}

	registry := frameworkplugins.NewInTreeRegistry()
	estimateFramework, err := frameworkruntime.NewFramework(registry,
		frameworkruntime.WithClientSet(kubeClient),
		frameworkruntime.WithInformerFactory(informerFactory),
	)
	if err != nil {
		return es, err
	}
	es.estimateFramework = estimateFramework

	addAllEventHandlers(es, informerFactory)

	return es, nil
}

// Start runs the accurate replica estimator server.
func (es *AccurateSchedulerEstimatorServer) Start(ctx context.Context) error {
	klog.Infof("Starting karmada cluster(%s) accurate scheduler estimator", es.clusterName)
	defer klog.Infof("Shutting down cluster(%s) accurate scheduler estimator", es.clusterName)

	es.informerFactory.Start(ctx.Done())
	es.informerFactory.WaitForCacheSync(ctx.Done())

	es.informerManager.Start()
	if synced := es.informerManager.WaitForCacheSync(); synced == nil {
		return fmt.Errorf("informer factory for cluster does not exist")
	}

	// Listen a port and register the gRPC server.
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", es.GrpcConfig.ServerPort))
	if err != nil {
		return fmt.Errorf("failed to listen port %d: %v", es.GrpcConfig.ServerPort, err)
	}
	klog.Infof("Listening port: %d", es.GrpcConfig.ServerPort)
	defer l.Close()

	s, err := es.GrpcConfig.NewServer()
	if err != nil {
		return fmt.Errorf("failed to create grpc server: %v", err)
	}
	estimatorservice.RegisterEstimatorServer(s, es)

	// Graceful stop when the context is cancelled.
	go func() {
		<-ctx.Done()
		s.GracefulStop()
	}()

	// Start the gRPC server.
	if err := s.Serve(l); err != nil {
		return err
	}

	// Should never reach here.
	return nil
}

// MaxAvailableReplicas is the implementation of gRPC interface. It will return the
// max available replicas that a cluster could accommodate based on its requirements.
func (es *AccurateSchedulerEstimatorServer) MaxAvailableReplicas(ctx context.Context, request *pb.MaxAvailableReplicasRequest) (response *pb.MaxAvailableReplicasResponse, rerr error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		klog.Warningf("No metadata from context.")
	}
	var object string
	if m := md.Get(string(util.ContextKeyObject)); len(m) != 0 {
		object = m[0]
	}

	startTime := time.Now()

	klog.V(4).Infof("Begin calculating cluster available replicas of resource(%s), request: %s", object, pretty.Sprint(*request))
	defer func(start time.Time) {
		metrics.CountRequests(rerr, metrics.EstimatingTypeMaxAvailableReplicas)
		metrics.UpdateEstimatingAlgorithmLatency(rerr, metrics.EstimatingTypeMaxAvailableReplicas, metrics.EstimatingStepTotal, start)
		if rerr != nil {
			klog.Errorf("Failed to calculate cluster available replicas: %v", rerr)
			return
		}
		klog.V(2).Infof("Finish calculating cluster available replicas of resource(%s), max replicas: %d, time elapsed: %s", object, response.MaxReplicas, time.Since(start))
	}(startTime)

	if request.Cluster != es.clusterName {
		return nil, fmt.Errorf("cluster name does not match, got: %s, desire: %s", request.Cluster, es.clusterName)
	}
	maxReplicas, err := es.EstimateReplicas(ctx, object, request)
	if err != nil {
		return nil, fmt.Errorf("failed to estimate replicas: %v", err)
	}
	return &pb.MaxAvailableReplicasResponse{MaxReplicas: maxReplicas}, nil
}

// GetUnschedulableReplicas is the implementation of gRPC interface. It will return the
// unschedulable replicas of a workload.
func (es *AccurateSchedulerEstimatorServer) GetUnschedulableReplicas(ctx context.Context, request *pb.UnschedulableReplicasRequest) (response *pb.UnschedulableReplicasResponse, rerr error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		klog.Warningf("No metadata from context.")
	}
	var object string
	if m := md.Get(string(util.ContextKeyObject)); len(m) != 0 {
		object = m[0]
	}

	klog.V(4).Infof("Begin detecting cluster unschedulable replicas of resource(%s), request: %s", object, pretty.Sprint(*request))
	defer func(start time.Time) {
		metrics.CountRequests(rerr, metrics.EstimatingTypeGetUnschedulableReplicas)
		metrics.UpdateEstimatingAlgorithmLatency(rerr, metrics.EstimatingTypeGetUnschedulableReplicas, metrics.EstimatingStepTotal, start)
		if rerr != nil {
			klog.Errorf("Failed to detect cluster unschedulable replicas: %v", rerr)
			return
		}
		klog.V(2).Infof("Finish detecting cluster unschedulable replicas of resource(%s), unschedulable replicas: %d, time elapsed: %s", object, response.UnschedulableReplicas, time.Since(start))
	}(time.Now())

	if request.Cluster != es.clusterName {
		return nil, fmt.Errorf("cluster name does not match, got: %s, desire: %s", request.Cluster, es.clusterName)
	}

	// Get the workload.
	startTime := time.Now()
	gvk := schema.FromAPIVersionAndKind(request.Resource.APIVersion, request.Resource.Kind)
	unstructObj, err := helper.GetObjectFromSingleClusterCache(es.restMapper, es.informerManager, &keys.ClusterWideKey{
		Group:     gvk.Group,
		Version:   gvk.Version,
		Kind:      gvk.Kind,
		Namespace: request.Resource.Namespace,
		Name:      request.Resource.Name,
	})
	metrics.UpdateEstimatingAlgorithmLatency(err, metrics.EstimatingTypeGetUnschedulableReplicas, metrics.EstimatingStepGetObjectFromCache, startTime)
	if err != nil {
		return nil, err
	}

	// List all unschedulable replicas.
	startTime = time.Now()
	unschedulables, err := replica.GetUnschedulablePodsOfWorkload(unstructObj, request.UnschedulableThreshold, es.replicaLister)
	metrics.UpdateEstimatingAlgorithmLatency(err, metrics.EstimatingTypeGetUnschedulableReplicas, metrics.EstimatingStepGetUnschedulablePodsOfWorkload, startTime)
	if err != nil {
		return nil, err
	}

	return &pb.UnschedulableReplicasResponse{UnschedulableReplicas: unschedulables}, err
}

// newPodInformer creates a shared index informer that returns only non-terminal pods.
// The PodInformer allows indexers to be added, but note that only non-conflict indexers are allowed.
func newPodInformer(cs kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	selector := fmt.Sprintf("status.phase!=%v,status.phase!=%v", corev1.PodSucceeded, corev1.PodFailed)
	tweakListOptions := func(options *metav1.ListOptions) {
		options.FieldSelector = selector
	}
	return infov1.NewFilteredPodInformer(cs, metav1.NamespaceAll, resyncPeriod, cache.Indexers{}, tweakListOptions)
}
