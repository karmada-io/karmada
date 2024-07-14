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

package status

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

type clusterData struct {
	// readyCondition is the last observed ready condition of the cluster.
	readyCondition metav1.ConditionStatus
	// thresholdStartTime is the time that the ready condition changed.
	thresholdStartTime time.Time
}

type clusterConditionStore struct {
	clusterDataMap sync.Map
	// successThreshold is the duration of successes for the cluster to be considered healthy after recovery.
	successThreshold time.Duration
	// failureThreshold is the duration of failure for the cluster to be considered unhealthy.
	failureThreshold time.Duration
}

func (c *clusterConditionStore) thresholdAdjustedReadyCondition(cluster *clusterv1alpha1.Cluster, observedReadyCondition *metav1.Condition) *metav1.Condition {
	saved := c.get(cluster.Name)
	if saved == nil {
		// the cluster is just joined
		c.update(cluster.Name, &clusterData{
			readyCondition: observedReadyCondition.Status,
		})
		return observedReadyCondition
	}
	curReadyCondition := meta.FindStatusCondition(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady)
	if curReadyCondition == nil {
		return observedReadyCondition
	}

	now := time.Now()
	if saved.readyCondition != observedReadyCondition.Status {
		// ready condition status changed, record the threshold start time
		saved = &clusterData{
			readyCondition:     observedReadyCondition.Status,
			thresholdStartTime: now,
		}
		c.update(cluster.Name, saved)
	}

	var threshold time.Duration
	if observedReadyCondition.Status == metav1.ConditionTrue {
		threshold = c.successThreshold
	} else {
		threshold = c.failureThreshold
	}

	// we only care about true/not true
	// for unknown->false, just return the observed ready condition
	if ((observedReadyCondition.Status == metav1.ConditionTrue && curReadyCondition.Status != metav1.ConditionTrue) ||
		(observedReadyCondition.Status != metav1.ConditionTrue && curReadyCondition.Status == metav1.ConditionTrue)) &&
		now.Before(saved.thresholdStartTime.Add(threshold)) {
		// retain old status until threshold exceeded to avoid network unstable problems.
		return curReadyCondition
	}
	return observedReadyCondition
}

func (c *clusterConditionStore) get(cluster string) *clusterData {
	condition, ok := c.clusterDataMap.Load(cluster)
	if !ok {
		return nil
	}
	return condition.(*clusterData)
}

func (c *clusterConditionStore) update(cluster string, data *clusterData) {
	// ready condition status changed, record the threshold start time
	c.clusterDataMap.Store(cluster, data)
}

func (c *clusterConditionStore) delete(cluster string) {
	c.clusterDataMap.Delete(cluster)
}
