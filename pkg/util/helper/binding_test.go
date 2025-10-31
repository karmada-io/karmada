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

package helper

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
	"github.com/karmada-io/karmada/pkg/util/names"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

const (
	ClusterMember1 = "member1"
	ClusterMember2 = "member2"
	ClusterMember3 = "member3"
	emptyUID       = ""
	evenUID        = "4858ca61-3ff8-4095-8267-f3d059d8074c"
	oddUID         = "d43fdeeb-d750-4e2b-bee3-64eb94534f4c"
)

func Test_dispenser_AllocateByWeight(t *testing.T) {
	tests := []struct {
		name              string
		newReplicas       int32
		initialAssignment []workv1alpha2.TargetCluster
		weightList        ClusterWeightInfoList
		desired           []workv1alpha2.TargetCluster
		uuid              types.UID
	}{
		{
			name:        "Scale up 6 replicas",
			newReplicas: 6,
			uuid:        emptyUID,
			initialAssignment: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 1},
				{Name: "B", Replicas: 2},
				{Name: "C", Replicas: 3},
			},
			weightList: []ClusterWeightInfo{
				{ClusterName: "A", Weight: 1},
				{ClusterName: "B", Weight: 2},
				{ClusterName: "C", Weight: 3},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 2},
				{Name: "B", Replicas: 4},
				{Name: "C", Replicas: 6},
			},
		},
		{
			name:        "Scale up 3 replicas",
			newReplicas: 3,
			uuid:        emptyUID,
			initialAssignment: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 1},
				{Name: "B", Replicas: 2},
				{Name: "C", Replicas: 3},
			},
			weightList: []ClusterWeightInfo{
				{ClusterName: "A", Weight: 1},
				{ClusterName: "B", Weight: 2},
				{ClusterName: "C", Weight: 3},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 2},
				{Name: "B", Replicas: 3},
				{Name: "C", Replicas: 4},
			},
		},
		{
			name:        "Scale up 2 replicas",
			newReplicas: 2,
			uuid:        emptyUID,
			initialAssignment: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 1},
				{Name: "B", Replicas: 2},
				{Name: "C", Replicas: 3},
			},
			weightList: []ClusterWeightInfo{
				{ClusterName: "A", Weight: 1},
				{ClusterName: "B", Weight: 2},
				{ClusterName: "C", Weight: 3},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 1},
				{Name: "B", Replicas: 3},
				{Name: "C", Replicas: 4},
			},
		},
		{
			name:              "HA scenario with weight 2:1: assign 1 replica",
			newReplicas:       1,
			uuid:              emptyUID,
			initialAssignment: nil,
			weightList: []ClusterWeightInfo{
				{ClusterName: "A", Weight: 2},
				{ClusterName: "B", Weight: 1},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 1},
				{Name: "B", Replicas: 0},
			},
		},
		{
			name:              "HA scenario with weight 2:1: assign 2 replicas",
			newReplicas:       2,
			uuid:              emptyUID,
			initialAssignment: nil,
			weightList: []ClusterWeightInfo{
				{ClusterName: "A", Weight: 2},
				{ClusterName: "B", Weight: 1},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 1},
				{Name: "B", Replicas: 1},
			},
		},
		{
			name:              "HA scenario with weight 2:1: assign 3 replicas",
			newReplicas:       3,
			uuid:              emptyUID,
			initialAssignment: nil,
			weightList: []ClusterWeightInfo{
				{ClusterName: "A", Weight: 2},
				{ClusterName: "B", Weight: 1},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 2},
				{Name: "B", Replicas: 1},
			},
		},
		{
			name:              "brand new assign 5 replicas",
			newReplicas:       5,
			uuid:              emptyUID,
			initialAssignment: nil,
			weightList: []ClusterWeightInfo{
				{ClusterName: "A", Weight: 170},
				{ClusterName: "B", Weight: 110},
				{ClusterName: "C", Weight: 60},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 2},
				{Name: "B", Replicas: 2},
				{Name: "C", Replicas: 1},
			},
		},
		{
			name:              "brand new assign 6 replicas",
			newReplicas:       6,
			uuid:              emptyUID,
			initialAssignment: nil,
			weightList: []ClusterWeightInfo{
				{ClusterName: "A", Weight: 170},
				{ClusterName: "B", Weight: 110},
				{ClusterName: "C", Weight: 60},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 3},
				{Name: "B", Replicas: 2},
				{Name: "C", Replicas: 1},
			},
		},
		{
			name:              "brand new assign 5 replicas with odd uuid",
			newReplicas:       5,
			uuid:              oddUID,
			initialAssignment: nil,
			weightList: []ClusterWeightInfo{
				{ClusterName: "A", Weight: 1},
				{ClusterName: "B", Weight: 1},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 2},
				{Name: "B", Replicas: 3},
			},
		},
		{
			name:              "brand new assign 5 replicas with even uuid",
			newReplicas:       5,
			uuid:              evenUID,
			initialAssignment: nil,
			weightList: []ClusterWeightInfo{
				{ClusterName: "A", Weight: 1},
				{ClusterName: "B", Weight: 1},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 3},
				{Name: "B", Replicas: 2},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewDispenser(tt.newReplicas, tt.initialAssignment, tt.uuid)
			a.AllocateByWeight(tt.weightList)
			if !a.Done() {
				t.Errorf("dispensing unexpectedly not finished")
			}
			if !testhelper.IsScheduleResultEqual(a.Result, tt.desired) {
				t.Errorf("expected result after AllocateByWeight: %v, but got: %v", tt.desired, a.Result)
			}
		})
	}
}

