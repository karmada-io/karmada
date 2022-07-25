package helper

import (
	"encoding/json"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// ParsingJobStatus generates new status of given 'AggregatedStatusItem'.
//nolint:gocyclo
func ParsingJobStatus(obj *batchv1.Job, status []workv1alpha2.AggregatedStatusItem) (*batchv1.JobStatus, error) {
	var jobFailed []string
	var startTime, completionTime *metav1.Time
	successfulJobs, completionJobs := 0, 0
	newStatus := &batchv1.JobStatus{}
	for _, item := range status {
		if item.Status == nil {
			continue
		}
		temp := &batchv1.JobStatus{}
		if err := json.Unmarshal(item.Status.Raw, temp); err != nil {
			klog.Errorf("Failed to unmarshal status of job(%s/%s): %v", obj.Namespace, obj.Name, err)
			return nil, err
		}
		klog.V(3).Infof("Grab job(%s/%s) status from cluster(%s), active: %d, succeeded %d, failed: %d",
			obj.Namespace, obj.Name, item.ClusterName, temp.Active, temp.Succeeded, temp.Failed)

		newStatus.Active += temp.Active
		newStatus.Succeeded += temp.Succeeded
		newStatus.Failed += temp.Failed

		isFinished, finishedStatus := GetJobFinishedStatus(temp)
		if isFinished && finishedStatus == batchv1.JobComplete {
			successfulJobs++
		} else if isFinished && finishedStatus == batchv1.JobFailed {
			jobFailed = append(jobFailed, item.ClusterName)
		}

		// StartTime
		if startTime == nil || temp.StartTime.Before(startTime) {
			startTime = temp.StartTime
		}
		// CompletionTime
		if temp.CompletionTime != nil {
			completionJobs++
			if completionTime == nil || completionTime.Before(temp.CompletionTime) {
				completionTime = temp.CompletionTime
			}
		}
	}

	if len(jobFailed) != 0 {
		newStatus.Conditions = append(newStatus.Conditions, batchv1.JobCondition{
			Type:               batchv1.JobFailed,
			Status:             corev1.ConditionTrue,
			LastProbeTime:      metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "JobFailed",
			Message:            fmt.Sprintf("Job executed failed in member clusters %s", strings.Join(jobFailed, ",")),
		})
	}

	// aggregated status can be empty when the binding is just created
	// in which case we should not set the job status to complete
	if successfulJobs == len(status) && successfulJobs > 0 {
		newStatus.Conditions = append(newStatus.Conditions, batchv1.JobCondition{
			Type:               batchv1.JobComplete,
			Status:             corev1.ConditionTrue,
			LastProbeTime:      metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "Completed",
			Message:            "Job completed",
		})
	}

	if startTime != nil {
		newStatus.StartTime = startTime.DeepCopy()
	}
	if completionTime != nil && completionJobs == len(status) {
		newStatus.CompletionTime = completionTime.DeepCopy()
	}

	return newStatus, nil
}

// GetJobFinishedStatus checks whether the given Job has finished execution.
// It does not discriminate between successful and failed terminations.
func GetJobFinishedStatus(jobStatus *batchv1.JobStatus) (bool, batchv1.JobConditionType) {
	for _, c := range jobStatus.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true, c.Type
		}
	}
	return false, ""
}
