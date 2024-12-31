/*
Copyright 2022 The Karmada Authors.

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

package modeling

import (
	"container/list"
	"errors"
	"math"

	rbt "github.com/emirpasic/gods/trees/redblacktree"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

// ResourceSummary records the list of resourceModels
type ResourceSummary struct {
	RMs                       []resourceModels
	modelSortings             [][]resource.Quantity
	modelSortingResourceNames []corev1.ResourceName
}

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
	resourceList corev1.ResourceList
}

// InitSummary is the init function of modeling data structure
func InitSummary(resourceModel []clusterapis.ResourceModel) (ResourceSummary, error) {
	var rsName []corev1.ResourceName
	var rsList []corev1.ResourceList
	for _, rm := range resourceModel {
		tmp := map[corev1.ResourceName]resource.Quantity{}
		for _, rmItem := range rm.Ranges {
			if len(rsName) != len(rm.Ranges) {
				rsName = append(rsName, rmItem.Name)
			}
			tmp[rmItem.Name] = rmItem.Min
		}
		rsList = append(rsList, tmp)
	}

	if len(rsName) != 0 && len(rsList) != 0 && (len(rsName) != len(rsList[0])) {
		return ResourceSummary{}, errors.New("the number of resourceName is not equal the number of resourceList")
	}

	rms := make([]resourceModels, len(rsList))
	// generate a sorted array by first priority of ResourceName
	modelSortings := make([][]resource.Quantity, len(rsName))
	for index := 0; index < len(rsList); index++ {
		for i, name := range rsName {
			modelSortings[i] = append(modelSortings[i], rsList[index][name])
		}
	}
	return ResourceSummary{RMs: rms, modelSortings: modelSortings, modelSortingResourceNames: rsName}, nil
}

// NewClusterResourceNode create new cluster resource node
func NewClusterResourceNode(resourceList corev1.ResourceList) ClusterResourceNode {
	return ClusterResourceNode{
		quantity:     1,
		resourceList: resourceList,
	}
}

func (rs *ResourceSummary) getIndex(crn ClusterResourceNode) int {
	index := math.MaxInt
	for i, m := range rs.modelSortingResourceNames {
		tmpIndex := searchLastLessElement(rs.modelSortings[i], crn.resourceList[m])
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
func (rs *ResourceSummary) clusterResourceNodeComparator(a, b interface{}) int {
	s1 := a.(ClusterResourceNode)
	s2 := b.(ClusterResourceNode)
	for index := 0; index < len(rs.modelSortingResourceNames); index++ {
		tmp1, tmp2 := s1.resourceList[rs.modelSortingResourceNames[index]], s2.resourceList[rs.modelSortingResourceNames[index]]

		diff := tmp1.Cmp(tmp2)
		if diff != 0 {
			return diff
		}
	}
	return 0
}

// AddToResourceSummary add resource node into modeling summary
func (rs *ResourceSummary) AddToResourceSummary(crn ClusterResourceNode) {
	index := rs.getIndex(crn)
	if index == -1 {
		klog.Errorf("Failed to add node to resource summary due to no appropriate grade. ClusterResourceNode:%v", crn)
		return
	}
	modeling := &(*rs).RMs[index]
	if rs.GetNodeNumFromModel(modeling) <= 5 {
		root := modeling.linkedlist
		if root == nil {
			root = list.New()
		}
		found := false
		// traverse linkedlist to add quantity of recourse modeling
		for element := root.Front(); element != nil; element = element.Next() {
			switch rs.clusterResourceNodeComparator(element.Value, crn) {
			case 0:
				{
					tmpCrn := element.Value.(ClusterResourceNode)
					tmpCrn.quantity += crn.quantity
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
			root = rs.llConvertToRbt(modeling.linkedlist)
			modeling.linkedlist = nil
		}
		tmpNode := root.GetNode(crn)
		if tmpNode != nil {
			node := tmpNode.Key.(ClusterResourceNode)
			node.quantity += crn.quantity
			tmpNode.Key = node
		} else {
			root.Put(crn, crn.quantity)
		}
		modeling.redblackTree = root
	}
	modeling.Quantity += crn.quantity
}

func (rs *ResourceSummary) llConvertToRbt(list *list.List) *rbt.Tree {
	root := rbt.NewWith(rs.clusterResourceNodeComparator)
	for element := list.Front(); element != nil; element = element.Next() {
		tmpCrn := element.Value.(ClusterResourceNode)
		root.Put(tmpCrn, tmpCrn.quantity)
	}
	return root
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
