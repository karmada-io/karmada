package provider

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// labelSelectorAnnotationInternal is the annotation used internal in karmada-metrics-adapter,
	// to record the selector specified by the user
	labelSelectorAnnotationInternal = "internal.karmada.io/selector"
	// namespaceSpecifiedAnnotation is the annotation used in karmada-metrics-adapter,
	// to record the namespace specified by the user
	namespaceSpecifiedAnnotation = "internal.karmada.io/namespace"
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
		PodLister:       NewPodLister(clusterLister, informerManager),
		NodeLister:      NewNodeLister(clusterLister, informerManager),
	}
}

// getMetricsParallel is a parallel func to query metrics from member clusters
func (r *ResourceMetricsProvider) getMetricsParallel(resourceFunc queryResourceFromClustersFunc,
	metricsFunc queryMetricsFromClustersFunc) ([]interface{}, error) {
	clusters, err := r.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return nil, err
	}

	// step 1. Find out the target clusters in lister cache
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
	metricsQueryFunc := func(sci genericmanager.SingleClusterInformerManager, clusterName string) (interface{}, error) {
		metricsValue, err := sci.GetClient().Resource(podMetricsGVR).
			Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		util.MergeAnnotation(metricsValue, autoscalingv1alpha1.QuerySourceAnnotationKey, clusterName)
		return metricsValue, err
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
	metricsQueryFunc := func(sci genericmanager.SingleClusterInformerManager, clusterName string) (interface{}, error) {
		metricsList, err := sci.GetClient().Resource(podMetricsGVR).
			Namespace(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			return nil, err
		}
		for i := range metricsList.Items {
			util.MergeAnnotation(&metricsList.Items[i], autoscalingv1alpha1.QuerySourceAnnotationKey, clusterName)
		}
		return metricsList, err
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
	metricsQueryFunc := func(sci genericmanager.SingleClusterInformerManager, clusterName string) (interface{}, error) {
		metricsValue, err := sci.GetClient().Resource(nodeMetricsGVR).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		util.MergeAnnotation(metricsValue, autoscalingv1alpha1.QuerySourceAnnotationKey, clusterName)
		return metricsValue, err
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
	metricsQueryFunc := func(sci genericmanager.SingleClusterInformerManager, clusterName string) (interface{}, error) {
		metricsList, err := sci.GetClient().Resource(nodeMetricsGVR).List(context.Background(), metav1.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			return nil, err
		}
		for i := range metricsList.Items {
			util.MergeAnnotation(&metricsList.Items[i], autoscalingv1alpha1.QuerySourceAnnotationKey, clusterName)
		}
		return metricsList, err
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
	var queryData []metrics.PodMetrics
	if len(pods) == 0 {
		return queryData, nil
	}
	var err error
	// In the previous step, we construct the annotations, so it couldn't be nil
	if _, ok := pods[0].Annotations[labelSelectorAnnotationInternal]; ok {
		namespace := pods[0].Annotations[namespaceSpecifiedAnnotation]
		selectorStr := pods[0].Annotations[labelSelectorAnnotationInternal]
		queryData, err = r.queryPodMetricsBySelector(selectorStr, namespace)
	} else {
		queryData, err = r.queryPodMetricsByName(pods[0].Name, pods[0].Namespace)
	}
	if err != nil {
		return nil, err
	}
	return queryData, nil
}

// GetNodeMetrics queries metrics by the internal constructed node
func (r *ResourceMetricsProvider) GetNodeMetrics(nodes ...*corev1.Node) ([]metrics.NodeMetrics, error) {
	var queryData []metrics.NodeMetrics
	if len(nodes) == 0 {
		return queryData, nil
	}
	var err error
	// In the previous step, we construct the annotations, so it couldn't be nil
	if _, ok := nodes[0].Annotations[labelSelectorAnnotationInternal]; ok {
		selectorStr := nodes[0].Annotations[labelSelectorAnnotationInternal]
		queryData, err = r.queryNodeMetricsBySelector(selectorStr)
	} else {
		queryData, err = r.queryNodeMetricsByName(nodes[0].Name)
	}
	if err != nil {
		return nil, err
	}
	return queryData, nil
}

// PodLister is an internal lister for pods
type PodLister struct {
	namespaceSpecified string
	clusterLister      clusterlister.ClusterLister
	informerManager    genericmanager.MultiClusterInformerManager
}

// NewPodLister creates an internal new PodLister
func NewPodLister(clusterLister clusterlister.ClusterLister, informerManager genericmanager.MultiClusterInformerManager) *PodLister {
	return &PodLister{
		clusterLister:   clusterLister,
		informerManager: informerManager,
	}
}

// List returns the internal constructed pod with label selector info
func (p *PodLister) List(selector labels.Selector) (ret []runtime.Object, err error) {
	klog.V(4).Infof("List query pods with selector: %s", selector.String())

	clusters, err := p.clusterLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, cluster := range clusters {
		sci := p.informerManager.GetSingleClusterManager(cluster.Name)
		if sci == nil {
			klog.Errorf("Failed to get SingleClusterInformerManager for cluster(%s)", cluster.Name)
			continue
		}
		pods, err := sci.Lister(PodsGVR).ByNamespace(p.namespaceSpecified).List(selector)
		if err != nil {
			klog.Errorf("Failed to list pods from cluster(%s) in namespace(%s): %v", cluster.Name, p.namespaceSpecified, err)
			return nil, err
		}
		for _, pod := range pods {
			podTyped := &corev1.Pod{}
			err = helper.ConvertToTypedObject(pod, podTyped)
			if err != nil {
				klog.Errorf("Failed to convert to typed object: %v", err)
				return nil, err
			}
			podPartial := p.convertToPodPartialData(podTyped, selector.String(), true)
			ret = append(ret, podPartial)
		}
	}

	return ret, nil
}

// convertToPodPartialData converts pod to partial data
func (p *PodLister) convertToPodPartialData(pod *corev1.Pod, selector string, labelSelector bool) *metav1.PartialObjectMetadata {
	ret := &metav1.PartialObjectMetadata{
		TypeMeta:   pod.TypeMeta,
		ObjectMeta: pod.ObjectMeta,
	}
	if ret.Annotations == nil {
		ret.Annotations = map[string]string{}
	}

	//If user sets this annotation, we need to remove it to avoid parsing wrong next.
	if !labelSelector {
		delete(ret.Annotations, namespaceSpecifiedAnnotation)
		delete(ret.Annotations, labelSelectorAnnotationInternal)
		return ret
	}
	ret.Annotations[labelSelectorAnnotationInternal] = selector
	ret.Annotations[namespaceSpecifiedAnnotation] = p.namespaceSpecified

	return ret
}

// Get returns the internal constructed pod with name info
func (p *PodLister) Get(name string) (runtime.Object, error) {
	klog.V(4).Infof("Query pod in namespace(%s) with name:%s", p.namespaceSpecified, name)

	clusters, err := p.clusterLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var podPartial *metav1.PartialObjectMetadata
	for _, cluster := range clusters {
		sci := p.informerManager.GetSingleClusterManager(cluster.Name)
		if sci == nil {
			klog.Errorf("Failed to get SingleClusterInformerManager for cluster(%s)", cluster.Name)
			continue
		}
		pod, err := sci.Lister(PodsGVR).ByNamespace(p.namespaceSpecified).Get(name)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("Failed to get pod  from clsuster(%s) in namespace(%s): %v", cluster.Name, p.namespaceSpecified, err)
			}
			continue
		}

		if podPartial != nil {
			err := fmt.Errorf("the pod(%s) found in more than one clusters", name)
			return nil, errors.NewConflict(PodsGVR.GroupResource(), name, err)
		}
		podTyped := &corev1.Pod{}
		err = helper.ConvertToTypedObject(pod, podTyped)
		if err != nil {
			klog.Errorf("Failed to convert to typed object: %v", err)
			return nil, err
		}
		podPartial = p.convertToPodPartialData(podTyped, "", false)
	}

	if podPartial != nil {
		return podPartial, nil
	}

	return nil, errors.NewNotFound(PodsGVR.GroupResource(), name)
}

// ByNamespace returns the pod lister with namespace info
func (p *PodLister) ByNamespace(namespace string) cache.GenericNamespaceLister {
	klog.V(4).Infof("Query Pods in namespace: %s", namespace)

	listerCopy := &PodLister{
		clusterLister:   p.clusterLister,
		informerManager: p.informerManager,
	}
	listerCopy.namespaceSpecified = namespace
	return listerCopy
}

// NodeLister is an internal lister for nodes
type NodeLister struct {
	clusterLister   clusterlister.ClusterLister
	informerManager genericmanager.MultiClusterInformerManager
}

// NewNodeLister creates an internal new NodeLister
func NewNodeLister(clusterLister clusterlister.ClusterLister, informerManager genericmanager.MultiClusterInformerManager) *NodeLister {
	return &NodeLister{
		clusterLister:   clusterLister,
		informerManager: informerManager,
	}
}

// List returns the internal constructed node with label selector info
func (n *NodeLister) List(selector labels.Selector) (ret []*corev1.Node, err error) {
	klog.V(4).Infof("Query node metrics with selector: %s", selector.String())

	clusters, err := n.clusterLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, cluster := range clusters {
		sci := n.informerManager.GetSingleClusterManager(cluster.Name)
		if sci == nil {
			klog.Errorf("Failed to get SingleClusterInformerManager for cluster(%s)", cluster.Name)
			continue
		}
		nodes, err := sci.Lister(NodesGVR).List(selector)
		if err != nil {
			klog.Errorf("Failed to list nodes from cluster(%s): %v", cluster.Name, err)
			return nil, err
		}
		for index := range nodes {
			nodeTyped := &corev1.Node{}
			err = helper.ConvertToTypedObject(nodes[index], nodeTyped)
			if err != nil {
				klog.Errorf("Failed to convert to typed object: %v", err)
				return nil, err
			}
			if nodeTyped.Annotations == nil {
				nodeTyped.Annotations = map[string]string{}
			}

			//If user sets this annotation, we need to reset it.
			nodeTyped.Annotations[labelSelectorAnnotationInternal] = selector.String()
			ret = append(ret, nodeTyped)
		}
	}

	return ret, nil
}

// Get returns the internal constructed node with name info
func (n *NodeLister) Get(name string) (*corev1.Node, error) {
	klog.V(4).Infof("Query node metrics with name:%s", name)

	clusters, err := n.clusterLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var nodeTyped *corev1.Node
	for _, cluster := range clusters {
		sci := n.informerManager.GetSingleClusterManager(cluster.Name)
		if sci == nil {
			klog.Errorf("Failed to get SingleClusterInformerManager for cluster(%s)", cluster.Name)
			continue
		}
		node, err := sci.Lister(NodesGVR).Get(name)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("Failed to get node from cluster(%s):%v", cluster.Name, err)
			}
			continue
		}

		if nodeTyped != nil {
			err := fmt.Errorf("the node(%s) found in more than one clusters", name)
			return nil, errors.NewConflict(NodesGVR.GroupResource(), name, err)
		}

		nodeTyped = &corev1.Node{}
		err = helper.ConvertToTypedObject(node, nodeTyped)
		if err != nil {
			klog.Errorf("Failed to convert to typed object: %v", err)
			return nil, err
		}
		if nodeTyped.Annotations == nil {
			nodeTyped.Annotations = map[string]string{}
		}

		//If user sets this annotation, we need to remove it to avoid parsing wrong next.
		delete(nodeTyped.Annotations, labelSelectorAnnotationInternal)
	}

	if nodeTyped != nil {
		return nodeTyped, nil
	}

	return nil, errors.NewNotFound(NodesGVR.GroupResource(), name)
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
