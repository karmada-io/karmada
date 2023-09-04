package modeling

import (
	"container/list"
	"errors"
	"math"
	"sync"

	rbt "github.com/emirpasic/gods/trees/redblacktree"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

var (
	mu                  sync.Mutex
	modelSortings       [][]resource.Quantity
	defaultModelSorting []clusterapis.ResourceName
)

// ResourceSummary records the list of resourceModels
type ResourceSummary []resourceModels

// resourceModels records the number of each allocatable resource models.
type resourceModels struct {
	// quantity is the total number of each allocatable resource models
	// +required
	Quantity int

	// when the number of node is less than or equal to six, it will be sorted by linkedlist,
	// when the number of node is more than six, it will be sorted by red-black tree.

	// when the data structure is linkedlist,
	// each item will store clusterResourceNode.
	// +required
	linkedlist *list.List

	// when the data structure is redblack tree,
	// each item will store a key-value pair,
	// key is ResourceList, the value is quantity of this ResourceList
	// +optional
	redblackTree *rbt.Tree
}

// ClusterResourceNode represents the each raw resource entity without modeling.
type ClusterResourceNode struct {
	// quantity is the the number of this node
	// Only when the resourceLists are exactly the same can they be counted as the same node.
	// +required
	quantity int

	// resourceList records the resource list of this node.
	// It maybe contain cpu, memory, gpu...
	// User can specify which parameters need to be included before the cluster starts
	// +required
	resourceList ResourceList
}

// ResourceList is a set of (resource name, quantity) pairs.
type ResourceList map[clusterapis.ResourceName]resource.Quantity

// InitSummary is the init function of modeling data structure
func InitSummary(resourceModels []clusterapis.ResourceModel) (ResourceSummary, error) {
	var rsName []clusterapis.ResourceName
	var rsList []ResourceList
	for _, rm := range resourceModels {
		tmp := map[clusterapis.ResourceName]resource.Quantity{}
		for _, rmItem := range rm.Ranges {
			if len(rsName) != len(rm.Ranges) {
				rsName = append(rsName, rmItem.Name)
			}
			tmp[rmItem.Name] = rmItem.Min
		}
		rsList = append(rsList, tmp)
	}

	if len(rsName) != 0 && len(rsList) != 0 && (len(rsName) != len(rsList[0])) {
		return nil, errors.New("the number of resourceName is not equal the number of resourceList")
	}
	var rs ResourceSummary
	if len(rsName) != 0 {
		defaultModelSorting = rsName
	}
	rs = make(ResourceSummary, len(rsList))
	// generate a sorted array by first priority of ResourceName
	modelSortings = make([][]resource.Quantity, len(rsName))
	for index := 0; index < len(rsList); index++ {
		for i, name := range rsName {
			modelSortings[i] = append(modelSortings[i], rsList[index][name])
		}
	}
	return rs, nil
}

// NewClusterResourceNode create new cluster resource node
func NewClusterResourceNode(resourceList corev1.ResourceList) ClusterResourceNode {
	return ClusterResourceNode{
		quantity:     1,
		resourceList: ConvertToResourceList(resourceList),
	}
}

func (rs *ResourceSummary) getIndex(crn ClusterResourceNode) int {
	index := math.MaxInt
	for i, m := range defaultModelSorting {
		tmpIndex := searchLastLessElement(modelSortings[i], crn.resourceList[m])
		if tmpIndex < index {
			index = tmpIndex
		}
	}
	return index
}

func searchLastLessElement(nums []resource.Quantity, target resource.Quantity) int {
	low, high := 0, len(nums)-1
	for low <= high {
		mid := low + ((high - low) >> 1)
		diff1 := nums[mid].Cmp(target)
		var diff2 int
		if mid != len(nums)-1 {
			diff2 = nums[mid+1].Cmp(target)
		}
		// diff < 1 means nums[mid] <= target
		// diff == 1 means nums[mid+1] > target
		if diff1 < 1 {
			if (mid == len(nums)-1) || (diff2 == 1) {
				// find the last less element that equal to target element
				return mid
			}
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return -1
}

// clusterResourceNodeComparator provides a fast comparison on clusterResourceNodes
func clusterResourceNodeComparator(a, b interface{}) int {
	s1 := a.(ClusterResourceNode)
	s2 := b.(ClusterResourceNode)
	for index := 0; index < len(defaultModelSorting); index++ {
		tmp1, tmp2 := s1.resourceList[defaultModelSorting[index]], s2.resourceList[defaultModelSorting[index]]

		diff := tmp1.Cmp(tmp2)
		if diff != 0 {
			return diff
		}
	}
	return 0
}

func safeChangeNum(num *int, change int) {
	mu.Lock()
	defer mu.Unlock()

	*num += change
}

// AddToResourceSummary add resource node into modeling summary
func (rs *ResourceSummary) AddToResourceSummary(crn ClusterResourceNode) {
	index := rs.getIndex(crn)
	if index == -1 {
		klog.Error("ClusterResource can not add to resource summary: index is invalid.")
		return
	}
	modeling := &(*rs)[index]
	if rs.GetNodeNumFromModel(modeling) <= 5 {
		root := modeling.linkedlist
		if root == nil {
			root = list.New()
		}
		found := false
		// traverse linkedlist to add quantity of recourse modeling
		for element := root.Front(); element != nil; element = element.Next() {
			switch clusterResourceNodeComparator(element.Value, crn) {
			case 0:
				{
					tmpCrn := element.Value.(ClusterResourceNode)
					safeChangeNum(&tmpCrn.quantity, crn.quantity)
					element.Value = tmpCrn
					found = true
					break
				}
			case 1:
				{
					root.InsertBefore(crn, element)
					found = true
					break
				}
			case -1:
				{
					continue
				}
			}
			if found {
				break
			}
		}
		if !found {
			root.PushBack(crn)
		}
		modeling.linkedlist = root
	} else {
		root := modeling.redblackTree
		if root == nil {
			root = llConvertToRbt(modeling.linkedlist)
			modeling.linkedlist = nil
		}
		tmpNode := root.GetNode(crn)
		if tmpNode != nil {
			node := tmpNode.Key.(ClusterResourceNode)
			safeChangeNum(&node.quantity, crn.quantity)
			tmpNode.Key = node
		} else {
			root.Put(crn, crn.quantity)
		}
		modeling.redblackTree = root
	}
	safeChangeNum(&modeling.Quantity, crn.quantity)
}

func llConvertToRbt(list *list.List) *rbt.Tree {
	root := rbt.NewWith(clusterResourceNodeComparator)
	for element := list.Front(); element != nil; element = element.Next() {
		tmpCrn := element.Value.(ClusterResourceNode)
		root.Put(tmpCrn, tmpCrn.quantity)
	}
	return root
}

func rbtConvertToLl(rbt *rbt.Tree) *list.List {
	root := list.New()
	for _, v := range rbt.Keys() {
		root.PushBack(v)
	}
	return root
}

// ConvertToResourceList is convert from corev1.ResourceList to ResourceList
func ConvertToResourceList(rsList corev1.ResourceList) ResourceList {
	resourceList := ResourceList{}
	for name, quantity := range rsList {
		if name == corev1.ResourceCPU {
			resourceList[clusterapis.ResourceCPU] = quantity
		} else if name == corev1.ResourceMemory {
			resourceList[clusterapis.ResourceMemory] = quantity
		} else if name == corev1.ResourceStorage {
			resourceList[clusterapis.ResourceStorage] = quantity
		} else if name == corev1.ResourceEphemeralStorage {
			resourceList[clusterapis.ResourceEphemeralStorage] = quantity
		}
	}
	return resourceList
}

// GetNodeNumFromModel is for getting node number from the modeling
func (rs *ResourceSummary) GetNodeNumFromModel(model *resourceModels) int {
	if model.linkedlist != nil && model.redblackTree == nil {
		return model.linkedlist.Len()
	} else if model.linkedlist == nil && model.redblackTree != nil {
		return model.redblackTree.Size()
	} else if model.linkedlist == nil && model.redblackTree == nil {
		return 0
	} else if model.linkedlist != nil && model.redblackTree != nil {
		klog.Info("GetNodeNum: unknown error")
	}
	return 0
}

// DeleteFromResourceSummary deletes resource node into modeling summary
func (rs *ResourceSummary) DeleteFromResourceSummary(crn ClusterResourceNode) error {
	index := rs.getIndex(crn)
	if index == -1 {
		return errors.New("ClusterResource can not delete the resource summary: index is invalid")
	}
	modeling := &(*rs)[index]
	if rs.GetNodeNumFromModel(modeling) >= 6 {
		root := modeling.redblackTree
		tmpNode := root.GetNode(crn)
		if tmpNode != nil {
			node := tmpNode.Key.(ClusterResourceNode)
			safeChangeNum(&node.quantity, -crn.quantity)
			tmpNode.Key = node
			if node.quantity == 0 {
				root.Remove(tmpNode)
			}
		} else {
			return errors.New("delete fail: node no found in redblack tree")
		}
		modeling.redblackTree = root
	} else {
		root, tree := modeling.linkedlist, modeling.redblackTree
		if root == nil && tree != nil {
			root = rbtConvertToLl(tree)
		}
		if root == nil && tree == nil {
			return errors.New("delete fail: node no found in linked list")
		}
		found := false
		// traverse linkedlist to remove quantity of recourse modeling
		for element := root.Front(); element != nil; element = element.Next() {
			if clusterResourceNodeComparator(element.Value, crn) == 0 {
				tmpCrn := element.Value.(ClusterResourceNode)
				safeChangeNum(&tmpCrn.quantity, -crn.quantity)
				element.Value = tmpCrn
				if tmpCrn.quantity == 0 {
					root.Remove(element)
				}
				found = true
			}
			if found {
				break
			}
		}
		if !found {
			return errors.New("delete fail: node no found in linkedlist")
		}
		modeling.linkedlist = root
	}
	safeChangeNum(&modeling.Quantity, -crn.quantity)
	return nil
}

// UpdateInResourceSummary update resource node into modeling summary
func (rs *ResourceSummary) UpdateInResourceSummary(oldNode, newNode ClusterResourceNode) error {
	rs.AddToResourceSummary(newNode)
	err := rs.DeleteFromResourceSummary(oldNode)
	if err != nil {
		return errors.New("delete fail: node no found in linked list")
	}
	return nil
}
