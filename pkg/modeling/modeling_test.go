package modeling

import (
	"container/list"
	"fmt"
	"reflect"
	"testing"

	rbt "github.com/emirpasic/gods/trees/redblacktree"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

func TestInitSummary(t *testing.T) {
	rms := []clusterapis.ResourceModel{
		{
			Grade: 0,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024, resource.DecimalSI),
				},
			},
		},
		{
			Grade: 1,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
					Max:  *resource.NewQuantity(2, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(1024, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024*2, resource.DecimalSI),
				},
			},
		},
	}

	rs, err := InitSummary(rms)
	if actualValue := len(rs); actualValue != 2 {
		t.Errorf("Got %v expected %v", actualValue, 2)
	}

	if err != nil {
		t.Errorf("Got %v expected %v", err, nil)
	}
}

func TestInitSummaryError(t *testing.T) {
	rms := []clusterapis.ResourceModel{
		{
			Grade: 0,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024, resource.DecimalSI),
				},
			},
		},
		{
			Grade: 1,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
					Max:  *resource.NewQuantity(2, resource.DecimalSI),
				},
			},
		},
	}

	rs, err := InitSummary(rms)
	if actualValue := len(rs); actualValue != 0 {
		t.Errorf("Got %v expected %v", actualValue, 0)
	}

	if err == nil {
		t.Errorf("Got %v expected %v", err, nil)
	}
}

func TestSearchLastLessElement(t *testing.T) {
	nums, target := []resource.Quantity{
		*resource.NewMilliQuantity(1999, resource.DecimalSI),
		*resource.NewMilliQuantity(4327, resource.DecimalSI),
		*resource.NewMilliQuantity(9783, resource.DecimalSI),
		*resource.NewMilliQuantity(12378, resource.DecimalSI),
		*resource.NewMilliQuantity(23480, resource.DecimalSI),
		*resource.NewMilliQuantity(74217, resource.DecimalSI),
	}, *resource.NewMilliQuantity(10000, resource.DecimalSI)
	index := searchLastLessElement(nums, target)
	if index != 2 {
		t.Errorf("Got %v expected %v", index, 2)
	}
	nums, target = []resource.Quantity{
		*resource.NewMilliQuantity(1000, resource.DecimalSI),
		*resource.NewMilliQuantity(3400, resource.DecimalSI),
		*resource.NewMilliQuantity(4500, resource.DecimalSI),
		*resource.NewMilliQuantity(8700, resource.DecimalSI),
		*resource.NewMilliQuantity(11599, resource.DecimalSI),
		*resource.NewMilliQuantity(62300, resource.DecimalSI),
		*resource.NewMilliQuantity(95600, resource.DecimalSI),
		*resource.NewMilliQuantity(134600, resource.DecimalSI),
	}, *resource.NewMilliQuantity(9700, resource.DecimalSI)
	index = searchLastLessElement(nums, target)
	if index != 3 {
		t.Errorf("Got %v expected %v", index, 3)
	}
	nums, target = []resource.Quantity{
		*resource.NewMilliQuantity(1000, resource.DecimalSI),
		*resource.NewMilliQuantity(2000, resource.DecimalSI),
	}, *resource.NewMilliQuantity(0, resource.DecimalSI)
	index = searchLastLessElement(nums, target)
	if index != -1 {
		t.Errorf("Got %v expected %v", index, -1)
	}
}

func TestGetIndex(t *testing.T) {
	rms := []clusterapis.ResourceModel{
		{
			Grade: 0,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024, resource.DecimalSI),
				},
			},
		},
		{
			Grade: 1,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
					Max:  *resource.NewQuantity(2, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(1024, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024*2, resource.DecimalSI),
				},
			},
		},
	}

	rs, err := InitSummary(rms)

	if err != nil {
		t.Errorf("Got %v expected %v", err, nil)
	}

	crn := ClusterResourceNode{
		quantity: 1,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024, resource.DecimalSI),
		},
	}
	index := rs.getIndex(crn)

	if index != 1 {
		t.Errorf("Got %v expected %v", index, 1)
	}

	crn = ClusterResourceNode{
		quantity: 1,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(20, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*100, resource.DecimalSI),
		},
	}
	index = rs.getIndex(crn)

	if index != 1 {
		t.Errorf("Got %v expected %v", index, 1)
	}
}

