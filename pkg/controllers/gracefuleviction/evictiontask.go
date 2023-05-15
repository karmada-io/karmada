package gracefuleviction

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

type assessmentOption struct {
	timeout        time.Duration
	scheduleResult []workv1alpha2.TargetCluster
	observedStatus []workv1alpha2.AggregatedStatusItem
}

// assessEvictionTasks assesses each task according to graceful eviction rules and
// returns the tasks that should be kept.
func assessEvictionTasks(bindingSpec workv1alpha2.ResourceBindingSpec,
	observedStatus []workv1alpha2.AggregatedStatusItem,
	timeout time.Duration,
	now metav1.Time,
) ([]workv1alpha2.GracefulEvictionTask, []string) {
	var keptTasks []workv1alpha2.GracefulEvictionTask
	var evictedClusters []string

	for _, task := range bindingSpec.GracefulEvictionTasks {
		// set creation timestamp for new task
		if task.CreationTimestamp.IsZero() {
			task.CreationTimestamp = now
			keptTasks = append(keptTasks, task)
			continue
		}

		// assess task according to observed status
		kt := assessSingleTask(task, assessmentOption{
			scheduleResult: bindingSpec.Clusters,
			timeout:        timeout,
			observedStatus: observedStatus,
		})
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

	timeout := opt.timeout
	if task.GracePeriodSeconds != nil {
		timeout = time.Duration(*task.GracePeriodSeconds) * time.Second
	}
	// task exceeds timeout
	if metav1.Now().After(task.CreationTimestamp.Add(timeout)) {
		return nil
	}

	if allScheduledResourceInHealthyState(opt) {
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

func nextRetry(tasks []workv1alpha2.GracefulEvictionTask, timeout time.Duration, timeNow time.Time) time.Duration {
	if len(tasks) == 0 {
		return 0
	}

	retryInterval := timeout / 10

	for i := range tasks {
		if tasks[i].SuppressDeletion != nil {
			if *tasks[i].SuppressDeletion {
				continue
			}
		}
		if tasks[i].GracePeriodSeconds != nil {
			timeout = time.Duration(*tasks[i].GracePeriodSeconds) * time.Second
		}
		next := tasks[i].CreationTimestamp.Add(timeout).Sub(timeNow)
		if next < retryInterval {
			retryInterval = next
		}
	}

	return retryInterval
}
