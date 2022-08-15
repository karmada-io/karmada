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
) []workv1alpha2.GracefulEvictionTask {
	var keptTasks []workv1alpha2.GracefulEvictionTask

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
		}
	}
	return keptTasks
}

func assessSingleTask(task workv1alpha2.GracefulEvictionTask, opt assessmentOption) *workv1alpha2.GracefulEvictionTask {
	// TODO(): gradually evict replica as per observed status.

	// task exceeds timeout
	if metav1.Now().After(task.CreationTimestamp.Add(opt.timeout)) {
		return nil
	}

	return &task
}

func nextRetry(tasks []workv1alpha2.GracefulEvictionTask, timeout time.Duration, timeNow time.Time) time.Duration {
	if len(tasks) == 0 {
		return 0
	}

	retryInterval := timeout / 10

	for i := range tasks {
		next := tasks[i].CreationTimestamp.Add(timeout).Sub(timeNow)
		if next < retryInterval {
			retryInterval = next
		}
	}

	return retryInterval
}
