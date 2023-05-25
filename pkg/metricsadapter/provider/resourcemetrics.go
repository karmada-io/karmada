package provider

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// labelSelectorAnnotationInternal is the annotation used internal in karmada-metrics-adapter,
	// to record the selector specified by the user
	labelSelectorAnnotationInternal = "internal.karmada.io/selector"
)

var (
	// podMetricsGVR is the gvr of pod metrics(v1beta1 version)
	podMetricsGVR = metricsv1beta1.SchemeGroupVersion.WithResource("pods")
	// nodeMetricsGVR is the gvr of node metrics(v1beta1 version)
	nodeMetricsGVR = metricsv1beta1.SchemeGroupVersion.WithResource("nodes")
	// PodsGVR is the gvr of pods
	PodsGVR = corev1.SchemeGroupVersion.WithResource("pods")
	// NodesGVR is the gvr of nodes
	NodesGVR = corev1.SchemeGroupVersion.WithResource("nodes")
)

type queryResourceFromClustersFunc func(sci genericmanager.SingleClusterInformerManager, clusterName string) error
type queryMetricsFromClustersFunc func(sci genericmanager.SingleClusterInformerManager, clusterName string) (interface{}, error)

// ResourceMetricsProvider is a resource metrics provider, to provide cpu/memory metrics
type ResourceMetricsProvider struct {
	PodLister  *PodLister
	NodeLister *NodeLister

	clusterLister   clusterlister.ClusterLister
	informerManager genericmanager.MultiClusterInformerManager
}

// NewResourceMetricsProvider creates a new resource metrics provider
func NewResourceMetricsProvider(clusterLister clusterlister.ClusterLister, informerManager genericmanager.MultiClusterInformerManager) *ResourceMetricsProvider {
	return &ResourceMetricsProvider{
		clusterLister:   clusterLister,
		informerManager: informerManager,
		PodLister:       NewPodLister(),
		NodeLister:      NewNodeLister(),
	}
}

// getMetricsParallel is a parallel func of to query metrics from member clusters
func (r *ResourceMetricsProvider) getMetricsParallel(resourceFunc queryResourceFromClustersFunc,
	metricsFunc queryMetricsFromClustersFunc) ([]interface{}, error) {
	clusters, err := r.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return nil, err
	}

	// step 1. Find out the target clusters with lister cache
	var targetClusters []string
	for _, cluster := range clusters {
		sci := r.informerManager.GetSingleClusterManager(cluster.Name)
		if sci == nil {
			klog.Errorf("Failed to get cluster(%s) manager", cluster.Name)
			continue
		}
		err := resourceFunc(sci, cluster.Name)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("Failed to query resource in cluster(%s): %v", cluster.Name, err)
			}
			continue
		}
		targetClusters = append(targetClusters, cluster.Name)
	}

	var metrics []interface{}
	if len(targetClusters) == 0 {
		return metrics, nil
	}

	// step 2. Query metrics from the filtered target clusters
	metricsChanel := make(chan interface{})

	var wg sync.WaitGroup
	for _, clusterName := range targetClusters {
		wg.Add(1)

		go func(cluster string) {
			defer wg.Done()

			sci := r.informerManager.GetSingleClusterManager(cluster)
			if sci == nil {
				klog.Errorf("Failed to get cluster(%s) manager", cluster)
				return
			}

			metrics, err := metricsFunc(sci, cluster)
			if err != nil {
				if !errors.IsNotFound(err) {
					klog.Errorf("Failed to query metrics in cluster(%s): %v", cluster, err)
				}
				return
			}

			// If there are multiple metrics with same name, it's ok because it's an array instead of a map.
			// The HPA controller will calculate the average utilization with the array.
			metricsChanel <- metrics
		}(clusterName)
	}

	go func() {
		wg.Wait()
		close(metricsChanel)
	}()

	for {
		data, ok := <-metricsChanel
		if !ok {
			break
		}
		metrics = append(metrics, data)
	}

	return metrics, nil
}