func TestDispenseReplicasByTargetClusters(t *testing.T) {
	type args struct {
		clusters []workv1alpha2.TargetCluster
		sum      int32
	}
	tests := []struct {
		name string
		args args
		// wants specifies multi possible desired result, any one got is expected
		wants [][]workv1alpha2.TargetCluster
	}{
		{
			name: "empty clusters",
			args: args{
				clusters: []workv1alpha2.TargetCluster{},
				sum:      10,
			},
			wants: [][]workv1alpha2.TargetCluster{
				{},
			},
		},
		{
			name: "1 cluster, 5 replicas, 10 sum",
			args: args{
				clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 5,
					},
				},
				sum: 10,
			},
			wants: [][]workv1alpha2.TargetCluster{
				{
					{
						Name:     ClusterMember1,
						Replicas: 10,
					},
				},
			},
		},
		{
			name: "3 cluster, 1:1:1, 12 sum",
			args: args{
				clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 5,
					},
					{
						Name:     ClusterMember2,
						Replicas: 5,
					},
					{
						Name:     ClusterMember3,
						Replicas: 5,
					},
				},
				sum: 12,
			},
			wants: [][]workv1alpha2.TargetCluster{
				{
					{
						Name:     ClusterMember1,
						Replicas: 4,
					},
					{
						Name:     ClusterMember2,
						Replicas: 4,
					},
					{
						Name:     ClusterMember3,
						Replicas: 4,
					},
				},
			},
		},
		{
			name: "3 cluster, 1:1:1, 10 sum",
			args: args{
				clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 5,
					},
					{
						Name:     ClusterMember2,
						Replicas: 5,
					},
					{
						Name:     ClusterMember3,
						Replicas: 5,
					},
				},
				sum: 10,
			},
			wants: [][]workv1alpha2.TargetCluster{
				{
					{
						Name:     ClusterMember1,
						Replicas: 4,
					},
					{
						Name:     ClusterMember2,
						Replicas: 3,
					},
					{
						Name:     ClusterMember3,
						Replicas: 3,
					},
				},
				{
					{
						Name:     ClusterMember1,
						Replicas: 3,
					},
					{
						Name:     ClusterMember2,
						Replicas: 4,
					},
					{
						Name:     ClusterMember3,
						Replicas: 3,
					},
				},
				{
					{
						Name:     ClusterMember1,
						Replicas: 3,
					},
					{
						Name:     ClusterMember2,
						Replicas: 3,
					},
					{
						Name:     ClusterMember3,
						Replicas: 4,
					},
				},
			},
		},
		{
			name: "3 cluster, 1:2:3, 13 sum",
			args: args{
				clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 1,
					},
					{
						Name:     ClusterMember2,
						Replicas: 2,
					},
					{
						Name:     ClusterMember3,
						Replicas: 3,
					},
				},
				sum: 13,
			},
			wants: [][]workv1alpha2.TargetCluster{
				{
					{
						Name:     ClusterMember1,
						Replicas: 2,
					},
					{
						Name:     ClusterMember2,
						Replicas: 4,
					},
					{
						Name:     ClusterMember3,
						Replicas: 7,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SpreadReplicasByTargetClusters(tt.args.sum, tt.args.clusters, nil, "")
			for _, want := range tt.wants {
				if testhelper.IsScheduleResultEqual(got, want) {
					return
				}
			}
			t.Errorf("dynamicScaleUp() got = %v, wants %v", got, tt.wants)
		})
	}
}

