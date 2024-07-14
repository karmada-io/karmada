/*
Copyright 2024 The Karmada Authors.

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

package remediation

import (
	"context"
	"reflect"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	remedyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func Test_isRemedyWorkOnCluster(t *testing.T) {
	type args struct {
		remedy  *remedyv1alpha1.Remedy
		cluster *clusterv1alpha1.Cluster
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "remedy.spec.clusterAffinity is nil",
			args: args{
				remedy: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo"},
					Spec: remedyv1alpha1.RemedySpec{
						DecisionMatches: []remedyv1alpha1.DecisionMatch{
							{
								ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
									ConditionType:   remedyv1alpha1.ServiceDomainNameResolutionReady,
									Operator:        remedyv1alpha1.ClusterConditionEqual,
									ConditionStatus: "True",
								}},
						},
						Actions: []remedyv1alpha1.RemedyAction{remedyv1alpha1.TrafficControl},
					},
				},
				cluster: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
				},
			},
			want: true,
		},
		{
			name: "remedy.spec.clusterAffinity.clusterNames is nil",
			args: args{
				remedy: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo"},
					Spec: remedyv1alpha1.RemedySpec{
						ClusterAffinity: &remedyv1alpha1.ClusterAffinity{
							ClusterNames: nil,
						},
						DecisionMatches: []remedyv1alpha1.DecisionMatch{
							{
								ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
									ConditionType:   remedyv1alpha1.ServiceDomainNameResolutionReady,
									Operator:        remedyv1alpha1.ClusterConditionEqual,
									ConditionStatus: "True",
								}},
						},
						Actions: []remedyv1alpha1.RemedyAction{remedyv1alpha1.TrafficControl},
					},
				},
				cluster: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
				},
			},
			want: false,
		},
		{
			name: "remedy.spec.clusterAffinity.clusterNames contains the target cluster",
			args: args{
				remedy: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo"},
					Spec: remedyv1alpha1.RemedySpec{
						ClusterAffinity: &remedyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"member1", "member2"},
						},
						DecisionMatches: []remedyv1alpha1.DecisionMatch{
							{
								ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
									ConditionType:   remedyv1alpha1.ServiceDomainNameResolutionReady,
									Operator:        remedyv1alpha1.ClusterConditionEqual,
									ConditionStatus: "True",
								}},
						},
						Actions: []remedyv1alpha1.RemedyAction{remedyv1alpha1.TrafficControl},
					},
				},
				cluster: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
				},
			},
			want: true,
		},
		{
			name: "remedy.spec.clusterAffinity.clusterNames do not contains the target cluster",
			args: args{
				remedy: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo"},
					Spec: remedyv1alpha1.RemedySpec{
						ClusterAffinity: &remedyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"member1", "member2"},
						},
						DecisionMatches: []remedyv1alpha1.DecisionMatch{
							{
								ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
									ConditionType:   remedyv1alpha1.ServiceDomainNameResolutionReady,
									Operator:        remedyv1alpha1.ClusterConditionEqual,
									ConditionStatus: "True",
								}},
						},
						Actions: []remedyv1alpha1.RemedyAction{remedyv1alpha1.TrafficControl},
					},
				},
				cluster: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "no-exist"},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRemedyWorkOnCluster(tt.args.remedy, tt.args.cluster); got != tt.want {
				t.Errorf("isRemedyWorkOnCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_remedyDecisionMatchWithCluster(t *testing.T) {
	type args struct {
		decisionMatches []remedyv1alpha1.DecisionMatch
		conditions      []metav1.Condition
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "nil decisionMatches",
			args: args{
				decisionMatches: nil,
				conditions:      []metav1.Condition{},
			},
			want: true,
		},
		{
			name: "nil conditions",
			args: args{
				decisionMatches: []remedyv1alpha1.DecisionMatch{
					{
						ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
							ConditionType:   remedyv1alpha1.ServiceDomainNameResolutionReady,
							Operator:        remedyv1alpha1.ClusterConditionEqual,
							ConditionStatus: "True",
						},
					},
				},
				conditions: nil,
			},
			want: false,
		},
		{
			name: "decisionMatches have nil clusterConditionMatch",
			args: args{
				decisionMatches: []remedyv1alpha1.DecisionMatch{
					{ClusterConditionMatch: nil},
				},
				conditions: []metav1.Condition{
					{
						Type:   "Ready",
						Status: metav1.ConditionFalse,
					},
					{
						Type:   string(remedyv1alpha1.ServiceDomainNameResolutionReady),
						Status: metav1.ConditionFalse,
					},
				},
			},
			want: false,
		},
		{
			name: "one of decisionMatches match with the conditions under Equal operation",
			args: args{
				decisionMatches: []remedyv1alpha1.DecisionMatch{
					{
						ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
							ConditionType:   remedyv1alpha1.ServiceDomainNameResolutionReady,
							Operator:        remedyv1alpha1.ClusterConditionEqual,
							ConditionStatus: "False",
						},
					},
					{
						ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
							ConditionType:   remedyv1alpha1.ConditionType("DoNotCareType"),
							Operator:        remedyv1alpha1.ClusterConditionEqual,
							ConditionStatus: "True",
						},
					},
				},
				conditions: []metav1.Condition{
					{
						Type:   "Ready",
						Status: metav1.ConditionFalse,
					},
					{
						Type:   string(remedyv1alpha1.ServiceDomainNameResolutionReady),
						Status: metav1.ConditionFalse,
					},
				},
			},
			want: true,
		},
		{
			name: "one of decisionMatches match with the conditions under NotEqual operation",
			args: args{
				decisionMatches: []remedyv1alpha1.DecisionMatch{
					{
						ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
							ConditionType:   remedyv1alpha1.ServiceDomainNameResolutionReady,
							Operator:        remedyv1alpha1.ClusterConditionNotEqual,
							ConditionStatus: "True",
						},
					},
					{
						ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
							ConditionType:   remedyv1alpha1.ConditionType("DoNotCareType"),
							Operator:        remedyv1alpha1.ClusterConditionEqual,
							ConditionStatus: "True",
						},
					},
				},
				conditions: []metav1.Condition{
					{
						Type:   "Ready",
						Status: metav1.ConditionFalse,
					},
					{
						Type:   string(remedyv1alpha1.ServiceDomainNameResolutionReady),
						Status: metav1.ConditionFalse,
					},
				},
			},
			want: true,
		},
		{
			name: "all of decisionMatches do not match with the conditions",
			args: args{
				decisionMatches: []remedyv1alpha1.DecisionMatch{
					{
						ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
							ConditionType:   remedyv1alpha1.ServiceDomainNameResolutionReady,
							Operator:        remedyv1alpha1.ClusterConditionEqual,
							ConditionStatus: "False",
						},
					},
					{
						ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
							ConditionType:   remedyv1alpha1.ConditionType("DoNotCareType"),
							Operator:        remedyv1alpha1.ClusterConditionEqual,
							ConditionStatus: "False",
						},
					},
				},
				conditions: []metav1.Condition{
					{
						Type:   "Ready",
						Status: metav1.ConditionTrue,
					},
					{
						Type:   string(remedyv1alpha1.ServiceDomainNameResolutionReady),
						Status: metav1.ConditionTrue,
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := remedyDecisionMatchWithCluster(tt.args.decisionMatches, tt.args.conditions); got != tt.want {
				t.Errorf("remedyDecisionMatchWithCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_calculateActions(t *testing.T) {
	type args struct {
		clusterRelatedRemedies []*remedyv1alpha1.Remedy
		cluster                *clusterv1alpha1.Cluster
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "nil clusterRelatedRemedies",
			args: args{
				clusterRelatedRemedies: nil,
				cluster: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionFalse,
							},
							{
								Type:   string(remedyv1alpha1.ServiceDomainNameResolutionReady),
								Status: metav1.ConditionFalse,
							},
						},
					},
				},
			},
			want: []string{},
		},
		{
			name: "normal test case",
			args: args{
				clusterRelatedRemedies: []*remedyv1alpha1.Remedy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "foo-01"},
						Spec: remedyv1alpha1.RemedySpec{
							DecisionMatches: []remedyv1alpha1.DecisionMatch{
								{
									ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
										ConditionType:   remedyv1alpha1.ServiceDomainNameResolutionReady,
										Operator:        remedyv1alpha1.ClusterConditionEqual,
										ConditionStatus: "False",
									}},
							},
							Actions: []remedyv1alpha1.RemedyAction{remedyv1alpha1.TrafficControl},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "foo-02"},
						Spec: remedyv1alpha1.RemedySpec{
							DecisionMatches: []remedyv1alpha1.DecisionMatch{
								{
									ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
										ConditionType:   "Ready",
										Operator:        remedyv1alpha1.ClusterConditionEqual,
										ConditionStatus: "False",
									}},
							},
							Actions: []remedyv1alpha1.RemedyAction{"IsolateSchedule"},
						},
					},
				},
				cluster: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionFalse,
							},
							{
								Type:   string(remedyv1alpha1.ServiceDomainNameResolutionReady),
								Status: metav1.ConditionFalse,
							},
						},
					},
				},
			},
			want: []string{"IsolateSchedule", "TrafficControl"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateActions(tt.args.clusterRelatedRemedies, tt.args.cluster); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("calculateActions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getClusterRelatedRemedies(t *testing.T) {
	type args struct {
		ctx     context.Context
		client  client.Client
		cluster *clusterv1alpha1.Cluster
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "client does not have Remedy object",
			args: args{
				ctx:     context.TODO(),
				client:  fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
				cluster: &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member1"}},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "normal test case",
			args: args{
				ctx: context.TODO(),
				client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&remedyv1alpha1.Remedy{
						ObjectMeta: metav1.ObjectMeta{Name: "nil-clusteraffinity"},
						Spec:       remedyv1alpha1.RemedySpec{},
					},
					&remedyv1alpha1.Remedy{
						ObjectMeta: metav1.ObjectMeta{Name: "contains-member1"},
						Spec: remedyv1alpha1.RemedySpec{
							ClusterAffinity: &remedyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
						},
					},
					&remedyv1alpha1.Remedy{
						ObjectMeta: metav1.ObjectMeta{Name: "do-not-contains-member1"},
						Spec: remedyv1alpha1.RemedySpec{
							ClusterAffinity: &remedyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}},
						},
					},
				).Build(),
				cluster: &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member1"}},
			},
			want:    []string{"contains-member1", "nil-clusteraffinity"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterRelatedRemedies, err := getClusterRelatedRemedies(tt.args.ctx, tt.args.client, tt.args.cluster)
			if (err != nil) != tt.wantErr {
				t.Errorf("getClusterRelatedRemedies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			var got []string
			for _, remedy := range clusterRelatedRemedies {
				got = append(got, remedy.Name)
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getClusterRelatedRemedies() got = %v, want %v", got, tt.want)
			}
		})
	}
}
