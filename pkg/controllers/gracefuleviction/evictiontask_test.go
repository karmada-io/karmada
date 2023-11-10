package gracefuleviction

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func Test_assessSingleTask(t *testing.T) {
	timeNow := metav1.Now()

	type args struct {
		task workv1alpha2.EvictionTask
		opt  assessmentOption
	}
	tests := []struct {
		name string
		args args
		want *workv1alpha2.EvictionTask
	}{
		{
			name: "binding scheduled result is healthy, task should be nil",
			args: args{
				task: workv1alpha2.EvictionTask{
					FromCluster:       "member1",
					CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					scheduleResult: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					observedStatus: []workv1alpha2.AggregatedStatusItem{
						{ClusterName: "memberA", Health: workv1alpha2.ResourceHealthy},
					},
				},
			},
			want: nil,
		},
		{
			name: "binding scheduled result is unhealthy, task has no effect",
			args: args{
				task: workv1alpha2.EvictionTask{
					FromCluster:       "member1",
					CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					scheduleResult: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					observedStatus: []workv1alpha2.AggregatedStatusItem{
						{ClusterName: "memberA", Health: workv1alpha2.ResourceUnhealthy},
					},
				},
			},
			want: &workv1alpha2.EvictionTask{
				FromCluster:       "member1",
				CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -1)},
			},
		},
		{
			name: "binding scheduled result is unknown, task has no effect",
			args: args{
				task: workv1alpha2.EvictionTask{
					FromCluster:       "member1",
					CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					scheduleResult: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					observedStatus: []workv1alpha2.AggregatedStatusItem{
						{ClusterName: "memberA", Health: workv1alpha2.ResourceUnknown},
					},
				},
			},
			want: &workv1alpha2.EvictionTask{
				FromCluster:       "member1",
				CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -1)},
			},
		},
		{
			name: "suppressDeletion is declared in gracefulEvictionTask and is true",
			args: args{
				task: workv1alpha2.EvictionTask{
					FromCluster:       "member1",
					SuppressDeletion:  pointer.Bool(true),
					CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					scheduleResult: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					observedStatus: []workv1alpha2.AggregatedStatusItem{
						{ClusterName: "memberA", Health: workv1alpha2.ResourceHealthy},
					},
				},
			},
			want: &workv1alpha2.EvictionTask{
				FromCluster:       "member1",
				SuppressDeletion:  pointer.Bool(true),
				CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -1)},
			},
		},
		{
			name: "suppressDeletion is declared in gracefulEvictionTask and is false",
			args: args{
				task: workv1alpha2.EvictionTask{
					FromCluster:       "member1",
					SuppressDeletion:  pointer.Bool(false),
					CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					scheduleResult: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					observedStatus: []workv1alpha2.AggregatedStatusItem{
						{ClusterName: "memberA", Health: workv1alpha2.ResourceHealthy},
					},
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := assessSingleTask(tt.args.task, tt.args.opt); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("assessSingleTask() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_assessEvictionTasks(t *testing.T) {
	timeNow := metav1.Now()

	type args struct {
		bindingSpec    workv1alpha2.ResourceBindingSpec
		observedStatus []workv1alpha2.AggregatedStatusItem
		now            metav1.Time
	}
	tests := []struct {
		name        string
		args        args
		wantTask    []workv1alpha2.EvictionTask
		wantCluster []string
	}{
		{
			name: "tasks without creation timestamp",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					EvictionTasks: []workv1alpha2.EvictionTask{
						{FromCluster: "member1"},
						{FromCluster: "member2"},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{},
				now:            timeNow,
			},
			wantTask: []workv1alpha2.EvictionTask{
				{
					FromCluster:       "member1",
					CreationTimestamp: timeNow,
				},
				{
					FromCluster:       "member2",
					CreationTimestamp: timeNow,
				},
			},
			wantCluster: nil,
		},
		{
			name: "evication task is done",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					EvictionTasks: []workv1alpha2.EvictionTask{
						{FromCluster: "member1", CreationTimestamp: timeNow},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "memberA", Health: workv1alpha2.ResourceHealthy},
				},
				now: timeNow,
			},
			wantTask:    nil,
			wantCluster: []string{"member1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotTask, gotCluster := assessEvictionTasks(tt.args.bindingSpec, tt.args.observedStatus, tt.args.now); !reflect.DeepEqual(gotTask, tt.wantTask) || !reflect.DeepEqual(gotCluster, tt.wantCluster) {
				t.Errorf("assessEvictionTasks() = (%v, %v), want (%v, %v)", gotTask, gotCluster, tt.wantTask, tt.wantCluster)
			}
		})
	}
}
