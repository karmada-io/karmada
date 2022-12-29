package helper

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

func TestWorksFullyApplied(t *testing.T) {
	type args struct {
		aggregatedStatuses []workv1alpha2.AggregatedStatusItem
		targetClusters     sets.String
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "no cluster",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
				},
				targetClusters: nil,
			},
			want: false,
		},
		{
			name: "no aggregatedStatuses",
			args: args{
				aggregatedStatuses: nil,
				targetClusters:     sets.NewString("member1"),
			},
			want: false,
		},
		{
			name: "cluster size is not equal to aggregatedStatuses",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
				},
				targetClusters: sets.NewString("member1", "member2"),
			},
			want: false,
		},
		{
			name: "aggregatedStatuses is equal to clusterNames and all applied",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
					{
						ClusterName: "member2",
						Applied:     true,
					},
				},
				targetClusters: sets.NewString("member1", "member2"),
			},
			want: true,
		},
		{
			name: "aggregatedStatuses is equal to clusterNames but not all applied",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
					{
						ClusterName: "member2",
						Applied:     false,
					},
				},
				targetClusters: sets.NewString("member1", "member2"),
			},
			want: false,
		},
		{
			name: "target clusters not match expected status",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
				},
				targetClusters: sets.NewString("member2"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := worksFullyApplied(tt.args.aggregatedStatuses, tt.args.targetClusters); got != tt.want {
				t.Errorf("worksFullyApplied() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_assembleWorkStatus(t *testing.T) {
	deployment := testhelper.NewDeploymentWithStatus("default", "nginx")
	workload, _ := ToUnstructured(deployment)
	workloadJSON, _ := workload.MarshalJSON()
	status, _ := BuildStatusRawExtension(deployment.Status)

	tests := []struct {
		name    string
		works   []workv1alpha1.Work
		want    []workv1alpha2.AggregatedStatusItem
		wantErr bool
	}{
		{
			name: "all cluster applied successfully",
			works: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:       names.GenerateWorkName(workload.GetKind(), workload.GetName(), workload.GetNamespace()),
						Namespace:  names.ExecutionSpacePrefix + "member1",
						Finalizers: []string{util.ExecutionControllerFinalizer},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: workloadJSON,
									},
								},
							},
						},
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						}},
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Health: workv1alpha1.ResourceHealthy,
								Status: status,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:       names.GenerateWorkName(workload.GetKind(), workload.GetName(), workload.GetNamespace()),
						Namespace:  names.ExecutionSpacePrefix + "member2",
						Finalizers: []string{util.ExecutionControllerFinalizer},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: workloadJSON,
									},
								},
							},
						},
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						}},
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Health: workv1alpha1.ResourceHealthy,
								Status: status,
							},
						},
					},
				},
			},
			want: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Status:      status,
					Applied:     true,
					Health:      workv1alpha2.ResourceHealthy,
				},
				{
					ClusterName: "member2",
					Status:      status,
					Applied:     true,
					Health:      workv1alpha2.ResourceHealthy,
				},
			},
			wantErr: false,
		},
		{
			name: "one cluster applied unsuccessfully",
			works: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:       names.GenerateWorkName(workload.GetKind(), workload.GetName(), workload.GetNamespace()),
						Namespace:  names.ExecutionSpacePrefix + "member1",
						Finalizers: []string{util.ExecutionControllerFinalizer},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: workloadJSON,
									},
								},
							},
						},
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{{
							Type:    workv1alpha1.WorkApplied,
							Status:  metav1.ConditionFalse,
							Message: FullyAppliedFailedMessage,
						}},
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Health: workv1alpha1.ResourceHealthy,
								Status: status,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:       names.GenerateWorkName(workload.GetKind(), workload.GetName(), workload.GetNamespace()),
						Namespace:  names.ExecutionSpacePrefix + "member2",
						Finalizers: []string{util.ExecutionControllerFinalizer},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: workloadJSON,
									},
								},
							},
						},
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						}},
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Health: workv1alpha1.ResourceHealthy,
								Status: status,
							},
						},
					},
				},
			},
			want: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName:    "member1",
					Status:         status,
					Applied:        false,
					AppliedMessage: FullyAppliedFailedMessage,
					Health:         workv1alpha2.ResourceHealthy,
				},
				{
					ClusterName: "member2",
					Status:      status,
					Applied:     true,
					Health:      workv1alpha2.ResourceHealthy,
				},
			},
			wantErr: false,
		},
		{
			name: "all cluster applied unsuccessfully",
			works: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:       names.GenerateWorkName(workload.GetKind(), workload.GetName(), workload.GetNamespace()),
						Namespace:  names.ExecutionSpacePrefix + "member1",
						Finalizers: []string{util.ExecutionControllerFinalizer},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: workloadJSON,
									},
								},
							},
						},
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{{
							Type:    workv1alpha1.WorkApplied,
							Status:  metav1.ConditionFalse,
							Message: FullyAppliedFailedMessage,
						}},
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Health: workv1alpha1.ResourceHealthy,
								Status: status,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:       names.GenerateWorkName(workload.GetKind(), workload.GetName(), workload.GetNamespace()),
						Namespace:  names.ExecutionSpacePrefix + "member2",
						Finalizers: []string{util.ExecutionControllerFinalizer},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: workloadJSON,
									},
								},
							},
						},
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{{
							Type:    workv1alpha1.WorkApplied,
							Status:  metav1.ConditionUnknown,
							Message: FullyAppliedFailedMessage,
						}},
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Health: workv1alpha1.ResourceUnhealthy,
								Status: status,
							},
						},
					},
				},
			},
			want: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName:    "member1",
					Status:         status,
					Applied:        false,
					AppliedMessage: FullyAppliedFailedMessage,
					Health:         workv1alpha2.ResourceHealthy,
				},
				{
					ClusterName:    "member2",
					Status:         status,
					Applied:        false,
					AppliedMessage: FullyAppliedFailedMessage,
					Health:         workv1alpha2.ResourceUnhealthy,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range tt.works {
				identifier, _ := BuildStatusIdentifier(&tt.works[i], workload)
				tt.works[i].Status.ManifestStatuses[0].Identifier = *identifier
			}
			got, err := assembleWorkStatus(tt.works, workload)
			if (err != nil) != tt.wantErr {
				t.Errorf("assembleWorkStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("assembleWorkStatus() got = %v, want %v", got, tt.want)
			}
		})
	}
}