func TestClusterResourceNodeComparator(t *testing.T) {
	crn1 := ClusterResourceNode{
		quantity: 10,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(10, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024, resource.DecimalSI),
		},
	}

	crn2 := ClusterResourceNode{
		quantity: 789,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(2, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024, resource.DecimalSI),
		},
	}
	if res := clusterResourceNodeComparator(crn1, crn2); res != 1 {
		t.Errorf("Got %v expected %v", res, 1)
	}

	crn1 = ClusterResourceNode{
		quantity: 10,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(6, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024, resource.DecimalSI),
		},
	}

	crn2 = ClusterResourceNode{
		quantity: 789,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(6, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024, resource.DecimalSI),
		},
	}
	if res := clusterResourceNodeComparator(crn1, crn2); res != 0 {
		t.Errorf("Got %v expected %v", res, 0)
	}

	crn1 = ClusterResourceNode{
		quantity: 10,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(6, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024, resource.DecimalSI),
		},
	}

	crn2 = ClusterResourceNode{
		quantity: 789,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(6, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*10, resource.DecimalSI),
		},
	}
	if res := clusterResourceNodeComparator(crn1, crn2); res != -1 {
		t.Errorf("Got %v expected %v", res, -1)
	}
}

func TestGetNodeNumFromModel(t *testing.T) {
	rms := []clusterapis.ResourceModel{
		{
			Grade: 0,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024, resource.DecimalSI),
				},
			},
		},
		{
			Grade: 1,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
					Max:  *resource.NewQuantity(2, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(1024, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024*2, resource.DecimalSI),
				},
			},
		},
	}

	rs, err := InitSummary(rms)
	if actualValue := len(rs); actualValue != 2 {
		t.Errorf("Got %v expected %v", actualValue, 2)
	}

	if err != nil {
		t.Errorf("Got %v expected %v", err, nil)
	}

	crn1 := ClusterResourceNode{
		quantity: 3,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(8, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*3, resource.DecimalSI),
		},
	}

	crn2 := ClusterResourceNode{
		quantity: 1,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*6, resource.DecimalSI),
		},
	}

	crn3 := ClusterResourceNode{
		quantity: 2,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*6, resource.DecimalSI),
		},
	}

	rs.AddToResourceSummary(crn1)
	rs.AddToResourceSummary(crn2)
	rs.AddToResourceSummary(crn3)

	for index := range rs {
		num := rs.GetNodeNumFromModel(&rs[index])
		if index == 0 {
			if num != 0 {
				t.Errorf("Got %v expected %v", num, 0)
			}
		}
		if index == 1 {
			if num != 2 {
				t.Errorf("Got %v expected %v", num, 2)
			}
		}
	}
}

func TestConvertToResourceList(t *testing.T) {
	rslist := corev1.ResourceList{
		corev1.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
		corev1.ResourceMemory: *resource.NewQuantity(1024*6, resource.DecimalSI),
	}
	rsl := ConvertToResourceList(rslist)
	for name := range rsl {
		if reflect.TypeOf(name).String() != "v1alpha1.ResourceName" {
			t.Errorf("Got %v expected %v", reflect.TypeOf(name), "v1alpha1.ResourceName")
		}
	}
}