// queryPodMetricsByName queries metrics by pod name from target clusters
func (r *ResourceMetricsProvider) queryPodMetricsByName(name, namespace string) ([]metrics.PodMetrics, error) {
	resourceQueryFunc := func(sci genericmanager.SingleClusterInformerManager, _ string) error {
		_, err := sci.Lister(PodsGVR).ByNamespace(namespace).Get(name)
		return err
	}
	metricsQueryFunc := func(sci genericmanager.SingleClusterInformerManager, _ string) (interface{}, error) {
		metrics, err := sci.GetClient().Resource(podMetricsGVR).
			Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		return metrics, err
	}

	metricsQuery, err := r.getMetricsParallel(resourceQueryFunc, metricsQueryFunc)
	if err != nil {
		return nil, err
	}

	var podMetrics []metrics.PodMetrics
	for index := range metricsQuery {
		internalMetrics, err := metricsConvertV1beta1PodToInternalPod(*metricsQuery[index].(*unstructured.Unstructured))
		if err != nil {
			continue
		}
		podMetrics = append(podMetrics, internalMetrics...)
	}

	return podMetrics, nil
}

// queryPodMetricsBySelector queries metrics by pod selector from target clusters
func (r *ResourceMetricsProvider) queryPodMetricsBySelector(selector, namespace string) ([]metrics.PodMetrics, error) {
	labelSelector, err := labels.Parse(selector)
	if err != nil {
		klog.Errorf("Failed to parse label selector: %v", err)
		return nil, err
	}

	resourceQueryFunc := func(sci genericmanager.SingleClusterInformerManager, clusterName string) error {
		pods, err := sci.Lister(PodsGVR).ByNamespace(namespace).List(labelSelector)
		if err != nil {
			klog.Errorf("Failed to list pods in cluster(%s): %v", clusterName, err)
			return err
		}
		if len(pods) == 0 {
			return errors.NewNotFound(PodsGVR.GroupResource(), "")
		}
		return nil
	}
	metricsQueryFunc := func(sci genericmanager.SingleClusterInformerManager, _ string) (interface{}, error) {
		metrics, err := sci.GetClient().Resource(podMetricsGVR).
			Namespace(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: selector,
		})
		return metrics, err
	}

	metricsQuery, err := r.getMetricsParallel(resourceQueryFunc, metricsQueryFunc)
	if err != nil {
		return nil, err
	}

	var podMetrics []metrics.PodMetrics
	for index := range metricsQuery {
		metricsData := metricsQuery[index].(*unstructured.UnstructuredList)
		internalMetrics, err := metricsConvertV1beta1PodToInternalPod(metricsData.Items...)
		if err != nil {
			continue
		}
		podMetrics = append(podMetrics, internalMetrics...)
	}

	return podMetrics, nil
}

// queryNodeMetricsByName queries metrics by node name from target clusters
func (r *ResourceMetricsProvider) queryNodeMetricsByName(name string) ([]metrics.NodeMetrics, error) {
	resourceQueryFunc := func(sci genericmanager.SingleClusterInformerManager, _ string) error {
		_, err := sci.Lister(NodesGVR).Get(name)
		return err
	}
	metricsQueryFunc := func(sci genericmanager.SingleClusterInformerManager, _ string) (interface{}, error) {
		metrics, err := sci.GetClient().Resource(nodeMetricsGVR).Get(context.Background(), name, metav1.GetOptions{})
		return metrics, err
	}

	metricsQuery, err := r.getMetricsParallel(resourceQueryFunc, metricsQueryFunc)
	if err != nil {
		return nil, err
	}

	var nodeMetrics []metrics.NodeMetrics
	for index := range metricsQuery {
		internalMetrics, err := metricsConvertV1beta1NodeToInternalNode(*metricsQuery[index].(*unstructured.Unstructured))
		if err != nil {
			continue
		}
		nodeMetrics = append(nodeMetrics, internalMetrics...)
	}

	return nodeMetrics, nil
}

