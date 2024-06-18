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

package gracefuleviction

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

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
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					timeout: timeout,
					scheduleResult: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
				},
			},
			want: &workv1alpha2.GracefulEvictionTask{
				FromCluster:       "member1",
				CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
			},
		},
		{
			name: "task that exceeds the timeout",
			args: args{
				task: workv1alpha2.GracefulEvictionTask{
					FromCluster:       "member1",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -4)},
				},
				opt: assessmentOption{
					timeout: timeout,
					scheduleResult: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
				},
			},
			want: nil,
		},
		{
			name: "binding scheduled result is healthy, task should be nil",
			args: args{
				task: workv1alpha2.GracefulEvictionTask{
					FromCluster:       "member1",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					timeout: timeout,
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
				task: workv1alpha2.GracefulEvictionTask{
					FromCluster:       "member1",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					timeout: timeout,
					scheduleResult: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					observedStatus: []workv1alpha2.AggregatedStatusItem{
						{ClusterName: "memberA", Health: workv1alpha2.ResourceUnhealthy},
					},
				},
			},
			want: &workv1alpha2.GracefulEvictionTask{
				FromCluster:       "member1",
				CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
			},
		},
		{
			name: "binding scheduled result is unknown, task has no effect",
			args: args{
				task: workv1alpha2.GracefulEvictionTask{
					FromCluster:       "member1",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					timeout: timeout,
					scheduleResult: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					observedStatus: []workv1alpha2.AggregatedStatusItem{
						{ClusterName: "memberA", Health: workv1alpha2.ResourceUnknown},
					},
				},
			},
			want: &workv1alpha2.GracefulEvictionTask{
				FromCluster:       "member1",
				CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
			},
		},
		{
			name: "gracePeriodSeconds is declared in gracefulEvictionTask and timeout is not reached",
			args: args{
				task: workv1alpha2.GracefulEvictionTask{
					FromCluster:        "member1",
					GracePeriodSeconds: ptr.To[int32](30),
					CreationTimestamp:  &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					timeout: timeout,
					scheduleResult: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					observedStatus: []workv1alpha2.AggregatedStatusItem{
						{ClusterName: "memberA", Health: workv1alpha2.ResourceUnknown},
					},
				},
			},
			want: nil,
		},
		{
			name: "gracePeriodSeconds is declared in gracefulEvictionTask and timeout is reached",
			args: args{
				task: workv1alpha2.GracefulEvictionTask{
					FromCluster:        "member1",
					GracePeriodSeconds: ptr.To[int32](120),
					CreationTimestamp:  &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					timeout: timeout,
					scheduleResult: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					observedStatus: []workv1alpha2.AggregatedStatusItem{
						{ClusterName: "memberA", Health: workv1alpha2.ResourceUnknown},
					},
				},
			},
			want: &workv1alpha2.GracefulEvictionTask{
				FromCluster:        "member1",
				GracePeriodSeconds: ptr.To[int32](120),
				CreationTimestamp:  &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
			},
		},
		{
			name: "suppressDeletion is declared in gracefulEvictionTask and is true",
			args: args{
				task: workv1alpha2.GracefulEvictionTask{
					FromCluster:       "member1",
					SuppressDeletion:  ptr.To[bool](true),
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					timeout: timeout,
					scheduleResult: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					observedStatus: []workv1alpha2.AggregatedStatusItem{
						{ClusterName: "memberA", Health: workv1alpha2.ResourceHealthy},
					},
				},
			},
			want: &workv1alpha2.GracefulEvictionTask{
				FromCluster:       "member1",
				SuppressDeletion:  ptr.To[bool](true),
				CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
			},
		},
		{
			name: "suppressDeletion is declared in gracefulEvictionTask and is false",
			args: args{
				task: workv1alpha2.GracefulEvictionTask{
					FromCluster:       "member1",
					SuppressDeletion:  ptr.To[bool](false),
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				opt: assessmentOption{
					timeout: timeout,
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
	timeout := time.Minute * 3

	type args struct {
		bindingSpec    workv1alpha2.ResourceBindingSpec
		observedStatus []workv1alpha2.AggregatedStatusItem
		timeout        time.Duration
		now            metav1.Time
	}
	tests := []struct {
		name        string
		args        args
		wantTask    []workv1alpha2.GracefulEvictionTask
		wantCluster []string
	}{
		{
			name: "tasks without creation timestamp",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{FromCluster: "member1"},
						{FromCluster: "member2"},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{},
				timeout:        timeout,
				now:            timeNow,
			},
			wantTask: []workv1alpha2.GracefulEvictionTask{
				{
					FromCluster:       "member1",
					CreationTimestamp: &timeNow,
				},
				{
					FromCluster:       "member2",
					CreationTimestamp: &timeNow,
				},
			},
			wantCluster: nil,
		},
		{
			name: "tasks that do not exceed the timeout should do nothing",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "member1",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
						},
						{
							FromCluster:       "member2",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -2)},
						},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{},
				timeout:        timeout,
				now:            timeNow,
			},
			wantTask: []workv1alpha2.GracefulEvictionTask{
				{
					FromCluster:       "member1",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				{
					FromCluster:       "member2",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -2)},
				},
			},
			wantCluster: nil,
		},
		{
			name: "tasks that exceed the timeout should be removed",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
					},
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "member1",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -4)},
						},
						{
							FromCluster:       "member2",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -5)},
						},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{},
				timeout:        timeout,
				now:            timeNow,
			},
			wantTask:    nil,
			wantCluster: []string{"member1", "member2"},
		},
		{
			name: "mixed tasks",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
						{Name: "memberB"},
					},
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster: "member1",
						},
						{
							FromCluster:       "member2",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -2)},
						},
						{
							FromCluster:       "member3",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -4)},
						},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{},
				timeout:        timeout,
				now:            timeNow,
			},
			wantTask: []workv1alpha2.GracefulEvictionTask{
				{
					FromCluster:       "member1",
					CreationTimestamp: &timeNow,
				},
				{
					FromCluster:       "member2",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -2)},
				},
			},
			wantCluster: []string{"member3"},
		},
		{
			name: "tasks that do not exceed the timeout and someone binding scheduled result is missing, should do nothing",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
						{Name: "memberB"},
					},
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "member1",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
						},
						{
							FromCluster:       "member2",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -2)},
						},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "memberA", Health: workv1alpha2.ResourceHealthy},
				},
				timeout: timeout,
				now:     timeNow,
			},
			wantTask: []workv1alpha2.GracefulEvictionTask{
				{
					FromCluster:       "member1",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				{
					FromCluster:       "member2",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -2)},
				},
			},
			wantCluster: nil,
		},
		{
			name: "tasks that do not exceed the timeout and binding scheduled result is healthy, tasks need to be removed",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
						{Name: "memberB"},
					},
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "member1",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
						},
						{
							FromCluster:       "member2",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -2)},
						},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "memberA", Health: workv1alpha2.ResourceHealthy},
					{ClusterName: "memberB", Health: workv1alpha2.ResourceHealthy},
				},
				timeout: timeout,
				now:     timeNow,
			},
			wantTask:    nil,
			wantCluster: []string{"member1", "member2"},
		},
		{
			name: "tasks that do not exceed the timeout and someone binding scheduled result is unhealthy, should do nothing",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
						{Name: "memberB"},
					},
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "member1",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
						},
						{
							FromCluster:       "member2",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -2)},
						},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "memberA", Health: workv1alpha2.ResourceHealthy},
					{ClusterName: "memberB", Health: workv1alpha2.ResourceUnhealthy},
				},
				timeout: timeout,
				now:     timeNow,
			},
			wantTask: []workv1alpha2.GracefulEvictionTask{
				{
					FromCluster:       "member1",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				{
					FromCluster:       "member2",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -2)},
				},
			},
			wantCluster: nil,
		},
		{
			name: "tasks that do not exceed the timeout and someone binding scheduled result is unknown, should do nothing",
			args: args{
				bindingSpec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "memberA"},
						{Name: "memberB"},
					},
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "member1",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
						},
						{
							FromCluster:       "member2",
							CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -2)},
						},
					},
				},
				observedStatus: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "memberA", Health: workv1alpha2.ResourceHealthy},
					{ClusterName: "memberB", Health: workv1alpha2.ResourceUnknown},
				},
				timeout: timeout,
				now:     timeNow,
			},
			wantTask: []workv1alpha2.GracefulEvictionTask{
				{
					FromCluster:       "member1",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -1)},
				},
				{
					FromCluster:       "member2",
					CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -2)},
				},
			},
			wantCluster: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotTask, gotCluster := assessEvictionTasks(tt.args.bindingSpec, tt.args.observedStatus, tt.args.timeout, tt.args.now); !reflect.DeepEqual(gotTask, tt.wantTask) || !reflect.DeepEqual(gotCluster, tt.wantCluster) {
				t.Errorf("assessEvictionTasks() = (%v, %v), want (%v, %v)", gotTask, gotCluster, tt.wantTask, tt.wantCluster)
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
						CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -19)},
					},
					{
						FromCluster:       "member2",
						CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -10)},
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
						CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -10)},
					},
					{
						FromCluster:       "member2",
						CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -5)},
					},
				},
				timeout: timeout,
				timeNow: timeNow.Time,
			},
			want: time.Minute * 10,
		},
		{
			name: "suppression and graciously tasks co-exist",
			args: args{
				task: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster:       "member1",
						CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -60)},
						SuppressDeletion:  ptr.To[bool](true),
					},
					{
						FromCluster:       "member2",
						CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -5)},
					},
				},
				timeout: timeout,
				timeNow: timeNow.Time,
			},
			want: time.Minute * 15,
		},
		{
			name: "only suppression tasks",
			args: args{
				task: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster:       "member1",
						CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -60)},
						SuppressDeletion:  ptr.To[bool](true),
					},
					{
						FromCluster:       "member2",
						CreationTimestamp: &metav1.Time{Time: timeNow.Add(time.Minute * -5)},
						SuppressDeletion:  ptr.To[bool](true),
					},
				},
				timeout: timeout,
				timeNow: timeNow.Time,
			},
			want: 0,
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
