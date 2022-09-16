package server

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/kr/pretty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
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
	"github.com/karmada-io/karmada/pkg/estimator/server/metrics"
	nodeutil "github.com/karmada-io/karmada/pkg/estimator/server/nodes"
	"github.com/karmada-io/karmada/pkg/estimator/server/replica"
	estimatorservice "github.com/karmada-io/karmada/pkg/estimator/service"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

const (
	nodeNameKeyIndex = "spec.nodeName"
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
	port            int
	clusterName     string
	kubeClient      kubernetes.Interface
	restMapper      meta.RESTMapper
	informerFactory informers.SharedInformerFactory
	nodeInformer    infov1.NodeInformer
	podInformer     infov1.PodInformer
	nodeLister      listv1.NodeLister
	replicaLister   *replica.ListerWrapper
	getPodFunc      func(nodeName string) ([]*corev1.Pod, error)
	informerManager genericmanager.SingleClusterInformerManager
	parallelizer    lifted.Parallelizer
}

// NewEstimatorServer creates an instance of AccurateSchedulerEstimatorServer.
func NewEstimatorServer(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	discoveryClient discovery.DiscoveryInterface,
	opts *options.Options,
	stopChan <-chan struct{},
) *AccurateSchedulerEstimatorServer {
	cachedDiscoClient := cacheddiscovery.NewMemCacheClient(discoveryClient)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoClient)
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	es := &AccurateSchedulerEstimatorServer{
		port:            opts.ServerPort,
		clusterName:     opts.ClusterName,
		kubeClient:      kubeClient,
		restMapper:      restMapper,
		informerFactory: informerFactory,
		nodeInformer:    informerFactory.Core().V1().Nodes(),
		podInformer:     informerFactory.Core().V1().Pods(),
		nodeLister:      informerFactory.Core().V1().Nodes().Lister(),
		replicaLister: &replica.ListerWrapper{
			PodLister:        informerFactory.Core().V1().Pods().Lister(),
			ReplicaSetLister: informerFactory.Apps().V1().ReplicaSets().Lister(),
		},
		parallelizer: lifted.NewParallelizer(opts.Parallelism),
	}
	// ignore the error here because the informers haven't been started
	_ = es.nodeInformer.Informer().SetTransform(fedinformer.StripUnusedFields)
	_ = es.podInformer.Informer().SetTransform(fedinformer.StripUnusedFields)
	_ = informerFactory.Apps().V1().ReplicaSets().Informer().SetTransform(fedinformer.StripUnusedFields)

	// Establish a connection between the pods and their assigned nodes.
	_ = es.podInformer.Informer().AddIndexers(cache.Indexers{
		nodeNameKeyIndex: func(obj interface{}) ([]string, error) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return []string{}, nil
			}
			if len(pod.Spec.NodeName) == 0 {
				return []string{}, nil
			}
			return []string{pod.Spec.NodeName}, nil
		},
	})

	// The indexer helps us get all the pods that assigned to a node.
	podIndexer := es.podInformer.Informer().GetIndexer()
	es.getPodFunc = func(nodeName string) ([]*corev1.Pod, error) {
		objs, err := podIndexer.ByIndex(nodeNameKeyIndex, nodeName)
		if err != nil {
			return nil, err
		}
		pods := make([]*corev1.Pod, 0, len(objs))
		for _, obj := range objs {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				continue
			}
			// Succeeded and failed pods are not considered because they don't occupy any resource.
			// See https://github.com/kubernetes/kubernetes/blob/f61ed439882e34d9dad28b602afdc852feb2337a/pkg/scheduler/scheduler.go#L756-L763
			if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
				pods = append(pods, pod)
			}
		}
		return pods, nil
	}
	es.informerManager = genericmanager.NewSingleClusterInformerManager(dynamicClient, 0, stopChan)
	for _, gvr := range supportedGVRs {
		es.informerManager.Lister(gvr)
	}
	return es
}