func TestObtainBindingSpecExistingClusters(t *testing.T) {
	tests := []struct {
		name        string
		bindingSpec workv1alpha2.ResourceBindingSpec
		want        sets.Set[string]
	}{
		{
			name: "unique cluster name without GracefulEvictionTasks field",
			bindingSpec: workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
					{
						Name:     "member2",
						Replicas: 3,
					},
				},
				RequiredBy: []workv1alpha2.BindingSnapshot{
					{
						Clusters: []workv1alpha2.TargetCluster{
							{
								Name:     "member3",
								Replicas: 2,
							},
						},
					},
				},
			},
			want: sets.New("member1", "member2", "member3"),
		},
		{
			name: "all spec fields do not contain duplicate cluster names",
			bindingSpec: workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
					{
						Name:     "member2",
						Replicas: 3,
					},
				},
				RequiredBy: []workv1alpha2.BindingSnapshot{
					{
						Clusters: []workv1alpha2.TargetCluster{
							{
								Name:     "member3",
								Replicas: 2,
							},
						},
					},
				},
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "member4",
					},
				},
			},
			want: sets.New("member1", "member2", "member3", "member4"),
		},
		{
			name: "duplicate cluster name",
			bindingSpec: workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
					{
						Name:     "member2",
						Replicas: 3,
					},
				},
				RequiredBy: []workv1alpha2.BindingSnapshot{
					{
						Clusters: []workv1alpha2.TargetCluster{
							{
								Name:     "member3",
								Replicas: 2,
							},
						},
					},
				},
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "member3",
					},
				},
			},
			want: sets.New("member1", "member2", "member3"),
		},
		{
			name: "unique cluster names with GracefulEvictionTasks with PurgeMode Immediately",
			bindingSpec: workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
					{
						Name:     "member2",
						Replicas: 3,
					},
				},
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "member3",
						PurgeMode:   policyv1alpha1.PurgeModeDirectly,
					},
					{
						FromCluster: "member4",
						PurgeMode:   policyv1alpha1.PurgeModeGracefully,
					},
					{
						FromCluster: "member5",
						PurgeMode:   policyv1alpha1.Never,
					},
				},
			},
			want: sets.New("member1", "member2", "member4", "member5"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ObtainBindingSpecExistingClusters(tt.bindingSpec); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ObtainBindingSpecExistingClusters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindOrphanWorks(t *testing.T) {
	type args struct {
		c                client.Client
		bindingNamespace string
		bindingName      string
		bindingID        string
		expectClusters   sets.Set[string]
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha1.Work
		wantErr bool
	}{
		{
			name: "get cluster name error",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "w1",
							Namespace:       "wrong format",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingPermanentIDLabel: "3617252f-b1bb-43b0-98a1-c7de833c472c",
							},
						},
					},
				).WithIndex(
					&workv1alpha1.Work{},
					indexregistry.WorkIndexByLabelResourceBindingID,
					indexregistry.GenLabelIndexerFunc(workv1alpha2.ResourceBindingPermanentIDLabel),
				).Build(),
				bindingNamespace: "default",
				bindingName:      "binding",
				bindingID:        "3617252f-b1bb-43b0-98a1-c7de833c472c",
				expectClusters:   sets.New("clusterx"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "namespace scope",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "w1",
							Namespace:       names.ExecutionSpacePrefix + "cluster1",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingPermanentIDLabel: "3617252f-b1bb-43b0-98a1-c7de833c472c",
							},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "not-selected-because-of-labels",
							Namespace:       names.ExecutionSpacePrefix + "cluster1",
							ResourceVersion: "999",
							Labels:          map[string]string{},
							Annotations: map[string]string{
								workv1alpha2.ResourceBindingNamespaceAnnotationKey: "default",
								workv1alpha2.ResourceBindingNameAnnotationKey:      "binding",
							},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "not-selected-because-of-cluster",
							Namespace:       names.ExecutionSpacePrefix + "clusterx",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingPermanentIDLabel: "3617252f-b1bb-43b0-98a1-c7de833c472c",
							},
						},
					},
				).WithIndex(
					&workv1alpha1.Work{},
					indexregistry.WorkIndexByLabelResourceBindingID,
					indexregistry.GenLabelIndexerFunc(workv1alpha2.ResourceBindingPermanentIDLabel),
				).Build(),
				bindingNamespace: "default",
				bindingName:      "binding",
				bindingID:        "3617252f-b1bb-43b0-98a1-c7de833c472c",
				expectClusters:   sets.New("clusterx"),
			},
			want: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "w1",
						Namespace:       names.ExecutionSpacePrefix + "cluster1",
						ResourceVersion: "999",
						Labels: map[string]string{
							workv1alpha2.ResourceBindingPermanentIDLabel: "3617252f-b1bb-43b0-98a1-c7de833c472c",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "cluster scope",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "w1",
							Namespace:       names.ExecutionSpacePrefix + "cluster1",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ClusterResourceBindingPermanentIDLabel: "3617252f-b1bb-43b0-98a1-c7de833c472c",
							},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "not-selected-because-of-labels",
							Namespace:       names.ExecutionSpacePrefix + "cluster1",
							ResourceVersion: "999",
							Labels:          map[string]string{},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "not-selected-because-of-cluster",
							Namespace:       names.ExecutionSpacePrefix + "clusterx",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ClusterResourceBindingPermanentIDLabel: "3617252f-b1bb-43b0-98a1-c7de833c472c",
							},
						},
					},
				).WithIndex(
					&workv1alpha1.Work{},
					indexregistry.WorkIndexByLabelClusterResourceBindingID,
					indexregistry.GenLabelIndexerFunc(workv1alpha2.ClusterResourceBindingPermanentIDLabel),
				).Build(),
				bindingNamespace: "",
				bindingName:      "binding",
				bindingID:        "3617252f-b1bb-43b0-98a1-c7de833c472c",
				expectClusters:   sets.New("clusterx"),
			},
			want: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "w1",
						Namespace:       names.ExecutionSpacePrefix + "cluster1",
						ResourceVersion: "999",
						Labels: map[string]string{
							workv1alpha2.ClusterResourceBindingPermanentIDLabel: "3617252f-b1bb-43b0-98a1-c7de833c472c",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindOrphanWorks(context.Background(), tt.args.c, tt.args.bindingNamespace, tt.args.bindingName, tt.args.bindingID, tt.args.expectClusters)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindOrphanWorks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindOrphanWorks() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindWorksInClusters(t *testing.T) {
	type args struct {
		c                client.Client
		bindingNamespace string
		bindingName      string
		bindingID        string
		targetClusters   sets.Set[string]
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha1.Work
		wantErr bool
	}{
		{
			name: "namespace scope: no works in target clusters",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "work1",
							Namespace:       names.ExecutionSpacePrefix + "cluster1",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingPermanentIDLabel: "binding-id",
							},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "work2",
							Namespace:       names.ExecutionSpacePrefix + "cluster2",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingPermanentIDLabel: "binding-id",
							},
						},
					},
				).WithIndex(
					&workv1alpha1.Work{},
					indexregistry.WorkIndexByLabelResourceBindingID,
					indexregistry.GenLabelIndexerFunc(workv1alpha2.ResourceBindingPermanentIDLabel),
				).Build(),
				bindingNamespace: "default",
				bindingName:      "test-binding",
				bindingID:        "binding-id",
				targetClusters:   sets.New("cluster3"),
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "namespace scope: some works in target clusters",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "work1",
							Namespace:       names.ExecutionSpacePrefix + "cluster1",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingPermanentIDLabel: "binding-id",
							},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "work2",
							Namespace:       names.ExecutionSpacePrefix + "cluster2",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingPermanentIDLabel: "binding-id",
							},
						},
					},
				).WithIndex(
					&workv1alpha1.Work{},
					indexregistry.WorkIndexByLabelResourceBindingID,
					indexregistry.GenLabelIndexerFunc(workv1alpha2.ResourceBindingPermanentIDLabel),
				).Build(),
				bindingNamespace: "default",
				bindingName:      "test-binding",
				bindingID:        "binding-id",
				targetClusters:   sets.New("cluster1"),
			},
			want: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "work1",
						Namespace:       names.ExecutionSpacePrefix + "cluster1",
						ResourceVersion: "999",
						Labels: map[string]string{
							workv1alpha2.ResourceBindingPermanentIDLabel: "binding-id",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "namespace scope: error getting cluster name",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "work1",
							Namespace:       "invalid-namespace",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingPermanentIDLabel: "binding-id",
							},
						},
					},
				).WithIndex(
					&workv1alpha1.Work{},
					indexregistry.WorkIndexByLabelResourceBindingID,
					indexregistry.GenLabelIndexerFunc(workv1alpha2.ResourceBindingPermanentIDLabel),
				).Build(),
				bindingNamespace: "default",
				bindingName:      "test-binding",
				bindingID:        "binding-id",
				targetClusters:   sets.New("cluster1"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "cluster scope: some works in target clusters",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "work3",
							Namespace:       names.ExecutionSpacePrefix + "clusterA",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ClusterResourceBindingPermanentIDLabel: "cluster-binding-id",
							},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "work4",
							Namespace:       names.ExecutionSpacePrefix + "clusterB",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ClusterResourceBindingPermanentIDLabel: "cluster-binding-id",
							},
						},
					},
				).WithIndex(
					&workv1alpha1.Work{},
					indexregistry.WorkIndexByLabelClusterResourceBindingID,
					indexregistry.GenLabelIndexerFunc(workv1alpha2.ClusterResourceBindingPermanentIDLabel),
				).Build(),
				bindingNamespace: "",
				bindingName:      "cluster-binding",
				bindingID:        "cluster-binding-id",
				targetClusters:   sets.New("clusterA"),
			},
			want: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "work3",
						Namespace:       names.ExecutionSpacePrefix + "clusterA",
						ResourceVersion: "999",
						Labels: map[string]string{
							workv1alpha2.ClusterResourceBindingPermanentIDLabel: "cluster-binding-id",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindWorksInClusters(context.TODO(), tt.args.c, tt.args.bindingNamespace, tt.args.bindingName, tt.args.bindingID, tt.args.targetClusters)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindWorksInClusters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindWorksInClusters() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveOrphanWorks(t *testing.T) {
	makeWork := func(name string) *workv1alpha1.Work {
		return &workv1alpha1.Work{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: names.ExecutionSpacePrefix + "c",
				ResourceVersion: "9999",
			},
		}
	}

	type args struct {
		c     client.Client
		works []workv1alpha1.Work
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha1.Work
		wantErr bool
	}{
		{
			name: "works is empty",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					makeWork("w1"), makeWork("w2"), makeWork("w3")).Build(),
				works: nil,
			},
			wantErr: false,
			want:    []workv1alpha1.Work{*makeWork("w1"), *makeWork("w2"), *makeWork("w3")},
		},
		{
			name: "remove existing and non existing",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					makeWork("w1"), makeWork("w2"), makeWork("w3")).Build(),
				works: []workv1alpha1.Work{*makeWork("w3"), *makeWork("w4")},
			},
			wantErr: true,
			want:    []workv1alpha1.Work{*makeWork("w1"), *makeWork("w2")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RemoveOrphanWorks(context.Background(), tt.args.c, tt.args.works); (err != nil) != tt.wantErr {
				t.Errorf("RemoveOrphanWorks() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := &workv1alpha1.WorkList{}
			if err := tt.args.c.List(context.TODO(), got); err != nil {
				t.Error(err)
				return
			}
			if !reflect.DeepEqual(got.Items, tt.want) {
				t.Errorf("RemoveOrphanWorks() got = %v, want %v", got.Items, tt.want)
			}
		})
	}
}