func TestLlConvertToRbt(t *testing.T) {
	crn1 := ClusterResourceNode{
		quantity: 6,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(2, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*7, resource.DecimalSI),
		},
	}

	crn2 := ClusterResourceNode{
		quantity: 5,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(6, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*3, resource.DecimalSI),
		},
	}

	crn3 := ClusterResourceNode{
		quantity: 4,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(5, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*8, resource.DecimalSI),
		},
	}

	crn4 := ClusterResourceNode{
		quantity: 3,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(8, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*3, resource.DecimalSI),
		},
	}

	crn5 := ClusterResourceNode{
		quantity: 2,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*6, resource.DecimalSI),
		},
	}

	crn6 := ClusterResourceNode{
		quantity: 1,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(2, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*12, resource.DecimalSI),
		},
	}
	mylist := list.New()
	mylist.PushBack(crn5)
	mylist.PushBack(crn1)
	mylist.PushBack(crn6)
	mylist.PushBack(crn3)
	mylist.PushBack(crn2)
	mylist.PushBack(crn4)

	rbt := llConvertToRbt(mylist)
	fmt.Println(rbt)
	if actualValue := rbt.Size(); actualValue != 6 {
		t.Errorf("Got %v expected %v", actualValue, 6)
	}

	actualValue := rbt.GetNode(crn5)
	node := actualValue.Key.(ClusterResourceNode)
	if quantity := node.quantity; quantity != 2 {
		t.Errorf("Got %v expected %v", actualValue, 2)
	}

	actualValue = rbt.GetNode(crn6)
	node = actualValue.Key.(ClusterResourceNode)
	if quantity := node.quantity; quantity != 1 {
		t.Errorf("Got %v expected %v", actualValue, 1)
	}

	actualValue = rbt.GetNode(crn1)
	node = actualValue.Key.(ClusterResourceNode)
	if quantity := node.quantity; quantity != 6 {
		t.Errorf("Got %v expected %v", actualValue, 6)
	}
}

func TestRbtConvertToLl(t *testing.T) {
	crn1 := ClusterResourceNode{
		quantity: 6,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(2, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*7, resource.DecimalSI),
		},
	}

	crn2 := ClusterResourceNode{
		quantity: 5,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(6, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*3, resource.DecimalSI),
		},
	}

	crn3 := ClusterResourceNode{
		quantity: 4,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(5, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*8, resource.DecimalSI),
		},
	}

	crn4 := ClusterResourceNode{
		quantity: 3,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(8, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*3, resource.DecimalSI),
		},
	}

	crn5 := ClusterResourceNode{
		quantity: 2,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*6, resource.DecimalSI),
		},
	}

	crn6 := ClusterResourceNode{
		quantity: 1,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(2, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*12, resource.DecimalSI),
		},
	}
	tree := rbt.NewWith(clusterResourceNodeComparator)

	if actualValue := tree.Size(); actualValue != 0 {
		t.Errorf("Got %v expected %v", actualValue, 0)
	}

	if actualValue := tree.GetNode(2).Size(); actualValue != 0 {
		t.Errorf("Got %v expected %v", actualValue, 0)
	}

	tree.Put(crn2, crn2.quantity)
	tree.Put(crn1, crn1.quantity)
	tree.Put(crn6, crn6.quantity)
	tree.Put(crn3, crn3.quantity)
	tree.Put(crn5, crn5.quantity)
	tree.Put(crn4, crn4.quantity)

	ll := rbtConvertToLl(tree)
	fmt.Println(ll)

	for element := ll.Front(); element != nil; element = element.Next() {
		fmt.Println(element.Value)
	}

	actualValue := ll.Front()
	node := actualValue.Value.(ClusterResourceNode)
	if quantity := node.quantity; quantity != 2 {
		t.Errorf("Got %v expected %v", actualValue, 2)
	}
	ll.Remove(actualValue)

	actualValue = ll.Front()
	node = actualValue.Value.(ClusterResourceNode)
	if quantity := node.quantity; quantity != 6 {
		t.Errorf("Got %v expected %v", actualValue, 6)
	}
	ll.Remove(actualValue)

	actualValue = ll.Front()
	node = actualValue.Value.(ClusterResourceNode)
	if quantity := node.quantity; quantity != 1 {
		t.Errorf("Got %v expected %v", actualValue, 1)
	}
	ll.Remove(actualValue)

	actualValue = ll.Front()
	node = actualValue.Value.(ClusterResourceNode)
	if quantity := node.quantity; quantity != 4 {
		t.Errorf("Got %v expected %v", actualValue, 4)
	}
	ll.Remove(actualValue)

	actualValue = ll.Front()
	node = actualValue.Value.(ClusterResourceNode)
	if quantity := node.quantity; quantity != 5 {
		t.Errorf("Got %v expected %v", actualValue, 5)
	}
	ll.Remove(actualValue)

	actualValue = ll.Front()
	node = actualValue.Value.(ClusterResourceNode)
	if quantity := node.quantity; quantity != 3 {
		t.Errorf("Got %v expected %v", actualValue, 3)
	}
	ll.Remove(actualValue)

	if actualValue := ll.Len(); actualValue != 0 {
		t.Errorf("Got %v expected %v", actualValue, 0)
	}
}

