package strategy

import "testing"

func Test_divideRemainingReplicas(t *testing.T) {
	type args struct {
		remainingReplicas   int
		desiredReplicaInfos map[string]int64
		clusterNames        []string
	}
	tests := []struct {
		name string
		args args
		want map[string]int64
	}{
		{
			name: "remainingReplicas 13",
			args: args{
				remainingReplicas: 13,
				desiredReplicaInfos: map[string]int64{
					ClusterMember1: 2,
					ClusterMember2: 3,
					ClusterMember3: 4,
				},
				clusterNames: []string{
					ClusterMember1, ClusterMember2, ClusterMember3,
				},
			},
			want: map[string]int64{
				ClusterMember1: 7,
				ClusterMember2: 7,
				ClusterMember3: 8,
			},
		},
		{
			name: "remainingReplicas 17",
			args: args{
				remainingReplicas: 17,
				desiredReplicaInfos: map[string]int64{
					ClusterMember1: 4,
					ClusterMember2: 3,
					ClusterMember3: 2,
				},
				clusterNames: []string{
					ClusterMember1, ClusterMember2, ClusterMember3,
				},
			},
			want: map[string]int64{
				ClusterMember1: 10,
				ClusterMember2: 9,
				ClusterMember3: 7,
			},
		},
		{
			name: "remainingReplicas 2",
			args: args{
				remainingReplicas: 2,
				desiredReplicaInfos: map[string]int64{
					ClusterMember1: 1,
					ClusterMember2: 1,
					ClusterMember3: 1,
				},
				clusterNames: []string{
					ClusterMember1, ClusterMember2, ClusterMember3,
				},
			},
			want: map[string]int64{
				ClusterMember1: 2,
				ClusterMember2: 2,
				ClusterMember3: 1,
			},
		},
		{
			name: "remainingReplicas 0",
			args: args{
				remainingReplicas: 0,
				desiredReplicaInfos: map[string]int64{
					ClusterMember1: 3,
					ClusterMember2: 3,
					ClusterMember3: 3,
				},
				clusterNames: []string{
					ClusterMember1, ClusterMember2, ClusterMember3,
				},
			},
			want: map[string]int64{
				ClusterMember1: 3,
				ClusterMember2: 3,
				ClusterMember3: 3,
			},
		},
	}
	IsTwoMapEqual := func(a, b map[string]int64) bool {
		return a[ClusterMember1] == b[ClusterMember1] && a[ClusterMember2] == b[ClusterMember2] && a[ClusterMember3] == b[ClusterMember3]
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			divideRemainingReplicas(tt.args.remainingReplicas, tt.args.desiredReplicaInfos, tt.args.clusterNames)
			if !IsTwoMapEqual(tt.args.desiredReplicaInfos, tt.want) {
				t.Errorf("divideRemainingReplicas() got = %v, want %v", tt.args.desiredReplicaInfos, tt.want)
			}
		})
	}
}