// queryNodeMetricsBySelector queries metrics by node selector from target clusters
func (r *ResourceMetricsProvider) queryNodeMetricsBySelector(selector string) ([]metrics.NodeMetrics, error) {
	labelSelector, err := labels.Parse(selector)
	if err != nil {
		klog.Errorf("Failed to parse label selector: %v", err)
		return nil, err
	}

	resourceQueryFunc := func(sci genericmanager.SingleClusterInformerManager, clusterName string) error {
		nodes, err := sci.Lister(NodesGVR).List(labelSelector)
		if err != nil {
			klog.Errorf("Failed to list pods in cluster(%s): %v", clusterName, err)
			return err
		}
		if len(nodes) == 0 {
			return errors.NewNotFound(PodsGVR.GroupResource(), "")
		}
		return nil
	}
	metricsQueryFunc := func(sci genericmanager.SingleClusterInformerManager, _ string) (interface{}, error) {
		metrics, err := sci.GetClient().Resource(nodeMetricsGVR).List(context.Background(), metav1.ListOptions{
			LabelSelector: selector,
		})
		return metrics, err
	}

	metricsQuery, err := r.getMetricsParallel(resourceQueryFunc, metricsQueryFunc)
	if err != nil {
		return nil, err
	}

	var nodeMetrics []metrics.NodeMetrics
	for index := range metricsQuery {
		metricsData := metricsQuery[index].(*unstructured.UnstructuredList)
		internalMetrics, err := metricsConvertV1beta1NodeToInternalNode(metricsData.Items...)
		if err != nil {
			continue
		}
		nodeMetrics = append(nodeMetrics, internalMetrics...)
	}

	return nodeMetrics, nil
}

// GetPodMetrics queries metrics by the internal constructed pod
func (r *ResourceMetricsProvider) GetPodMetrics(pods ...*metav1.PartialObjectMetadata) ([]metrics.PodMetrics, error) {
	var podMetrics []metrics.PodMetrics

	// In the previous step, we construct pods list with only one element.
	if len(pods) != 1 {
		return podMetrics, nil
	}

	// In the previous step, if query with label selector, the name will be set to empty
	if pods[0].Name == "" {
		namespace := pods[0].Namespace
		selectorStr := pods[0].Annotations[labelSelectorAnnotationInternal]
		return r.queryPodMetricsBySelector(selectorStr, namespace)
	}

	return r.queryPodMetricsByName(pods[0].Name, pods[0].Namespace)
}

// GetNodeMetrics queries metrics by the internal constructed node
func (r *ResourceMetricsProvider) GetNodeMetrics(nodes ...*corev1.Node) ([]metrics.NodeMetrics, error) {
	var nodeMetrics []metrics.NodeMetrics

	// In the previous step, we construct node list with only one element, this should never happen
	if len(nodes) != 1 {
		// never reach here
		return nodeMetrics, nil
	}

	// In the previous step, if query with label selector, the name will be set to empty
	if nodes[0].Name == "" {
		selectorStr := nodes[0].Annotations[labelSelectorAnnotationInternal]
		return r.queryNodeMetricsBySelector(selectorStr)
	}

	return r.queryNodeMetricsByName(nodes[0].Name)
}

// PodLister is an internal lister for pods
type PodLister struct {
	namespaceSpecified string
}

// NewPodLister creates an internal new PodLister
func NewPodLister() *PodLister {
	return &PodLister{}
}

// List returns the internal constructed pod with label selector info
func (p *PodLister) List(selector labels.Selector) (ret []runtime.Object, err error) {
	klog.V(4).Infof("List query pods with selector: %v", selector.String())

	podData := &v1.PartialObjectMetadata{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Namespace: p.namespaceSpecified,
			Annotations: map[string]string{
				labelSelectorAnnotationInternal: selector.String(),
			},
		},
	}

	return []runtime.Object{podData}, nil
}

