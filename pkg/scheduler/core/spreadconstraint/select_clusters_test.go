package spreadconstraint

import (
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

var calcFunc calcAvailableReplicasFunc = func(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster {
	tc := make([]workv1alpha2.TargetCluster, 0, len(clusters))
	for i, cluster := range clusters {
		tc = append(tc, workv1alpha2.TargetCluster{
			Name:     cluster.Name,
			Replicas: 10 << i,
		})
	}
	return tc
}

func TestSelectClusters(t *testing.T) {
	tests := []struct {
		name             string
		clusterScores    framework.ClusterScoreList
		rbSpec           *workv1alpha2.ResourceBindingSpec
		expectedErr      error
		expectedClusters []string
	}{
		{
			name: "empty spread constraints",
			clusterScores: framework.ClusterScoreList{
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C1",
						},
					},
					Score: 90,
				},
			},
			rbSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{},
			},
			expectedErr:      nil,
			expectedClusters: []string{"C1"},
		},
		{
			name: "static weighted schedule strategy",
			clusterScores: framework.ClusterScoreList{
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C1",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C2",
						},
					},
					Score: 90,
				},
			},
			rbSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{
									TargetCluster: policyv1alpha1.ClusterAffinity{
										ClusterNames: []string{"C1", "C2"},
									},
									Weight: 1,
								},
							},
						},
					},
				},
			},
			expectedErr:      nil,
			expectedClusters: []string{"C1", "C2"},
		},
		{
			name: "spread by cluster with duplicated schedule strategy",
			clusterScores: framework.ClusterScoreList{
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C1",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C2",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C3",
						},
					},
					Score: 100,
				},
			},
			rbSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     2,
							MinGroups:     1,
						},
					},
				},
			},
			expectedErr:      nil,
			expectedClusters: []string{"C1", "C3"},
		},
		{
			name: "spread by cluster without duplicated schedule strategy",
			clusterScores: framework.ClusterScoreList{
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C1",
						},
					},
					Score: 50,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C2",
						},
					},
					Score: 40,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C3",
						},
					},
					Score: 90,
				},
			},
			rbSpec: &workv1alpha2.ResourceBindingSpec{
				Replicas: 60,
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     2,
							MinGroups:     1,
						},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
					},
				},
			},
			expectedErr:      nil,
			expectedClusters: []string{"C2", "C3"},
		},
		{
			name: "spread by label is not supported now",
			clusterScores: framework.ClusterScoreList{
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C1",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C2",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C3",
						},
					},
					Score: 100,
				},
			},
			rbSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByLabel: "example.io/env",
							MaxGroups:     2,
							MinGroups:     1,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     2,
							MinGroups:     1,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldProvider,
							MaxGroups:     2,
							MinGroups:     1,
						},
					},
				},
			},
			expectedErr:      errors.New("label:example.io/env spread is not supported now"),
			expectedClusters: nil,
		},
		{
			name: "spread constraints not be satisfied",
			clusterScores: framework.ClusterScoreList{
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C1",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Region: "R1",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C2",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Region: "R1",
						},
					},
					Score: 90,
				},
			},
			rbSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     2,
							MinGroups:     1,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldRegion,
							MaxGroups:     2,
							MinGroups:     2,
						},
					},
				},
			},
			expectedErr:      errors.New("available number 1 is less than region minimum requirement 2"),
			expectedClusters: nil,
		},
		{
			name: "spread by zone",
			clusterScores: framework.ClusterScoreList{
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C1",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone: "Z1",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C2",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone: "Z1",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C3",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone: "Z2",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C4",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone: "Z2",
						},
					},
					Score: 90,
				},
			},
			rbSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     5,
							MinGroups:     3,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldZone,
							MaxGroups:     2,
							MinGroups:     2,
						},
					},
				},
			},
			expectedErr:      nil,
			expectedClusters: []string{"C1", "C2", "C3", "C4"},
		},
		{
			name: "spread by region",
			clusterScores: framework.ClusterScoreList{
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C1",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone:   "Z1",
							Region: "R1",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C2",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone:   "Z2",
							Region: "R1",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C3",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone:   "Z3",
							Region: "R2",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C4",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone:   "Z4",
							Region: "R3",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C5",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone:   "Z5",
							Region: "R4",
						},
					},
					Score: 90,
				},
			},
			rbSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     5,
							MinGroups:     3,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldZone,
							MaxGroups:     6,
							MinGroups:     4,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldRegion,
							MaxGroups:     4,
							MinGroups:     3,
						},
					},
				},
			},
			expectedErr:      nil,
			expectedClusters: []string{"C1", "C2", "C3", "C4"},
		},
		{
			name: "spread by provider",
			clusterScores: framework.ClusterScoreList{
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C1",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone:     "Z1",
							Region:   "R1",
							Provider: "P1",
						},
					},
					Score: 90,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C2",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone:     "Z2",
							Region:   "R1",
							Provider: "P1",
						},
					},
					Score: 50,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C3",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone:     "Z3",
							Region:   "R2",
							Provider: "P2",
						},
					},
					Score: 30,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C4",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone:     "Z4",
							Region:   "R3",
							Provider: "P2",
						},
					},
					Score: 70,
				},
				{
					Cluster: &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "C5",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							Zone:     "Z5",
							Region:   "R4",
							Provider: "P3",
						},
					},
					Score: 80,
				},
			},
			rbSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     5,
							MinGroups:     3,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldZone,
							MaxGroups:     6,
							MinGroups:     4,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldRegion,
							MaxGroups:     4,
							MinGroups:     3,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldProvider,
							MaxGroups:     3,
							MinGroups:     2,
						},
					},
				},
			},
			expectedErr:      nil,
			expectedClusters: []string{"C1", "C2", "C3", "C4", "C5"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusters, err := SelectClusters(tt.clusterScores, tt.rbSpec, calcFunc)
			if tt.expectedErr != nil && err == nil {
				t.Errorf("expected err: %v, but did not get err", tt.expectedErr)
			}
			if tt.expectedErr == nil && err != nil {
				t.Errorf("expected no err, but got err: %v", err)
			}
			if tt.expectedErr != nil && err != nil && !strings.Contains(err.Error(), tt.expectedErr.Error()) {
				t.Errorf("expected err: %v, but got err: %v", tt.expectedErr, err)
			}

			var clusterNames []string
			for _, c := range clusters {
				clusterNames = append(clusterNames, c.Name)
			}
			// shouldn't care about the order of clusters
			sort.Slice(clusterNames, func(i, j int) bool {
				return clusterNames[i] < clusterNames[j]
			})
			if len(clusterNames) != len(tt.expectedClusters) || (len(clusterNames) > 0 && !reflect.DeepEqual(tt.expectedClusters, clusterNames)) {
				t.Errorf("expected: %v, but got %v", tt.expectedClusters, clusterNames)
			}
		})
	}
}
