package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/kr/pretty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	infov1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/scheduler-estimator/app/options"
	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/metrics"
	nodeutil "github.com/karmada-io/karmada/pkg/estimator/server/nodes"
	"github.com/karmada-io/karmada/pkg/estimator/server/replica"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	nodeNameKeyIndex = "spec.nodeName"
)

// AccurateSchedulerEstimatorServer is the gRPC server of a cluster accurate scheduler estimator.
// Please see https://github.com/karmada-io/karmada/pull/580 (#580).
type AccurateSchedulerEstimatorServer struct {
	port            int
	clusterName     string
	kubeClient      kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	nodeInformer    infov1.NodeInformer
	podInformer     infov1.PodInformer
	nodeLister      listv1.NodeLister
	podLister       listv1.PodLister
	getPodFunc      func(nodeName string) ([]*corev1.Pod, error)
}

// NewEstimatorServer creates an instance of AccurateSchedulerEstimatorServer.
func NewEstimatorServer(kubeClient kubernetes.Interface, opts *options.Options) *AccurateSchedulerEstimatorServer {
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	es := &AccurateSchedulerEstimatorServer{
		port:            opts.ServerPort,
		clusterName:     opts.ClusterName,
		kubeClient:      kubeClient,
		informerFactory: informerFactory,
		nodeInformer:    informerFactory.Core().V1().Nodes(),
		podInformer:     informerFactory.Core().V1().Pods(),
		nodeLister:      informerFactory.Core().V1().Nodes().Lister(),
		podLister:       informerFactory.Core().V1().Pods().Lister(),
	}

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
	return es
}

// Start runs the accurate replica estimator server.
func (es *AccurateSchedulerEstimatorServer) Start(ctx context.Context) error {
	stopCh := ctx.Done()
	klog.Infof("Starting karmada cluster(%s) accurate replica estimator", es.clusterName)
	defer klog.Infof("Shutting down cluster(%s) accurate replica estimator", es.clusterName)

	es.informerFactory.Start(stopCh)
	if !es.waitForCacheSync(stopCh) {
		return fmt.Errorf("failed to wait for cache sync")
	}

	// Listen a port and register the gRPC server.
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", es.port))
	if err != nil {
		return fmt.Errorf("failed to listen port %d: %v", es.port, err)
	}
	klog.Infof("Listening port: %d", es.port)
	defer l.Close()

	s := grpc.NewServer()
	pb.RegisterEstimatorServer(s, es)

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
	nodes, err := nodeutil.ListNodesByNodeClaim(es.nodeLister, request.ReplicaRequirements.NodeClaim)
	metrics.UpdateEstimatingAlgorithmLatency(err, metrics.EstimatingTypeMaxAvailableReplicas, metrics.EstimatingStepListNodesByNodeClaim, startTime)
	if err != nil {
		return nil, fmt.Errorf("failed to find matched nodes: %v", err)
	}

	// Step 2: Calculate cluster max available replicas by filtered nodes
	startTime = time.Now()
	maxReplicas := es.maxAvailableReplicas(nodes, request.ReplicaRequirements.ResourceRequest)
	metrics.UpdateEstimatingAlgorithmLatency(nil, metrics.EstimatingTypeMaxAvailableReplicas, metrics.EstimatingStepMaxAvailableReplicas, startTime)

	return &pb.MaxAvailableReplicasResponse{MaxReplicas: maxReplicas}, nil
}

// GetUnschedulableReplicas is the implementation of gRPC interface. It will return the
// unschedulable replicas of a workload.
func (es *AccurateSchedulerEstimatorServer) GetUnschedulableReplicas(ctx context.Context, request *pb.UnschedulableReplicasRequest) (*pb.UnschedulableReplicasResponse, error) {
	//TODO(Garrybest): implement me
	return nil, nil
}

func (es *AccurateSchedulerEstimatorServer) maxAvailableReplicas(nodes []*corev1.Node, request corev1.ResourceList) int32 {
	var maxReplicas int32
	for _, node := range nodes {
		maxReplica, err := es.nodeMaxAvailableReplica(node, request)
		if err != nil {
			klog.Errorf("Error: %v", err)
			continue
		}
		klog.V(4).Infof("Node(%s) max available replica: %d", node.Name, maxReplica)
		maxReplicas += maxReplica
	}
	return maxReplicas
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
		klog.Infof("Finish calculating cluster available replicas of resource(%s), max replicas: %d, time elapsed: %s", object, (*response).MaxReplicas, time.Since(start))
	}
}
