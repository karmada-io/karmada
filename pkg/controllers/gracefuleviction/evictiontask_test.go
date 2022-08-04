package gracefuleviction

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func Test_assessSingleTask(t *testing.T) {
	timeNow := metav1.Now()
	timeout := time.Minute * 3

	type args struct {
		task workv1alpha2.GracefulEvictionTask
		opt  assessmentOption
	}
	tests := []struct {
		name string
		args args
		want *workv1alpha2.GracefulEvictionTask
	}{
		{
			name: "task that doesn't exceed the timeout",
			args: args{
				task: workv1alpha2.GracefulEvictionTask{
					FromCluster:       "member1",
					CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					timeout: timeout,
				},
			},
			want: &workv1alpha2.GracefulEvictionTask{
				FromCluster:       "member1",
				CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -1)},
			},
		},
		{
			name: "task that exceeds the timeout",
			args: args{
				task: workv1alpha2.GracefulEvictionTask{
					FromCluster:       "member1",
					CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -4)},
				},
				opt: assessmentOption{
					timeout: timeout,
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
	timeout := time.Minute * 3

	type args struct {
		bindingSpec    workv1alpha2.ResourceBindingSpec
		observedStatus []workv1alpha2.AggregatedStatusItem
		timeout        time.Duration
		now            metav1.Time
	}
	tests := []struct {
		name string
		args args
		want []workv1alpha2.GracefulEvictionTask
	}{
		{
			name: "tasks without creation timestamp",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{FromCluster: "member1"},
						{FromCluster: "member2"},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{},
				timeout:        timeout,
				now:            timeNow,
			},
			want: []workv1alpha2.GracefulEvictionTask{
				{
					FromCluster:       "member1",
					CreationTimestamp: timeNow,
				},
				{
					FromCluster:       "member2",
					CreationTimestamp: timeNow,
				},
			},
		},
		{
			name: "tasks that do not exceed the timeout should do nothing",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "member1",
							CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -1)},
						},
						{
							FromCluster:       "member2",
							CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -2)},
						},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{},
				timeout:        timeout,
				now:            timeNow,
			},
			want: []workv1alpha2.GracefulEvictionTask{
				{
					FromCluster:       "member1",
					CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				{
					FromCluster:       "member2",
					CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -2)},
				},
			},
		},
		{
			name: "tasks that exceed the timeout should be removed",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "member1",
							CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -4)},
						},
						{
							FromCluster:       "member2",
							CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -5)},
						},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{},
				timeout:        timeout,
				now:            timeNow,
			},
			want: nil,
		},
		{
			name: "mixed tasks",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster: "member1",
						},
						{
							FromCluster:       "member2",
							CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -2)},
						},
						{
							FromCluster:       "member3",
							CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -4)},
						},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{},
				timeout:        timeout,
				now:            timeNow,
			},
			want: []workv1alpha2.GracefulEvictionTask{
				{
					FromCluster:       "member1",
					CreationTimestamp: timeNow,
				},
				{
					FromCluster:       "member2",
					CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -2)},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := assessEvictionTasks(tt.args.bindingSpec, tt.args.observedStatus, tt.args.timeout, tt.args.now); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("assessEvictionTasks() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nextRetry(t *testing.T) {
	timeNow := metav1.Now()
	timeout := time.Minute * 20
	type args struct {
		task    []workv1alpha2.GracefulEvictionTask
		timeout time.Duration
		timeNow time.Time
	}
	tests := []struct {
		name string
		args args
		want time.Duration
	}{
		{
			name: "empty tasks",
			args: args{
				task:    []workv1alpha2.GracefulEvictionTask{},
				timeout: timeout,
				timeNow: timeNow.Time,
			},
			want: 0,
		},
		{
			name: "retry interval is less than timeout / 10",
			args: args{
				task: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster:       "member1",
						CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -19)},
					},
					{
						FromCluster:       "member2",
						CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -10)},
					},
				},
				timeout: timeout,
				timeNow: timeNow.Time,
			},
			want: time.Minute * 1,
		},
		{
			name: "retry interval is equal to timeout / 10",
			args: args{
				task: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster:       "member1",
						CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -10)},
					},
					{
						FromCluster:       "member2",
						CreationTimestamp: metav1.Time{Time: timeNow.Add(time.Minute * -5)},
					},
				},
				timeout: timeout,
				timeNow: timeNow.Time,
			},
			want: timeout / 10,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextRetry(tt.args.task, tt.args.timeout, tt.args.timeNow); got != tt.want {
				t.Errorf("nextRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}
