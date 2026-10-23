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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/karmada-io/karmada/pkg/util/helper"
)

// interpretJobHealth determines the health status of a batch/v1 Job.
//
// A Job is considered Healthy when:
//   - The Job has completed successfully (JobComplete condition == True).
//   - The Job is actively running (has active pods or succeeded pods and not failed).
//   - The Job has just been created and no progress signal is available yet.
//
// A Job is considered Unhealthy when:
//   - The Job has failed (JobFailed condition == True).
//   - The Job has been running for an extended period with zero progress
//     (no active, no succeeded, no failed pods and startTime exceeds
//     jobProgressTimeout) — this indicates pods cannot be scheduled.
//
// Note: Since InterpretHealth operates on the aggregated Job object
// (without access to individual Pod details), the scheduler-blocked
// detection is based on job-level signals rather than per-pod inspection.
func interpretJobHealth(object *unstructured.Unstructured) (bool, error) {
	job := &batchv1.Job{}
	err := helper.ConvertToTypedObject(object, job)
	if err != nil {
		return false, err
	}

	// Terminal success — always healthy.
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true, nil
		}
	}

	// Terminal failure — always unhealthy.
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return false, nil
		}
	}

	// Suspended jobs are not making progress; treat them as unhealthy
	// so that failover can take them over when the suspension ends
	// or so that operators notice the stalled state.
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobSuspended && c.Status == corev1.ConditionTrue {
			return false, nil
		}
	}

	// Active or succeeded pods indicate the Job is making progress.
	if job.Status.Active > 0 || job.Status.Succeeded > 0 {
		return true, nil
	}

	// No progress at all — check if the Job has been stuck for too long.
	// A newly created Job without startTime yet is treated as healthy.
	if job.Status.StartTime != nil {
		// Default to healthy if the Job has only been waiting for a short
		// time (autoscaler provisioning, image pull, etc.). The threshold
		// mirrors common Kubernetes grace periods.
		const jobStuckThreshold = 10 * time.Minute
		if time.Since(job.Status.StartTime.Time) > jobStuckThreshold {
			return false, nil
		}
	}

	return true, nil
}

// interpretCronJobHealth determines the health status of a batch/v1 CronJob.
//
// CronJobs are periodic controllers that spawn child Jobs. They are considered:
//   - Healthy when the CronJob has active child jobs, has a LastScheduleTime
//     (meaning the CronJob controller has successfully scheduled at least once),
//     or is a freshly created CronJob not yet due for its first run.
//   - Unhealthy when the CronJob is suspended (spec.suspend == True), which
//     means it will not schedule any child jobs until resumed.
func interpretCronJobHealth(object *unstructured.Unstructured) (bool, error) {
	cronJob := &batchv1.CronJob{}
	err := helper.ConvertToTypedObject(object, cronJob)
	if err != nil {
		return false, err
	}

	// A suspended CronJob will not produce child jobs.
	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		return false, nil
	}

	// If there are active child jobs or the CronJob has scheduled at least once,
	// the controller is functioning normally.
	if len(cronJob.Status.Active) > 0 || cronJob.Status.LastScheduleTime != nil {
		return true, nil
	}

	// Fresh CronJob with no schedule time yet — healthy by default.
	return true, nil
}
