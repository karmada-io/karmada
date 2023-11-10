package gracefuleviction

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

type assessmentOption struct {
	scheduleResult []workv1alpha2.TargetCluster
	observedStatus []workv1alpha2.AggregatedStatusItem
}

// assessEvictionTasks assesses each task according to graceful eviction rules and
// returns the tasks that should be kept.
func assessEvictionTasks(bindingSpec workv1alpha2.ResourceBindingSpec,
	observedStatus []workv1alpha2.AggregatedStatusItem,
	now metav1.Time) ([]workv1alpha2.EvictionTask, []string) {
	var keptTasks []workv1alpha2.EvictionTask
	var evictedClusters []string

	for _, task := range bindingSpec.EvictionTasks {
		// set creation timestamp for new task
		if task.CreationTimestamp.IsZero() {
			task.CreationTimestamp = now
			keptTasks = append(keptTasks, task)
			continue
		}

		// assess task according to observed status
		kt := assessSingleTask(task, assessmentOption{
			scheduleResult: bindingSpec.Clusters,
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

func assessSingleTask(task workv1alpha2.EvictionTask, opt assessmentOption) *workv1alpha2.EvictionTask {
	if task.SuppressDeletion != nil {
		if *task.SuppressDeletion {
			return &task
		}
		// If *task.SuppressDeletion is equal to false,
		// it means users have confirmed that they want to delete the redundant copy.
		// In that case, we will delete the task immediately.
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
