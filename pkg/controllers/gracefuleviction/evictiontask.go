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
	"math"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

type assessmentOption struct {
	timeout        time.Duration
	scheduleResult []workv1alpha2.TargetCluster
	observedStatus []workv1alpha2.AggregatedStatusItem
	hasScheduled   bool
}

// assessEvictionTasks assesses each task according to graceful eviction rules and
// returns the tasks that should be kept.
// The now time is used as the input parameter to facilitate the unit test.
func assessEvictionTasks(tasks []workv1alpha2.GracefulEvictionTask, now metav1.Time, opt assessmentOption) ([]workv1alpha2.GracefulEvictionTask, []string) {
	var keptTasks []workv1alpha2.GracefulEvictionTask
	var evictedClusters []string

	for _, task := range tasks {
		// set creation timestamp for new task
		if task.CreationTimestamp.IsZero() {
			task.CreationTimestamp = &now
			keptTasks = append(keptTasks, task)
			continue
		}

		if task.GracePeriodSeconds != nil {
			opt.timeout = time.Duration(*task.GracePeriodSeconds) * time.Second
		}

		// assess task according to observed status
		kt := assessSingleTask(task, opt)
		if kt != nil {
			keptTasks = append(keptTasks, *kt)
		} else {
			evictedClusters = append(evictedClusters, task.FromCluster)
		}
	}
	return keptTasks, evictedClusters
}

func assessSingleTask(task workv1alpha2.GracefulEvictionTask, opt assessmentOption) *workv1alpha2.GracefulEvictionTask {
	if task.SuppressDeletion != nil {
		if *task.SuppressDeletion {
			return &task
		}
		// If *task.SuppressDeletion is equal to false,
		// it means users have confirmed that they want to delete the redundant copy.
		// In that case, we will delete the task immediately.
		return nil
	}

	// task exceeds timeout
	if metav1.Now().After(task.CreationTimestamp.Add(opt.timeout)) {
		return nil
	}

	// Only when the binding object has been scheduled can further judgment be made.
	// Otherwise, the binding status may be the old, which will affect the correctness of the judgment.
	if opt.hasScheduled && allScheduledResourceInHealthyState(opt) {
		return nil
	}

	return &task
}

func allScheduledResourceInHealthyState(opt assessmentOption) bool {
	for _, targetCluster := range opt.scheduleResult {
		var statusItem *workv1alpha2.AggregatedStatusItem

		// find the observed status of targetCluster
		for index, aggregatedStatus := range opt.observedStatus {
			if aggregatedStatus.ClusterName == targetCluster.Name {
				statusItem = &opt.observedStatus[index]
				break
			}
		}

		// no observed status found, maybe the resource hasn't been applied
		if statusItem == nil {
			return false
		}

		// resource not in healthy state
		if statusItem.Health != workv1alpha2.ResourceHealthy {
			return false
		}
	}

	return true
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