// Get returns the internal constructed pod with name info
func (p *PodLister) Get(name string) (runtime.Object, error) {
	klog.V(4).Infof("Query pod in namespace(%s) with name:%s", p.namespaceSpecified, name)

	podData := &v1.PartialObjectMetadata{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: p.namespaceSpecified,
		},
	}
	return podData, nil
}

// ByNamespace returns the pod lister with namespace info
func (p *PodLister) ByNamespace(namespace string) cache.GenericNamespaceLister {
	klog.V(4).Infof("Query Pods in namespace: %s", namespace)

	listerCopy := &PodLister{}
	listerCopy.namespaceSpecified = namespace
	return listerCopy
}

// NodeLister is an internal lister for nodes
type NodeLister struct {
}

// NewNodeLister creates an internal new NodeLister
func NewNodeLister() *NodeLister {
	return &NodeLister{}
}

// List returns the internal constructed node with label selector info
func (n *NodeLister) List(selector labels.Selector) (ret []*corev1.Node, err error) {
	klog.V(4).Infof("Query node metrics with selector: %s", selector.String())

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				labelSelectorAnnotationInternal: selector.String(),
			},
		},
	}
	return []*corev1.Node{node}, nil
}

// Get returns the internal constructed node with name info
func (n *NodeLister) Get(name string) (*corev1.Node, error) {
	klog.V(4).Infof("Query node metrics with name:%s", name)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return node, nil
}

// metricsConvertV1beta1PodToInternalPod converts metricsv1beta1.PodMetrics to metrics.PodMetrics
func metricsConvertV1beta1PodToInternalPod(objs ...unstructured.Unstructured) ([]metrics.PodMetrics, error) {
	var podMetricsV1beta1 []metricsv1beta1.PodMetrics

	for index := range objs {
		single := metricsv1beta1.PodMetrics{}
		if err := helper.ConvertToTypedObject(&objs[index], &single); err != nil {
			klog.Errorf("Failed to convert to typed object: %v", err)
			return nil, err
		}
		podMetricsV1beta1 = append(podMetricsV1beta1, single)
	}

	var podMetricsInternal []metrics.PodMetrics
	for index := range podMetricsV1beta1 {
		single := metrics.PodMetrics{}
		if err := metricsv1beta1.Convert_v1beta1_PodMetrics_To_metrics_PodMetrics(&podMetricsV1beta1[index], &single, nil); err != nil {
			klog.Errorf("Failed to convert to typed object: %v", err)
			return nil, err
		}

		podMetricsInternal = append(podMetricsInternal, single)
	}

	return podMetricsInternal, nil
}

// metricsConvertV1beta1NodeToInternalNode converts metricsv1beta1.NodeMetrics to metrics.NodeMetrics
func metricsConvertV1beta1NodeToInternalNode(objs ...unstructured.Unstructured) ([]metrics.NodeMetrics, error) {
	var nodeMetricsV1beta1 []metricsv1beta1.NodeMetrics

	for index := range objs {
		single := metricsv1beta1.NodeMetrics{}
		if err := helper.ConvertToTypedObject(&objs[index], &single); err != nil {
			klog.Errorf("Failed to convert to typed object: %v", err)
			return nil, err
		}
		nodeMetricsV1beta1 = append(nodeMetricsV1beta1, single)
	}

	var nodeMetricsInternal []metrics.NodeMetrics
	for index := range nodeMetricsV1beta1 {
		single := metrics.NodeMetrics{}
		if err := metricsv1beta1.Convert_v1beta1_NodeMetrics_To_metrics_NodeMetrics(&nodeMetricsV1beta1[index], &single, nil); err != nil {
			klog.Errorf("Failed to convert to typed object: %v", err)
			return nil, err
		}

		nodeMetricsInternal = append(nodeMetricsInternal, single)
	}

	return nodeMetricsInternal, nil
}
