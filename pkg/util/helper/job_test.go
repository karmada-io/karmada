package helper

import (
	"reflect"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestGetJobFinishedStatus(t *testing.T) {
	tests := []struct {
		name                  string
		jobStatus             *batchv1.JobStatus
		expectedJobFinished   bool
		expectedConditionType batchv1.JobConditionType
	}{
		{
			name: "job complete",
			jobStatus: &batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}},
			},
			expectedJobFinished:   true,
			expectedConditionType: batchv1.JobComplete,
		},
		{
			name: "job failed",
			jobStatus: &batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}},
			},
			expectedJobFinished:   true,
			expectedConditionType: batchv1.JobFailed,
		},
		{
			name: "job suspended",
			jobStatus: &batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{{Type: batchv1.JobSuspended, Status: corev1.ConditionTrue}},
			},
			expectedJobFinished:   false,
			expectedConditionType: "",
		},
		{
			name: "job condition unknown",
			jobStatus: &batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionUnknown}},
			},
			expectedJobFinished:   false,
			expectedConditionType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, resType := GetJobFinishedStatus(tt.jobStatus)
			if res != tt.expectedJobFinished || resType != tt.expectedConditionType {
				t.Errorf("expected: %v and %v, but got: %v and %v", tt.expectedJobFinished, tt.expectedConditionType, res, resType)
			}
		})
	}
}

func TestParsingJobStatus(t *testing.T) {
	testTime := time.Now()
	testV1time := metav1.NewTime(testTime)
	statusMap := map[string]interface{}{
		"active":         0,
		"succeeded":      1,
		"startTime":      testV1time,
		"completionTime": testV1time,
		"failed":         0,
		"conditions":     []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}},
	}
	raw, _ := BuildStatusRawExtension(statusMap)
	statusMapWithJobfailed := map[string]interface{}{
		"active":         0,
		"succeeded":      0,
		"startTime":      testV1time,
		"completionTime": testV1time,
		"failed":         1,
		"conditions":     []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}},
	}
	rawJobFailed, _ := BuildStatusRawExtension(statusMapWithJobfailed)
	tests := []struct {
		name                  string
		job                   *batchv1.Job
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedJobStatus     *batchv1.JobStatus
	}{
		{
			name: "",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "memberA", Status: raw},
				{ClusterName: "memberB", Status: raw},
			},
			expectedJobStatus: &batchv1.JobStatus{
				Succeeded: 2,
				Conditions: []batchv1.JobCondition{
					{
						Type:               batchv1.JobComplete,
						Status:             corev1.ConditionTrue,
						LastProbeTime:      testV1time,
						LastTransitionTime: testV1time,
						Reason:             "Completed",
						Message:            "Job completed",
					},
				},
			},
		},
		{
			name: "",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "memberA", Status: rawJobFailed},
				{ClusterName: "memberB", Status: rawJobFailed},
			},
			expectedJobStatus: &batchv1.JobStatus{
				Failed: 2,
				Conditions: []batchv1.JobCondition{
					{
						Type:               batchv1.JobFailed,
						Status:             corev1.ConditionTrue,
						LastProbeTime:      testV1time,
						LastTransitionTime: testV1time,
						Reason:             "JobFailed",
						Message:            "Job executed failed in member clusters memberA,memberB",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ParsingJobStatus(tt.job, tt.aggregatedStatusItems)
			if err != nil {
				t.Fatalf("failed to parse job status, err: %v", err)
			}
			// Do not compare times to avoid time errors due to processor processing
			res.StartTime = nil
			res.CompletionTime = nil
			for i := range res.Conditions {
				res.Conditions[i].LastProbeTime = testV1time
				res.Conditions[i].LastTransitionTime = testV1time
			}
			if !reflect.DeepEqual(res, tt.expectedJobStatus) {
				t.Errorf("expected: %v, but got: %v", tt.expectedJobStatus, res)
			}
		})
	}
}