func TestFetchWorkload(t *testing.T) {
	type args struct {
		dynamicClient   dynamic.Interface
		informerManager func(context.Context) genericmanager.SingleClusterInformerManager
		restMapper      meta.RESTMapper
		resource        workv1alpha2.ObjectReference
	}
	tests := []struct {
		name    string
		args    args
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "kind is not registered",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				informerManager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(ctx, dynamicfake.NewSimpleDynamicClient(scheme.Scheme), 0)
				},
				restMapper: meta.NewDefaultRESTMapper(nil),
				resource:   workv1alpha2.ObjectReference{APIVersion: "v1", Kind: "Pod"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "not found",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				informerManager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(ctx, dynamicfake.NewSimpleDynamicClient(scheme.Scheme), 0)
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Namespace:  "default",
					Name:       "pod",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "namespace scope: get from client",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
					&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"}}),
				informerManager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(ctx, dynamicfake.NewSimpleDynamicClient(scheme.Scheme), 0)
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Namespace:  "default",
					Name:       "pod",
				},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":      "pod",
					"namespace": "default",
				},
			}},
			wantErr: false,
		},
		{
			name: "namespace scope: get from cache",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				informerManager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"}})
					m := genericmanager.NewSingleClusterInformerManager(ctx, c, 0)
					m.Lister(corev1.SchemeGroupVersion.WithResource("pods"))
					m.Start()
					m.WaitForCacheSync()
					return m
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Namespace:  "default",
					Name:       "pod",
				},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":      "pod",
					"namespace": "default",
				},
			}},
			wantErr: false,
		},
		{
			name: "cluster scope: get from client",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node"}}),
				informerManager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(ctx, dynamicfake.NewSimpleDynamicClient(scheme.Scheme), 0)
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Node"), meta.RESTScopeRoot)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Node",
					Name:       "node",
				},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Node",
				"metadata": map[string]interface{}{
					"name": "node",
				},
			}},
			wantErr: false,
		},
		{
			name: "cluster scope: get from cache",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				informerManager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node"}})
					m := genericmanager.NewSingleClusterInformerManager(ctx, c, 0)
					m.Lister(corev1.SchemeGroupVersion.WithResource("nodes"))
					m.Start()
					m.WaitForCacheSync()
					return m
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Node"), meta.RESTScopeRoot)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Node",
					Name:       "node",
				},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Node",
				"metadata": map[string]interface{}{
					"name": "node",
				},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			mgr := tt.args.informerManager(ctx)
			got, err := FetchResourceTemplate(context.TODO(), tt.args.dynamicClient, mgr, tt.args.restMapper, tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchResourceTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil {
				// client will add some default fields in spec & status. We don't compare these fields.
				delete(got.Object, "spec")
				delete(got.Object, "status")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FetchResourceTemplate() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFetchWorkloadByLabelSelector(t *testing.T) {
	type args struct {
		dynamicClient   dynamic.Interface
		informerManager func(ctx context.Context) genericmanager.SingleClusterInformerManager
		restMapper      meta.RESTMapper
		resource        workv1alpha2.ObjectReference
		selector        *metav1.LabelSelector
	}
	tests := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "kind is not registered",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				informerManager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(ctx, dynamicfake.NewSimpleDynamicClient(scheme.Scheme), 0)
				},
				restMapper: meta.NewDefaultRESTMapper(nil),
				resource:   workv1alpha2.ObjectReference{APIVersion: "v1", Kind: "Pod"},
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "namespace scope: get from client",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
					&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", Labels: map[string]string{"foo": "foo"}}}),
				informerManager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(ctx, dynamicfake.NewSimpleDynamicClient(scheme.Scheme), 0)
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Namespace:  "default",
				},
				selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "foo"}},
			},
			want:    1,
			wantErr: false,
		},
		{
			name: "namespace scope: get from cache",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				informerManager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", Labels: map[string]string{"bar": "foo"}}})
					m := genericmanager.NewSingleClusterInformerManager(ctx, c, 0)
					m.Lister(corev1.SchemeGroupVersion.WithResource("pods"))
					m.Start()
					m.WaitForCacheSync()
					return m
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Namespace:  "default",
					Name:       "pod",
				},
				selector: &metav1.LabelSelector{MatchLabels: map[string]string{"bar": "foo"}},
			},
			want:    1,
			wantErr: false,
		},
		{
			name: "cluster scope: get from client",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", Labels: map[string]string{"bar": "bar"}}}),
				informerManager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(ctx, dynamicfake.NewSimpleDynamicClient(scheme.Scheme), 0)
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Node"), meta.RESTScopeRoot)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Node",
					Name:       "node",
				},
				selector: &metav1.LabelSelector{MatchLabels: map[string]string{"bar": "bar"}},
			},
			want:    1,
			wantErr: false,
		},
		{
			name: "cluster scope: get from cache",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				informerManager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", Labels: map[string]string{"bar": "foo"}}})
					m := genericmanager.NewSingleClusterInformerManager(ctx, c, 0)
					m.Lister(corev1.SchemeGroupVersion.WithResource("nodes"))
					m.Start()
					m.WaitForCacheSync()
					return m
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Node"), meta.RESTScopeRoot)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Node",
					Name:       "node",
				},
				selector: &metav1.LabelSelector{MatchLabels: map[string]string{"bar": "foo"}},
			},
			want:    1,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			mgr := tt.args.informerManager(ctx)
			selector, _ := metav1.LabelSelectorAsSelector(tt.args.selector)
			got, err := FetchResourceTemplatesByLabelSelector(tt.args.dynamicClient, mgr, tt.args.restMapper, tt.args.resource, selector)
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchResourceTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.want {
				t.Errorf("FetchResourceTemplate() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteWorkByRBNamespaceAndName(t *testing.T) {
	type args struct {
		c         client.Client
		namespace string
		name      string
		bindingID string
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha1.Work
		wantErr bool
	}{
		{
			name: "work is not found",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithIndex(
					&workv1alpha1.Work{},
					indexregistry.WorkIndexByLabelResourceBindingID,
					indexregistry.GenLabelIndexerFunc(workv1alpha2.ResourceBindingPermanentIDLabel),
				).Build(),
				namespace: "default",
				name:      "foo",
				bindingID: "3617252f-b1bb-43b0-98a1-c7de833c472c",
			},
			want:    []workv1alpha1.Work{},
			wantErr: false,
		},
		{
			name: "delete rb's work",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name: "w1", Namespace: names.ExecutionSpacePrefix + "cluster",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingPermanentIDLabel: "3617252f-b1bb-43b0-98a1-c7de833c472c",
							},
							Annotations: map[string]string{
								workv1alpha2.ResourceBindingNameAnnotationKey:      "foo",
								workv1alpha2.ResourceBindingNamespaceAnnotationKey: "default",
							},
						},
					},
				).WithIndex(
					&workv1alpha1.Work{},
					indexregistry.WorkIndexByLabelResourceBindingID,
					indexregistry.GenLabelIndexerFunc(workv1alpha2.ResourceBindingPermanentIDLabel),
				).Build(),
				namespace: "default",
				name:      "foo",
				bindingID: "3617252f-b1bb-43b0-98a1-c7de833c472c",
			},
			want:    []workv1alpha1.Work{},
			wantErr: false,
		},
		{
			name: "delete crb's work",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name: "w1", Namespace: names.ExecutionSpacePrefix + "cluster",
							Labels: map[string]string{
								workv1alpha2.ClusterResourceBindingPermanentIDLabel: "3617252f-b1bb-43b0-98a1-c7de833c472c",
							},
							Annotations: map[string]string{
								workv1alpha2.ClusterResourceBindingAnnotationKey: "foo",
							},
						},
					},
				).WithIndex(
					&workv1alpha1.Work{},
					indexregistry.WorkIndexByLabelClusterResourceBindingID,
					indexregistry.GenLabelIndexerFunc(workv1alpha2.ClusterResourceBindingPermanentIDLabel),
				).Build(),
				name:      "foo",
				bindingID: "3617252f-b1bb-43b0-98a1-c7de833c472c",
			},
			want:    []workv1alpha1.Work{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DeleteWorks(context.Background(), tt.args.c, tt.args.namespace, tt.args.name, tt.args.bindingID); (err != nil) != tt.wantErr {
				t.Errorf("DeleteWorks() error = %v, wantErr %v", err, tt.wantErr)
			}
			list := &workv1alpha1.WorkList{}
			if err := tt.args.c.List(context.Background(), list); err != nil {
				t.Error(err)
				return
			}
			if got := list.Items; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteWorks() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateReplicaRequirements(t *testing.T) {
	type args struct {
		podTemplate *corev1.PodTemplateSpec
	}
	tests := []struct {
		name string
		args args
		want *workv1alpha2.ReplicaRequirements
	}{
		{
			name: "nodeClaim and resource request are nil",
			args: args{
				podTemplate: &corev1.PodTemplateSpec{},
			},
			want: nil,
		},
		{
			name: "has nodeClaim",
			args: args{
				podTemplate: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						NodeSelector: map[string]string{"foo": "foo1"},
						Tolerations:  []corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}},
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{{
									MatchFields: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpExists}},
								}}}},
						},
					},
				},
			},
			want: &workv1alpha2.ReplicaRequirements{
				NodeClaim: &workv1alpha2.NodeClaim{
					NodeSelector: map[string]string{"foo": "foo1"},
					Tolerations:  []corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}},
					HardNodeAffinity: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{{
							MatchFields: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpExists}},
						}}},
				},
			},
		},
		{
			name: "has resourceRequest",
			args: args{
				podTemplate: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Resources: corev1.ResourceRequirements{
								Limits: map[corev1.ResourceName]resource.Quantity{
									"cpu": *resource.NewMilliQuantity(200, resource.DecimalSI),
								},
							},
						}},
					},
				},
			},
			want: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"cpu": *resource.NewMilliQuantity(200, resource.DecimalSI),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateReplicaRequirements(tt.args.podTemplate); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateReplicaRequirements() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConstructClusterWideKey(t *testing.T) {
	type args struct {
		resource workv1alpha2.ObjectReference
	}
	tests := []struct {
		name    string
		args    args
		want    keys.ClusterWideKey
		wantErr bool
	}{
		{
			name: "wrong APIVersion",
			args: args{resource: workv1alpha2.ObjectReference{
				APIVersion: "a/b/c",
				Kind:       "Foo",
				Namespace:  "test",
				Name:       "foo",
			}},
			want:    keys.ClusterWideKey{},
			wantErr: true,
		},
		{
			name: "APIVersion: v1",
			args: args{resource: workv1alpha2.ObjectReference{
				APIVersion: "v1",
				Kind:       "Foo",
				Namespace:  "test",
				Name:       "foo",
			}},
			want: keys.ClusterWideKey{
				Version:   "v1",
				Kind:      "Foo",
				Namespace: "test",
				Name:      "foo",
			},
			wantErr: false,
		},
		{
			name: "APIVersion: test/v1",
			args: args{resource: workv1alpha2.ObjectReference{
				APIVersion: "test/v1",
				Kind:       "Foo",
				Namespace:  "test",
				Name:       "foo",
			}},
			want: keys.ClusterWideKey{
				Group:     "test",
				Version:   "v1",
				Kind:      "Foo",
				Namespace: "test",
				Name:      "foo",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConstructClusterWideKey(tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConstructClusterWideKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConstructClusterWideKey() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func compareResourceLists(t *testing.T, expected, actual corev1.ResourceList) {
	for resourceName, expectedQuantity := range expected {
		actualQuantity, exists := actual[resourceName]
		assert.True(t, exists, "Resource %s is missing in actual result", resourceName)
		assert.Equal(t, expectedQuantity.Value(), actualQuantity.Value(), "Resource %s value mismatch", resourceName)
		assert.Equal(t, expectedQuantity.Format, actualQuantity.Format, "Resource %s format mismatch", resourceName)
	}
	assert.Equal(t, len(expected), len(actual), "Resource list length mismatch")
}

func TestCalculateResourceUsage(t *testing.T) {
	tests := []struct {
		name     string
		rb       *workv1alpha2.ResourceBinding
		expected corev1.ResourceList
	}{
		{
			name: "Calculate usage with components",
			rb: &workv1alpha2.ResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
						{Name: "cluster2"},
					},
					Components: []workv1alpha2.Component{
						{
							Replicas: 2,
							ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
								ResourceRequest: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
			expected: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("4Gi"),
			},
		},
		{
			name: "Calculate usage with replica requirements",
			rb: &workv1alpha2.ResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1", Replicas: 3},
						{Name: "cluster2", Replicas: 2},
					},
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
			expected: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("5"),
				corev1.ResourceMemory: resource.MustParse("10Gi"),
			},
		},
		{
			name:     "Nil ResourceBinding",
			rb:       nil,
			expected: corev1.ResourceList{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateResourceUsage(tt.rb)
			compareResourceLists(t, tt.expected, result)
		})
	}
}

func TestAggregateComponentResources(t *testing.T) {
	tests := []struct {
		name       string
		components []workv1alpha2.Component
		expected   corev1.ResourceList
	}{
		{
			name: "Aggregate resources from components",
			components: []workv1alpha2.Component{
				{
					Replicas: 2,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
				{
					Replicas: 3,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
			expected: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
		},
		{
			name: "No components",
			components: []workv1alpha2.Component{
				{
					Replicas: 0,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			expected: corev1.ResourceList{},
		},
		{
			name:       "Empty components",
			components: []workv1alpha2.Component{},
			expected:   corev1.ResourceList{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aggregateComponentResources(tt.components)
			compareResourceLists(t, tt.expected, result)
		})
	}
}

func TestFindTargetStatusItemByCluster(t *testing.T) {
	tests := []struct {
		name             string
		aggregatedStatus []workv1alpha2.AggregatedStatusItem
		cluster          string
		expectedItem     workv1alpha2.AggregatedStatusItem
		expectedFound    bool
	}{
		{
			name:             "empty aggregated status list",
			aggregatedStatus: []workv1alpha2.AggregatedStatusItem{},
			cluster:          "member1",
			expectedItem:     workv1alpha2.AggregatedStatusItem{},
			expectedFound:    false,
		},
		{
			name: "cluster not found in aggregated status list",
			aggregatedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "member2"},
				{ClusterName: "member3"},
			},
			cluster:       "member1",
			expectedItem:  workv1alpha2.AggregatedStatusItem{},
			expectedFound: false,
		},
		{
			name: "cluster found in aggregated status list",
			aggregatedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "member1", Status: &runtime.RawExtension{Raw: []byte(`{"key": "value"}`)}},
				{ClusterName: "member2"},
			},
			cluster: "member1",
			expectedItem: workv1alpha2.AggregatedStatusItem{
				ClusterName: "member1",
				Status:      &runtime.RawExtension{Raw: []byte(`{"key": "value"}`)},
			},
			expectedFound: true,
		},
		{
			name: "multiple clusters with the same name, return the first match",
			aggregatedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "member1", Status: &runtime.RawExtension{Raw: []byte(`{"key1": "value1"}`)}},
				{ClusterName: "member1", Status: &runtime.RawExtension{Raw: []byte(`{"key2": "value2"}`)}},
			},
			cluster: "member1",
			expectedItem: workv1alpha2.AggregatedStatusItem{
				ClusterName: "member1",
				Status:      &runtime.RawExtension{Raw: []byte(`{"key1": "value1"}`)},
			},
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, found := FindTargetStatusItemByCluster(tt.aggregatedStatus, tt.cluster)
			assert.Equal(t, tt.expectedFound, found)
			assert.Equal(t, tt.expectedItem.ClusterName, item.ClusterName)
			if tt.expectedItem.Status != nil && item.Status != nil {
				assert.Equal(t, tt.expectedItem.Status.Raw, item.Status.Raw)
			} else {
				assert.Nil(t, item.Status)
				assert.Nil(t, tt.expectedItem.Status)
			}
		})
	}
}

func TestObtainClustersWithPurgeModeDirectly(t *testing.T) {
	tests := []struct {
		name         string
		bindingSpec  workv1alpha2.ResourceBindingSpec
		wantClusters sets.Set[string]
	}{
		{
			name: "Multiple clusters with different PurgeModes",
			bindingSpec: workv1alpha2.ResourceBindingSpec{
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "cluster1",
						PurgeMode:   policyv1alpha1.PurgeModeDirectly,
					},
					{
						FromCluster: "cluster2",
						PurgeMode:   policyv1alpha1.PurgeModeDirectly,
					},
					{
						FromCluster: "cluster3",
						PurgeMode:   policyv1alpha1.PurgeModeGracefully,
					},
				},
			},
			wantClusters: sets.New("cluster1", "cluster2"),
		},
		{
			name: "Empty GracefulEvictionTasks",
			bindingSpec: workv1alpha2.ResourceBindingSpec{
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{},
			},
			wantClusters: sets.New[string](),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClusters := ObtainClustersWithPurgeModeDirectly(tt.bindingSpec)
			if !gotClusters.Equal(tt.wantClusters) {
				t.Errorf("ObtainClustersWithPurgeModeDirectly() = %v, want %v", gotClusters.UnsortedList(), tt.wantClusters.UnsortedList())
			}
		})
	}
}
