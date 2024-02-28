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

package store

import (
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type detectCondition struct {
	// condition is the last observed condition of the detection.
	condition metav1.Condition
	// thresholdStartTime is the time that the condition changed.
	thresholdStartTime time.Time
}

// ConditionCache caches observed conditions until threshold.
type ConditionCache struct {
	conditionMap sync.Map
	// successThreshold is the duration of successes for the condition to be considered healthy after recovery.
	successThreshold time.Duration
	// failureThreshold is the duration of failure for the condition to be considered unhealthy.
	failureThreshold time.Duration
}

// NewConditionCache returns a cache for condition.
func NewConditionCache(successThreshold, failureThreshold time.Duration) ConditionCache {
	return ConditionCache{
		conditionMap:     sync.Map{},
		successThreshold: successThreshold,
		failureThreshold: failureThreshold,
	}
}

// ThresholdAdjustedCondition adjusts conditions according to the threshold.
func (c *ConditionCache) ThresholdAdjustedCondition(key string, curr, observed *metav1.Condition) *metav1.Condition {
	saved := c.get(key)
	if saved == nil {
		// first detect
		c.update(key, &detectCondition{
			condition:          *observed.DeepCopy(),
			thresholdStartTime: time.Now(),
		})
		return observed
	}
	if curr == nil {
		return observed
	}

	now := time.Now()
	if saved.condition.Status != observed.Status {
		// condition status changed, record the threshold start time
		saved = &detectCondition{
			condition:          *observed.DeepCopy(),
			thresholdStartTime: now,
		}
		c.update(key, saved)
	}

	var threshold time.Duration
	if observed.Status == metav1.ConditionTrue {
		threshold = c.successThreshold
	} else {
		threshold = c.failureThreshold
	}

	// we only care about true/not true
	// for unknown->false, just return the observed ready condition
	if ((observed.Status == metav1.ConditionTrue && curr.Status != metav1.ConditionTrue) ||
		(observed.Status != metav1.ConditionTrue && curr.Status == metav1.ConditionTrue)) &&
		now.Before(saved.thresholdStartTime.Add(threshold)) {
		// retain old status until threshold exceeded.
		return curr
	}
	return observed
}

func (c *ConditionCache) get(key string) *detectCondition {
	condition, ok := c.conditionMap.Load(key)
	if !ok {
		return nil
	}
	return condition.(*detectCondition)
}

func (c *ConditionCache) update(key string, data *detectCondition) {
	c.conditionMap.Store(key, data)
}