func TestAddToResourceSummary(t *testing.T) {
	rms := []clusterapis.ResourceModel{
		{
			Grade: 0,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024, resource.DecimalSI),
				},
			},
		},
		{
			Grade: 1,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
					Max:  *resource.NewQuantity(2, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(1024, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024*2, resource.DecimalSI),
				},
			},
		},
	}

	rs, err := InitSummary(rms)
	if actualValue := len(rs); actualValue != 2 {
		t.Errorf("Got %v expected %v", actualValue, 2)
	}

	if err != nil {
		t.Errorf("Got %v expected %v", err, nil)
	}

	crn1 := ClusterResourceNode{
		quantity: 3,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(8, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*3, resource.DecimalSI),
		},
	}

	crn2 := ClusterResourceNode{
		quantity: 1,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*6, resource.DecimalSI),
		},
	}

	crn3 := ClusterResourceNode{
		quantity: 2,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*6, resource.DecimalSI),
		},
	}

	rs.AddToResourceSummary(crn1)
	rs.AddToResourceSummary(crn2)
	rs.AddToResourceSummary(crn3)

	for index, v := range rs {
		num := rs.GetNodeNumFromModel(&rs[index])
		if index == 0 && num != 0 {
			t.Errorf("Got %v expected %v", num, 0)
		}
		if index == 1 && num != 2 {
			t.Errorf("Got %v expected %v", num, 2)
		}
		if index == 0 && v.Quantity != 0 {
			t.Errorf("Got %v expected %v", v.Quantity, 0)
		}
		if index == 1 && v.Quantity != 6 {
			t.Errorf("Got %v expected %v", v.Quantity, 6)
		}
	}

	crn4 := ClusterResourceNode{
		quantity: 2,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(0, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(19, resource.DecimalSI),
		},
	}

	rs.AddToResourceSummary(crn4)

	if actualValue := rs[0]; actualValue.Quantity != 2 {
		t.Errorf("Got %v expected %v", actualValue, 2)
	}

	if actualValue := rs[0]; rs.GetNodeNumFromModel(&actualValue) != 1 {
		t.Errorf("Got %v expected %v", actualValue, 1)
	}
}

func TestDeleteFromResourceSummary(t *testing.T) {
	rms := []clusterapis.ResourceModel{
		{
			Grade: 0,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024, resource.DecimalSI),
				},
			},
		},
		{
			Grade: 1,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
					Max:  *resource.NewQuantity(2, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(1024, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024*2, resource.DecimalSI),
				},
			},
		},
	}

	rs, err := InitSummary(rms)
	if actualValue := len(rs); actualValue != 2 {
		t.Errorf("Got %v expected %v", actualValue, 2)
	}

	if err != nil {
		t.Errorf("Got %v expected %v", err, nil)
	}

	crn1 := ClusterResourceNode{
		quantity: 3,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(8, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*3, resource.DecimalSI),
		},
	}

	crn2 := ClusterResourceNode{
		quantity: 1,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*6, resource.DecimalSI),
		},
	}

	crn3 := ClusterResourceNode{
		quantity: 2,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*6, resource.DecimalSI),
		},
	}

	rs.AddToResourceSummary(crn1)
	rs.AddToResourceSummary(crn2)
	rs.AddToResourceSummary(crn3)

	for index := range rs {
		num := rs.GetNodeNumFromModel(&rs[index])
		if index == 0 && num != 0 {
			t.Errorf("Got %v expected %v", num, 0)
		}
		if index == 1 && num != 2 {
			t.Errorf("Got %v expected %v", num, 2)
		}
		if index == 0 && rs[index].Quantity != 0 {
			t.Errorf("Got %v expected %v", rs[index].Quantity, 0)
		}
		if index == 1 && rs[index].Quantity != 6 {
			t.Errorf("Got %v expected %v", rs[index].Quantity, 6)
		}
	}

	crn4 := ClusterResourceNode{
		quantity: 2,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(19, resource.DecimalSI),
		},
	}

	err = rs.DeleteFromResourceSummary(crn4)

	if err == nil {
		t.Errorf("Got %v expected %v", err, nil)
	}

	if actualValue := rs[1]; actualValue.Quantity != 6 {
		t.Errorf("Got %v expected %v", actualValue, 6)
	}

	if actualValue := rs[0]; rs.GetNodeNumFromModel(&actualValue) != 0 {
		t.Errorf("Got %v expected %v", actualValue, 0)
	}
}

