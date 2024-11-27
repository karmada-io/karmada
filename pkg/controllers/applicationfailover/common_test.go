/*
Copyright 2023 The Karmada Authors.

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

package applicationfailover

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestTimeStampProcess(t *testing.T) {
	key := types.NamespacedName{
		Namespace: "default",
		Name:      "test",
	}
	cluster := "cluster-1"

	m := newWorkloadUnhealthyMap()
	m.setTimeStamp(key, cluster)
	res := m.hasWorkloadBeenUnhealthy(key, cluster)
	assert.Equal(t, true, res)

	time := m.getTimeStamp(key, cluster)
	assert.NotEmpty(t, time)

	m.delete(key)
	res = m.hasWorkloadBeenUnhealthy(key, cluster)
	assert.Equal(t, false, res)
}

func TestWorkloadUnhealthyMap_deleteIrrelevantClusters(t *testing.T) {
	cluster1 := "cluster-1"
	cluster2 := "cluster-2"
	cluster3 := "cluster-3"
	t.Run("normal case", func(t *testing.T) {
		key := types.NamespacedName{
			Namespace: "default",
			Name:      "test",
		}

		m := newWorkloadUnhealthyMap()

		m.setTimeStamp(key, cluster1)
		m.setTimeStamp(key, cluster2)
		m.setTimeStamp(key, cluster3)

		allClusters := sets.New[string](cluster2, cluster3)
		healthyClusters := []string{cluster3}

		m.deleteIrrelevantClusters(key, allClusters, healthyClusters)
		res1 := m.hasWorkloadBeenUnhealthy(key, cluster1)
		assert.Equal(t, false, res1)
		res2 := m.hasWorkloadBeenUnhealthy(key, cluster2)
		assert.Equal(t, true, res2)
		res3 := m.hasWorkloadBeenUnhealthy(key, cluster3)
		assert.Equal(t, false, res3)
	})

	t.Run("unhealthyClusters is nil", func(t *testing.T) {
		key := types.NamespacedName{
			Namespace: "default",
			Name:      "test",
		}

		m := newWorkloadUnhealthyMap()

		allClusters := sets.New[string](cluster2, cluster3)
		healthyClusters := []string{cluster3}

		m.deleteIrrelevantClusters(key, allClusters, healthyClusters)
		res := m.hasWorkloadBeenUnhealthy(key, cluster2)
		assert.Equal(t, false, res)
	})
}

func TestDistinguishUnhealthyClustersWithOthers(t *testing.T) {
	tests := []struct {
		name                  string
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		resourceBindingSpec   workv1alpha2.ResourceBindingSpec
		expectedClusters      []string
		expectedOthers        []string
	}{
		{
			name: "all applications are healthy",
			aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Health:      workv1alpha2.ResourceHealthy,
				},
				{
					ClusterName: "member2",
					Health:      workv1alpha2.ResourceHealthy,
				},
			},
			resourceBindingSpec: workv1alpha2.ResourceBindingSpec{},
			expectedClusters:    nil,
			expectedOthers:      []string{"member1", "member2"},
		},
		{
			name: "all applications are unknown",
			aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Health:      workv1alpha2.ResourceUnknown,
				},
				{
					ClusterName: "member2",
					Health:      workv1alpha2.ResourceUnknown,
				},
			},
			resourceBindingSpec: workv1alpha2.ResourceBindingSpec{},
			expectedClusters:    nil,
			expectedOthers:      []string{"member1", "member2"},
		},
		{
			name: "one application is unhealthy and not in gracefulEvictionTasks",
			aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Health:      workv1alpha2.ResourceHealthy,
				},
				{
					ClusterName: "member2",
					Health:      workv1alpha2.ResourceUnhealthy,
				},
			},
			resourceBindingSpec: workv1alpha2.ResourceBindingSpec{},
			expectedClusters:    []string{"member2"},
			expectedOthers:      []string{"member1"},
		},
		{
			name: "one application is unhealthy and in gracefulEvictionTasks",
			aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Health:      workv1alpha2.ResourceHealthy,
				},
				{
					ClusterName: "member2",
					Health:      workv1alpha2.ResourceUnhealthy,
				},
			},
			resourceBindingSpec: workv1alpha2.ResourceBindingSpec{
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "member2",
					},
				},
			},
			expectedClusters: nil,
			expectedOthers:   []string{"member1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, gotOthers := distinguishUnhealthyClustersWithOthers(tt.aggregatedStatusItems, tt.resourceBindingSpec); !reflect.DeepEqual(got, tt.expectedClusters) || !reflect.DeepEqual(gotOthers, tt.expectedOthers) {
				t.Errorf("distinguishUnhealthyClustersWithOthers() = (%v, %v), want (%v, %v)", got, gotOthers, tt.expectedClusters, tt.expectedOthers)
			}
		})
	}
}

func Test_parseJSONValue(t *testing.T) {
	type args struct {
		rawStatus []byte
		jsonPath  string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "target field not found",
			args: args{
				rawStatus: []byte(`{"readyReplicas": 2}`),
				jsonPath:  "{ .replicas }",
			},
			wantErr: assert.Error,
		},
		{
			name: "invalid jsonPath",
			args: args{
				rawStatus: []byte(`{"readyReplicas": 2}`),
				jsonPath:  "{ %replicas }",
			},
			wantErr: assert.Error,
		},
		{
			name: "success to parse",
			args: args{
				rawStatus: []byte(`{"replicas": 2}`),
				jsonPath:  "{ .replicas }",
			},
			wantErr: assert.NoError,
			want:    "2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJSONValue(tt.args.rawStatus, tt.args.jsonPath)
			if !tt.wantErr(t, err, fmt.Sprintf("parseJSONValue(%v, %v)", tt.args.rawStatus, tt.args.jsonPath)) {
				return
			}
			assert.Equalf(t, tt.want, got, "parseJSONValue(%v, %v)", tt.args.rawStatus, tt.args.jsonPath)
		})
	}
}

func Test_getClusterNamesFromTargetClusters(t *testing.T) {
	type args struct {
		targetClusters []workv1alpha2.TargetCluster
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "nil targetClusters",
			args: args{
				targetClusters: nil,
			},
			want: nil,
		},
		{
			name: "normal case",
			args: args{
				targetClusters: []workv1alpha2.TargetCluster{
					{Name: "c1", Replicas: 1},
					{Name: "c2", Replicas: 2},
				},
			},
			want: []string{"c1", "c2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, getClusterNamesFromTargetClusters(tt.args.targetClusters), "getClusterNamesFromTargetClusters(%v)", tt.args.targetClusters)
		})
	}
}

func Test_findTargetStatusItemByCluster(t *testing.T) {
	type args struct {
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		cluster               string
	}
	tests := []struct {
		name      string
		args      args
		want      workv1alpha2.AggregatedStatusItem
		wantExist bool
	}{
		{
			name: "nil aggregatedStatusItems",
			args: args{
				aggregatedStatusItems: nil,
				cluster:               "c1",
			},
			want:      workv1alpha2.AggregatedStatusItem{},
			wantExist: false,
		},
		{
			name: "cluster exist in the aggregatedStatusItems",
			args: args{
				aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "c1"},
					{ClusterName: "c2"},
				},
				cluster: "c1",
			},
			want:      workv1alpha2.AggregatedStatusItem{ClusterName: "c1"},
			wantExist: true,
		},
		{
			name: "cluster does not exist in the aggregatedStatusItems",
			args: args{
				aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "c1"},
					{ClusterName: "c2"},
				},
				cluster: "c?",
			},
			want:      workv1alpha2.AggregatedStatusItem{},
			wantExist: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := findTargetStatusItemByCluster(tt.args.aggregatedStatusItems, tt.args.cluster)
			assert.Equalf(t, tt.want, got, "findTargetStatusItemByCluster(%v, %v)", tt.args.aggregatedStatusItems, tt.args.cluster)
			assert.Equalf(t, tt.wantExist, got1, "findTargetStatusItemByCluster(%v, %v)", tt.args.aggregatedStatusItems, tt.args.cluster)
		})
	}
}

func Test_buildPreservedLabelState(t *testing.T) {
	type args struct {
		statePreservation *policyv1alpha1.StatePreservation
		rawStatus         []byte
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successful case",
			args: args{
				statePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{AliasLabelName: "key-a", JSONPath: "{ .replicas }"},
						{AliasLabelName: "key-b", JSONPath: "{ .health }"},
					},
				},
				rawStatus: []byte(`{"replicas": 2, "health": true}`),
			},
			wantErr: assert.NoError,
			want:    map[string]string{"key-a": "2", "key-b": "true"},
		},
		{
			name: "one statePreservation rule exist not found field",
			args: args{
				statePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{AliasLabelName: "key-a", JSONPath: "{ .replicas }"},
						{AliasLabelName: "key-b", JSONPath: "{ .notfound }"},
					},
				},
				rawStatus: []byte(`{"replicas": 2, "health": true}`),
			},
			wantErr: assert.Error,
			want:    nil,
		},
		{
			name: "one statePreservation rule has invalid jsonPath",
			args: args{
				statePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{AliasLabelName: "key-a", JSONPath: "{ .replicas }"},
						{AliasLabelName: "key-b", JSONPath: "{ %health }"},
					},
				},
				rawStatus: []byte(`{"replicas": 2, "health": true}`),
			},
			wantErr: assert.Error,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildPreservedLabelState(tt.args.statePreservation, tt.args.rawStatus)
			if !tt.wantErr(t, err, fmt.Sprintf("buildPreservedLabelState(%v, %v)", tt.args.statePreservation, tt.args.rawStatus)) {
				return
			}
			assert.Equalf(t, tt.want, got, "buildPreservedLabelState(%v, %v)", tt.args.statePreservation, tt.args.rawStatus)
		})
	}
}

func Test_buildTaskOptions(t *testing.T) {
	type args struct {
		failoverBehavior       *policyv1alpha1.ApplicationFailoverBehavior
		aggregatedStatus       []workv1alpha2.AggregatedStatusItem
		cluster                string
		producer               string
		clustersBeforeFailover []string
	}
	tests := []struct {
		name    string
		args    args
		want    workv1alpha2.TaskOptions
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Graciously purgeMode with ResourceBinding",
			args: args{
				failoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
					DecisionConditions: policyv1alpha1.DecisionConditions{
						TolerationSeconds: ptr.To[int32](100),
					},
					PurgeMode:          policyv1alpha1.Graciously,
					GracePeriodSeconds: ptr.To[int32](120),
					StatePreservation: &policyv1alpha1.StatePreservation{
						Rules: []policyv1alpha1.StatePreservationRule{
							{AliasLabelName: "key-a", JSONPath: "{ .replicas }"},
							{AliasLabelName: "key-b", JSONPath: "{ .health }"},
						},
					},
				},
				aggregatedStatus: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "c1", Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 2, "health": true}`)}},
					{ClusterName: "c2", Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 3, "health": false}`)}},
				},
				cluster:                "c1",
				producer:               RBApplicationFailoverControllerName,
				clustersBeforeFailover: []string{"c0"},
			},
			want: *workv1alpha2.NewTaskOptions(
				workv1alpha2.WithPurgeMode(policyv1alpha1.Graciously),
				workv1alpha2.WithProducer(RBApplicationFailoverControllerName),
				workv1alpha2.WithReason(workv1alpha2.EvictionReasonApplicationFailure),
				workv1alpha2.WithGracePeriodSeconds(ptr.To[int32](120)),
				workv1alpha2.WithPreservedLabelState(map[string]string{"key-a": "2", "key-b": "true"}),
				workv1alpha2.WithClustersBeforeFailover([]string{"c0"})),
			wantErr: assert.NoError,
		},
		{
			name: "Never purgeMode with ClusterResourceBinding",
			args: args{
				failoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
					DecisionConditions: policyv1alpha1.DecisionConditions{
						TolerationSeconds: ptr.To[int32](100),
					},
					PurgeMode: policyv1alpha1.Never,
					StatePreservation: &policyv1alpha1.StatePreservation{
						Rules: []policyv1alpha1.StatePreservationRule{
							{AliasLabelName: "key-a", JSONPath: "{ .replicas }"},
							{AliasLabelName: "key-b", JSONPath: "{ .health }"},
						},
					},
				},
				aggregatedStatus: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "c1", Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 2, "health": true}`)}},
					{ClusterName: "c2", Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 3, "health": false}`)}},
				},
				cluster:                "c1",
				producer:               CRBApplicationFailoverControllerName,
				clustersBeforeFailover: []string{"c0"},
			},
			want: *workv1alpha2.NewTaskOptions(
				workv1alpha2.WithPurgeMode(policyv1alpha1.Never),
				workv1alpha2.WithProducer(CRBApplicationFailoverControllerName),
				workv1alpha2.WithReason(workv1alpha2.EvictionReasonApplicationFailure),
				workv1alpha2.WithPreservedLabelState(map[string]string{"key-a": "2", "key-b": "true"}),
				workv1alpha2.WithSuppressDeletion(ptr.To[bool](true)),
				workv1alpha2.WithClustersBeforeFailover([]string{"c0"})),
			wantErr: assert.NoError,
		},
		{
			name: "Immediately purgeMode with ClusterResourceBinding",
			args: args{
				failoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
					DecisionConditions: policyv1alpha1.DecisionConditions{
						TolerationSeconds: ptr.To[int32](100),
					},
					PurgeMode: policyv1alpha1.Immediately,
					StatePreservation: &policyv1alpha1.StatePreservation{
						Rules: []policyv1alpha1.StatePreservationRule{
							{AliasLabelName: "key-a", JSONPath: "{ .replicas }"},
							{AliasLabelName: "key-b", JSONPath: "{ .health }"},
						},
					},
				},
				aggregatedStatus: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "c1", Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 2, "health": true}`)}},
					{ClusterName: "c2", Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 3, "health": false}`)}},
				},
				cluster:                "c1",
				producer:               CRBApplicationFailoverControllerName,
				clustersBeforeFailover: []string{"c0"},
			},
			want: *workv1alpha2.NewTaskOptions(
				workv1alpha2.WithPurgeMode(policyv1alpha1.Immediately),
				workv1alpha2.WithProducer(CRBApplicationFailoverControllerName),
				workv1alpha2.WithReason(workv1alpha2.EvictionReasonApplicationFailure),
				workv1alpha2.WithPreservedLabelState(map[string]string{"key-a": "2", "key-b": "true"}),
				workv1alpha2.WithClustersBeforeFailover([]string{"c0"})),
			wantErr: assert.NoError,
		},
		{
			name: "Graciously purgeMode with ResourceBinding, StatePreservation is nil",
			args: args{
				failoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
					DecisionConditions: policyv1alpha1.DecisionConditions{
						TolerationSeconds: ptr.To[int32](100),
					},
					PurgeMode:          policyv1alpha1.Graciously,
					GracePeriodSeconds: ptr.To[int32](120),
				},
				aggregatedStatus: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "c1", Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 2, "health": true}`)}},
					{ClusterName: "c2", Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 3, "health": false}`)}},
				},
				cluster:                "c1",
				producer:               RBApplicationFailoverControllerName,
				clustersBeforeFailover: []string{"c0"},
			},
			want: *workv1alpha2.NewTaskOptions(
				workv1alpha2.WithPurgeMode(policyv1alpha1.Graciously),
				workv1alpha2.WithProducer(RBApplicationFailoverControllerName),
				workv1alpha2.WithReason(workv1alpha2.EvictionReasonApplicationFailure),
				workv1alpha2.WithGracePeriodSeconds(ptr.To[int32](120))),
			wantErr: assert.NoError,
		},
		{
			name: "Graciously purgeMode with ResourceBinding, StatePreservation.Rules is nil",
			args: args{
				failoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
					DecisionConditions: policyv1alpha1.DecisionConditions{
						TolerationSeconds: ptr.To[int32](100),
					},
					PurgeMode:          policyv1alpha1.Graciously,
					GracePeriodSeconds: ptr.To[int32](120),
					StatePreservation:  &policyv1alpha1.StatePreservation{},
				},
				aggregatedStatus: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "c1", Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 2, "health": true}`)}},
					{ClusterName: "c2", Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 3, "health": false}`)}},
				},
				cluster:                "c1",
				producer:               RBApplicationFailoverControllerName,
				clustersBeforeFailover: []string{"c0"},
			},
			want: *workv1alpha2.NewTaskOptions(
				workv1alpha2.WithPurgeMode(policyv1alpha1.Graciously),
				workv1alpha2.WithProducer(RBApplicationFailoverControllerName),
				workv1alpha2.WithReason(workv1alpha2.EvictionReasonApplicationFailure),
				workv1alpha2.WithGracePeriodSeconds(ptr.To[int32](120))),
			wantErr: assert.NoError,
		},
		{
			name: "Graciously purgeMode with ResourceBinding, target cluster status in not collected",
			args: args{
				failoverBehavior: &policyv1alpha1.ApplicationFailoverBehavior{
					DecisionConditions: policyv1alpha1.DecisionConditions{
						TolerationSeconds: ptr.To[int32](100),
					},
					PurgeMode:          policyv1alpha1.Graciously,
					GracePeriodSeconds: ptr.To[int32](120),
					StatePreservation: &policyv1alpha1.StatePreservation{
						Rules: []policyv1alpha1.StatePreservationRule{
							{AliasLabelName: "key-a", JSONPath: "{ .replicas }"},
							{AliasLabelName: "key-b", JSONPath: "{ .health }"},
						},
					},
				},
				aggregatedStatus: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "c1"},
					{ClusterName: "c2", Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 3, "health": false}`)}},
				},
				cluster:                "c1",
				producer:               RBApplicationFailoverControllerName,
				clustersBeforeFailover: []string{"c0"},
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildTaskOptions(tt.args.failoverBehavior, tt.args.aggregatedStatus, tt.args.cluster, tt.args.producer, tt.args.clustersBeforeFailover)
			if !tt.wantErr(t, err, fmt.Sprintf("buildTaskOptions(%v, %v, %v, %v, %v)", tt.args.failoverBehavior, tt.args.aggregatedStatus, tt.args.cluster, tt.args.producer, tt.args.clustersBeforeFailover)) {
				return
			}
			gotTaskOptions := workv1alpha2.NewTaskOptions(got...)
			assert.Equalf(t, tt.want, *gotTaskOptions, "buildTaskOptions(%v, %v, %v, %v, %v)", tt.args.failoverBehavior, tt.args.aggregatedStatus, tt.args.cluster, tt.args.producer, tt.args.clustersBeforeFailover)
		})
	}
}
