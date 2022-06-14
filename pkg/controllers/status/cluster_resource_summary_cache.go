package status

import (
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
)

type nodeSummary struct {
	isReady         bool
	nodeAllocatable corev1.ResourceList
}

type podSummary struct {
	isAllocating bool
	isAllocated  bool
	podResource  *util.Resource
}

type singleClusterSummaryCache struct {
	nodeSummaryMap map[keys.ClusterWideKey]nodeSummary
	podSummaryMap  map[keys.ClusterWideKey]podSummary
}

func newSingleClusterSummaryCache() *singleClusterSummaryCache {
	return &singleClusterSummaryCache{
		nodeSummaryMap: make(map[keys.ClusterWideKey]nodeSummary),
		podSummaryMap:  make(map[keys.ClusterWideKey]podSummary),
	}
}

func (s *singleClusterSummaryCache) set(clusterWideKey keys.ClusterWideKey, object runtime.Object) error {
	switch clusterWideKey.Kind {
	case util.NodeKind:
		node, err := helper.ConvertToNode(object.(*unstructured.Unstructured))
		if err != nil {
			return fmt.Errorf("failed to convert unstructured to typed object: %v", err)
		}

		s.nodeSummaryMap[clusterWideKey] = nodeSummary{isReady: helper.NodeReady(node), nodeAllocatable: node.Status.Allocatable}
	case util.PodKind:
		pod, err := helper.ConvertToPod(object.(*unstructured.Unstructured))
		if err != nil {
			return fmt.Errorf("failed to convert unstructured to typed object: %v", err)
		}
		s.podSummaryMap[clusterWideKey] = podSummary{isAllocating: isPodAllocating(pod), isAllocated: isPodAllocated(pod), podResource: getPodResource(pod)}
	}

	return nil
}

func (s *singleClusterSummaryCache) delete(clusterWideKey keys.ClusterWideKey) {
	switch clusterWideKey.Kind {
	case util.NodeKind:
		delete(s.nodeSummaryMap, clusterWideKey)
	case util.PodKind:
		delete(s.podSummaryMap, clusterWideKey)
	}
}

// ClusterSummaryCache stores the summary data of nodes and pods in member clusters.
type ClusterSummaryCache struct {
	sync.RWMutex
	clusterSummary map[string]*singleClusterSummaryCache
}

// NewClusterSummaryCache returns a ClusterSummaryCache for the member clusters.
func NewClusterSummaryCache() *ClusterSummaryCache {
	return &ClusterSummaryCache{
		clusterSummary: make(map[string]*singleClusterSummaryCache),
	}
}

func (n *ClusterSummaryCache) set(fedKey keys.FederatedKey, object runtime.Object) error {
	n.Lock()
	defer n.Unlock()

	if _, ok := n.clusterSummary[fedKey.Cluster]; !ok {
		n.clusterSummary[fedKey.Cluster] = newSingleClusterSummaryCache()
	}
	if err := n.clusterSummary[fedKey.Cluster].set(fedKey.ClusterWideKey, object); err != nil {
		return err
	}
	return nil
}

func (n *ClusterSummaryCache) delete(fedKey keys.FederatedKey) {
	n.Lock()
	defer n.Unlock()

	if _, ok := n.clusterSummary[fedKey.Cluster]; !ok {
		return
	}
	n.clusterSummary[fedKey.Cluster].delete(fedKey.ClusterWideKey)
}

func (n *ClusterSummaryCache) deleteCluster(clusterName string) {
	n.Lock()
	defer n.Unlock()

	delete(n.clusterSummary, clusterName)
}

func (n *ClusterSummaryCache) getNodeSummary(clusterName string) (*clusterv1alpha1.NodeSummary, corev1.ResourceList) {
	n.Lock()
	defer n.Unlock()

	if _, ok := n.clusterSummary[clusterName]; !ok {
		return nil, nil
	}

	totalNum := len(n.clusterSummary[clusterName].nodeSummaryMap)
	readyNum := 0
	allocatable := make(corev1.ResourceList)

	for _, node := range n.clusterSummary[clusterName].nodeSummaryMap {
		if node.isReady {
			readyNum++
		}
		for key, val := range node.nodeAllocatable {
			tmpCap, ok := allocatable[key]
			if ok {
				tmpCap.Add(val)
			} else {
				tmpCap = val
			}
			allocatable[key] = tmpCap
		}
	}

	ns := clusterv1alpha1.NodeSummary{
		TotalNum: int32(totalNum),
		ReadyNum: int32(readyNum),
	}
	return &ns, allocatable
}

func (n *ClusterSummaryCache) getPodSummary(clusterName string) (corev1.ResourceList, corev1.ResourceList) {
	n.Lock()
	defer n.Unlock()

	if _, ok := n.clusterSummary[clusterName]; !ok {
		return nil, nil
	}

	allocatedPodNum := int64(0)
	allocatingPodNum := int64(0)
	allocating := util.EmptyResource()
	allocated := util.EmptyResource()

	for _, pod := range n.clusterSummary[clusterName].podSummaryMap {
		if pod.isAllocating {
			allocating.AddResource(pod.podResource)
			allocatingPodNum++
		}
		if pod.isAllocated {
			allocated.AddResource(pod.podResource)
			allocatedPodNum++
		}
	}

	allocating.AddResourcePods(allocatingPodNum)
	allocated.AddResourcePods(allocatedPodNum)
	return allocating.ResourceList(), allocated.ResourceList()
}

func getPodResource(pod *corev1.Pod) *util.Resource {
	resource := util.EmptyResource()
	resource.AddPodRequest(&pod.Spec)
	return resource
}

func isPodAllocating(pod *corev1.Pod) bool {
	return len(pod.Spec.NodeName) == 0
}

func isPodAllocated(pod *corev1.Pod) bool {
	if len(pod.Spec.NodeName) != 0 && pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
		return true
	}
	return false
}