func TestUpdateSummary(t *testing.T) {
	rms := []clusterapis.ResourceModel{
		{
			Grade: 0,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024, resource.DecimalSI),
				},
			},
		},
		{
			Grade: 1,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: clusterapis.ResourceCPU,
					Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
					Max:  *resource.NewQuantity(2, resource.DecimalSI),
				},
				{
					Name: clusterapis.ResourceMemory,
					Min:  *resource.NewMilliQuantity(1024, resource.DecimalSI),
					Max:  *resource.NewQuantity(1024*2, resource.DecimalSI),
				},
			},
		},
	}

	rs, err := InitSummary(rms)
	if actualValue := len(rs); actualValue != 2 {
		t.Errorf("Got %v expected %v", actualValue, 2)
	}

	if err != nil {
		t.Errorf("Got %v expected %v", err, nil)
	}

	crn1 := ClusterResourceNode{
		quantity: 3,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(8, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*3, resource.DecimalSI),
		},
	}

	crn2 := ClusterResourceNode{
		quantity: 1,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*6, resource.DecimalSI),
		},
	}

	crn3 := ClusterResourceNode{
		quantity: 2,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*6, resource.DecimalSI),
		},
	}

	rs.AddToResourceSummary(crn1)
	rs.AddToResourceSummary(crn2)
	rs.AddToResourceSummary(crn3)

	for index := range rs {
		num := rs.GetNodeNumFromModel(&rs[index])
		if index == 0 && num != 0 {
			t.Errorf("Got %v expected %v", num, 0)
		}
		if index == 1 && num != 2 {
			t.Errorf("Got %v expected %v", num, 2)
		}
		if index == 0 && rs[index].Quantity != 0 {
			t.Errorf("Got %v expected %v", rs[index].Quantity, 0)
		}
		if index == 1 && rs[index].Quantity != 6 {
			t.Errorf("Got %v expected %v", rs[index].Quantity, 6)
		}
	}

	crn2 = ClusterResourceNode{
		quantity: 1,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(1024*6, resource.DecimalSI),
		},
	}

	crn4 := ClusterResourceNode{
		quantity: 2,
		resourceList: ResourceList{
			clusterapis.ResourceCPU:    *resource.NewMilliQuantity(1, resource.DecimalSI),
			clusterapis.ResourceMemory: *resource.NewQuantity(19, resource.DecimalSI),
		},
	}

	err = rs.UpdateInResourceSummary(crn2, crn4)

	if err != nil {
		t.Errorf("Got %v expected %v", err, nil)
	}

	if actualValue := rs[1]; actualValue.Quantity != 7 {
		t.Errorf("Got %v expected %v", actualValue, 7)
	}

	if actualValue := rs[1]; rs.GetNodeNumFromModel(&actualValue) != 3 {
		t.Errorf("Got %v expected %v", rs.GetNodeNumFromModel(&actualValue), 3)
	}
}
