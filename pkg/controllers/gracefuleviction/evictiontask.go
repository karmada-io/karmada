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
	"context"
	"math"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	plugins "github.com/karmada-io/karmada/pkg/controllers/gracefuleviction/evictplugins"
)

type assessmentOptionRB struct {
	binding   *workv1alpha2.ResourceBinding
	pluginMgr *plugins.Manager
	timeout   time.Duration
}

type assessmentOptionCRB struct {
	binding   *workv1alpha2.ClusterResourceBinding
	pluginMgr *plugins.Manager
	timeout   time.Duration
}

func assessEvictionTasksForRB(opt assessmentOptionRB, now metav1.Time) ([]workv1alpha2.GracefulEvictionTask, []string) {
	var keptTasks []workv1alpha2.GracefulEvictionTask
	evictedClusters := make([]string, 0)

	for _, task := range opt.binding.Spec.GracefulEvictionTasks {
		taskCopy := task.DeepCopy() // Work on a copy to avoid side effects.
		if taskCopy.CreationTimestamp.IsZero() {
			taskCopy.CreationTimestamp = &now
		}

		// Create a task-specific option copy to handle per-task timeout overrides.
		taskOpt := opt
		if taskCopy.GracePeriodSeconds != nil {
			taskOpt.timeout = time.Duration(*taskCopy.GracePeriodSeconds) * time.Second
		}

		if !isTaskFinishedForRB(context.TODO(), taskCopy, taskOpt) {
			keptTasks = append(keptTasks, *taskCopy)
		} else {
			evictedClusters = append(evictedClusters, taskCopy.FromCluster)
		}
	}
	return keptTasks, evictedClusters
}

func isTaskFinishedForRB(ctx context.Context, task *workv1alpha2.GracefulEvictionTask, opt assessmentOptionRB) bool {
	if task.SuppressDeletion != nil {
		return !*task.SuppressDeletion // If SuppressDeletion is true, keep the task. If false, finish it.
	}

	// Check for timeout.
	if metav1.Now().After(task.CreationTimestamp.Add(opt.timeout)) {
		return true
	}

	// Check with plugins only when the binding has been scheduled.
	hasScheduled := opt.binding.Status.SchedulerObservedGeneration == opt.binding.GetGeneration()
	if hasScheduled && opt.pluginMgr.CanBeCleanedRB(ctx, task, opt.binding) {
		return true
	}

	return false
}

func assessEvictionTasksForCRB(opt assessmentOptionCRB, now metav1.Time) ([]workv1alpha2.GracefulEvictionTask, []string) {
	var keptTasks []workv1alpha2.GracefulEvictionTask
	evictedClusters := make([]string, 0)

	for _, task := range opt.binding.Spec.GracefulEvictionTasks {
		taskCopy := task.DeepCopy()
		if taskCopy.CreationTimestamp.IsZero() {
			taskCopy.CreationTimestamp = &now
		}

		taskOpt := opt
		if taskCopy.GracePeriodSeconds != nil {
			taskOpt.timeout = time.Duration(*taskCopy.GracePeriodSeconds) * time.Second
		}

		if !isTaskFinishedForCRB(context.TODO(), taskCopy, taskOpt) {
			keptTasks = append(keptTasks, *taskCopy)
		} else {
			evictedClusters = append(evictedClusters, taskCopy.FromCluster)
		}
	}
	return keptTasks, evictedClusters
}

func isTaskFinishedForCRB(ctx context.Context, task *workv1alpha2.GracefulEvictionTask, opt assessmentOptionCRB) bool {
	if task.SuppressDeletion != nil {
		return !*task.SuppressDeletion
	}

	if metav1.Now().After(task.CreationTimestamp.Add(opt.timeout)) {
		return true
	}

	hasScheduled := opt.binding.Status.SchedulerObservedGeneration == opt.binding.GetGeneration()
	if hasScheduled && opt.pluginMgr.CanBeCleanedCRB(ctx, task, opt.binding) {
		return true
	}

	return false
}

func nextRetry(tasks []workv1alpha2.GracefulEvictionTask, gracefulTimeout time.Duration, timeNow time.Time) time.Duration {
	if len(tasks) == 0 {
		return 0
	}

	retryInterval := time.Duration(math.MaxInt64)

	// Skip tasks whose type is SuppressDeletion because they are manually controlled by users.
	// We currently take the minimum value of the timeout of all GracefulEvictionTasks besides above.
	// When the application on the new cluster becomes healthy, a new event will be queued
	// because the controller can watch the changes of binding status.
	for i := range tasks {
		if tasks[i].SuppressDeletion != nil {
			continue
		}
		timeout := gracefulTimeout
		if tasks[i].GracePeriodSeconds != nil {
			timeout = time.Duration(*tasks[i].GracePeriodSeconds) * time.Second
		}
		next := tasks[i].CreationTimestamp.Add(timeout).Sub(timeNow)
		if next < retryInterval {
			retryInterval = next
		}
	}

	// When there are only tasks whose type is SuppressDeletion, we do not need to retry.
	if retryInterval == time.Duration(math.MaxInt64) {
		retryInterval = 0
	}
	return retryInterval
}