// Start runs the accurate replica estimator server.
func (es *AccurateSchedulerEstimatorServer) Start(ctx context.Context) error {
	stopCh := ctx.Done()
	klog.Infof("Starting karmada cluster(%s) accurate scheduler estimator", es.clusterName)
	defer klog.Infof("Shutting down cluster(%s) accurate scheduler estimator", es.clusterName)

	es.informerFactory.Start(stopCh)
	if !es.waitForCacheSync(stopCh) {
		return fmt.Errorf("failed to wait for cache sync")
	}

	es.informerManager.Start()
	synced := es.informerManager.WaitForCacheSync()
	if synced == nil {
		return fmt.Errorf("informer factory for cluster does not exist")
	}

	// Listen a port and register the gRPC server.
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", es.port))
	if err != nil {
		return fmt.Errorf("failed to listen port %d: %v", es.port, err)
	}
	klog.Infof("Listening port: %d", es.port)
	defer l.Close()

	s := grpc.NewServer()
	estimatorservice.RegisterEstimatorServer(s, es)

	// Graceful stop when the context is cancelled.
	go func() {
		<-stopCh
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

	defer traceMaxAvailableReplicas(object, time.Now(), request)(&response, &rerr)

	if request.Cluster != es.clusterName {
		return nil, fmt.Errorf("cluster name does not match, got: %s, desire: %s", request.Cluster, es.clusterName)
	}

	// Step 1: Get all matched nodes by node claim
	startTime := time.Now()
	ncw, err := nodeutil.NewNodeClaimWrapper(request.ReplicaRequirements.NodeClaim)
	if err != nil {
		return nil, fmt.Errorf("failed to new node claim wrapper: %v", err)
	}
	nodes, err := ncw.ListNodesByLabelSelector(es.nodeLister)
	if err != nil {
		return nil, fmt.Errorf("failed to list matched nodes by label selector: %v", err)
	}
	metrics.UpdateEstimatingAlgorithmLatency(err, metrics.EstimatingTypeMaxAvailableReplicas, metrics.EstimatingStepListNodesByNodeClaim, startTime)

	// Step 2: Calculate cluster max available replicas by filtered nodes concurrently
	startTime = time.Now()
	maxReplicas := es.maxAvailableReplicas(ctx, ncw, nodes, request.ReplicaRequirements.ResourceRequest)
	metrics.UpdateEstimatingAlgorithmLatency(nil, metrics.EstimatingTypeMaxAvailableReplicas, metrics.EstimatingStepMaxAvailableReplicas, startTime)

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

	defer traceGetUnschedulableReplicas(object, time.Now(), request)(&response, &rerr)

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

func (es *AccurateSchedulerEstimatorServer) maxAvailableReplicas(
	ctx context.Context,
	ncw *nodeutil.NodeClaimWrapper,
	nodes []*corev1.Node,
	request corev1.ResourceList,
) int32 {
	var res int32
	processNode := func(i int) {
		node := nodes[i]
		if node == nil {
			return
		}
		if !ncw.IsNodeMatched(node) {
			return
		}
		maxReplica, err := es.nodeMaxAvailableReplica(node, request)
		if err != nil {
			klog.Errorf("Error: %v", err)
			return
		}
		atomic.AddInt32(&res, maxReplica)
	}
	es.parallelizer.Until(ctx, len(nodes), processNode)
	return res
}

func (es *AccurateSchedulerEstimatorServer) nodeMaxAvailableReplica(node *corev1.Node, request corev1.ResourceList) (int32, error) {
	pods, err := es.getPodFunc(node.Name)
	if err != nil {
		return 0, fmt.Errorf("failed to get pods that assigned to node %s, err: %v", node.Name, err)
	}
	maxReplica, err := replica.NodeMaxAvailableReplica(node, pods, request)
	if err != nil {
		return 0, fmt.Errorf("failed to calculating max replica of node %s, err: %v", node.Name, err)
	}
	return maxReplica, nil
}

func (es *AccurateSchedulerEstimatorServer) waitForCacheSync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh, es.podInformer.Informer().HasSynced, es.nodeInformer.Informer().HasSynced)
}

func traceMaxAvailableReplicas(object string, start time.Time, request *pb.MaxAvailableReplicasRequest) func(response **pb.MaxAvailableReplicasResponse, err *error) {
	klog.V(4).Infof("Begin calculating cluster available replicas of resource(%s), request: %s", object, pretty.Sprint(*request))
	return func(response **pb.MaxAvailableReplicasResponse, err *error) {
		metrics.CountRequests(*err, metrics.EstimatingTypeMaxAvailableReplicas)
		metrics.UpdateEstimatingAlgorithmLatency(*err, metrics.EstimatingTypeMaxAvailableReplicas, metrics.EstimatingStepTotal, start)
		if *err != nil {
			klog.Errorf("Failed to calculate cluster available replicas: %v", *err)
			return
		}
		klog.V(2).Infof("Finish calculating cluster available replicas of resource(%s), max replicas: %d, time elapsed: %s", object, (*response).MaxReplicas, time.Since(start))
	}
}

func traceGetUnschedulableReplicas(object string, start time.Time, request *pb.UnschedulableReplicasRequest) func(response **pb.UnschedulableReplicasResponse, err *error) {
	klog.V(4).Infof("Begin detecting cluster unschedulable replicas of resource(%s), request: %s", object, pretty.Sprint(*request))
	return func(response **pb.UnschedulableReplicasResponse, err *error) {
		metrics.CountRequests(*err, metrics.EstimatingTypeGetUnschedulableReplicas)
		metrics.UpdateEstimatingAlgorithmLatency(*err, metrics.EstimatingTypeGetUnschedulableReplicas, metrics.EstimatingStepTotal, start)
		if *err != nil {
			klog.Errorf("Failed to detect cluster unschedulable replicas: %v", *err)
			return
		}
		klog.V(2).Infof("Finish detecting cluster unschedulable replicas of resource(%s), unschedulable replicas: %d, time elapsed: %s", object, (*response).UnschedulableReplicas, time.Since(start))
	}
}
