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

package native

import (
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/karmada-io/karmada/pkg/util/helper"
)

func toUnstructured(t *testing.T, obj any) *unstructured.Unstructured {
	t.Helper()
	u, err := helper.ToUnstructured(obj)
	if err != nil {
		t.Fatalf("ToUnstructured failed: %v", err)
	}
	return u
}

func TestInterpretJobHealth(t *testing.T) {
	now := time.Now()
	past := now.Add(-15 * time.Minute)  // 15 min ago — exceeds 10 min threshold
	recent := now.Add(-2 * time.Minute) // 2 min ago — within threshold

	tests := []struct {
		name    string
		job     *batchv1.Job
		want    bool
		wantErr bool
	}{
		// --- Healthy cases ---
		{
			name: "Job with JobComplete condition is healthy",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "completed-job",
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Job with active pods is healthy",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "running-job",
				},
				Status: batchv1.JobStatus{
					Active: 3,
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Job with succeeded pods (still running more) is healthy",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "partially-success-job",
				},
				Status: batchv1.JobStatus{
					Succeeded: 2,
					Active:    1,
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "New job with no startTime yet is healthy",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fresh-job",
				},
				Status: batchv1.JobStatus{},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Job recently started with no active pods yet is healthy (within threshold)",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "just-started-job",
				},
				Status: batchv1.JobStatus{
					StartTime: &metav1.Time{Time: recent},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Job with JobFailed condition False and some progress is healthy",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "running-with-conditions-job",
				},
				Status: batchv1.JobStatus{
					Active: 2,
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},

		// --- Unhealthy cases ---
		{
			name: "Job with JobFailed condition is unhealthy",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "failed-job",
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "Suspended Job is unhealthy",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "suspended-job",
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobSuspended,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "Job stuck with no active pods for over 10 minutes is unhealthy",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "stuck-job",
				},
				Status: batchv1.JobStatus{
					StartTime: &metav1.Time{Time: past},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "Job with Failed condition takes precedence over partial progress",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "failed-after-some-attempts-job",
				},
				Status: batchv1.JobStatus{
					Failed: 3,
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "JobComplete takes precedence over JobFailed (should never happen but safe)",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "complete-and-failed-job",
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want:    true, // JobComplete checked first
			wantErr: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u := toUnstructured(t, tt.job)
			got, err := interpretJobHealth(u)
			if (err != nil) != tt.wantErr {
				t.Errorf("interpretJobHealth() err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("interpretJobHealth() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInterpretCronJobHealth(t *testing.T) {
	trueVal := true
	now := time.Now()

	tests := []struct {
		name    string
		cronJob *batchv1.CronJob
		want    bool
		wantErr bool
	}{
		{
			name: "CronJob with active child jobs is healthy",
			cronJob: &batchv1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "active-cronjob",
				},
				Spec: batchv1.CronJobSpec{},
				Status: batchv1.CronJobStatus{
					Active: []corev1.ObjectReference{{Name: "child-job-1"}},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "CronJob with LastScheduleTime is healthy",
			cronJob: &batchv1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "scheduled-cronjob",
				},
				Spec: batchv1.CronJobSpec{},
				Status: batchv1.CronJobStatus{
					LastScheduleTime: &metav1.Time{Time: now},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "New CronJob with no schedule yet is healthy",
			cronJob: &batchv1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fresh-cronjob",
				},
				Spec:   batchv1.CronJobSpec{},
				Status: batchv1.CronJobStatus{},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Suspended CronJob is unhealthy",
			cronJob: &batchv1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name: "suspended-cronjob",
				},
				Spec: batchv1.CronJobSpec{
					Suspend: &trueVal,
				},
				Status: batchv1.CronJobStatus{
					Active: []corev1.ObjectReference{{Name: "old-child"}},
				},
			},
			want:    false,
			wantErr: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u := toUnstructured(t, tt.cronJob)
			got, err := interpretCronJobHealth(u)
			if (err != nil) != tt.wantErr {
				t.Errorf("interpretCronJobHealth() err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("interpretCronJobHealth() got = %v, want %v", got, tt.want)
			}
		})
	}
}
