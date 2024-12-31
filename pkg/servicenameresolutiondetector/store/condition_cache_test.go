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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewConditionCache(t *testing.T) {
	successThreshold := 5 * time.Minute
	failureThreshold := 3 * time.Minute

	cache := NewConditionCache(successThreshold, failureThreshold)

	assert.Equal(t, successThreshold, cache.successThreshold)
	assert.Equal(t, failureThreshold, cache.failureThreshold)
}

func TestConditionCache_ThresholdAdjustedCondition(t *testing.T) {
	testCases := []struct {
		name             string
		key              string
		curr             *metav1.Condition
		observed         *metav1.Condition
		existingCond     *detectCondition
		successThreshold time.Duration
		failureThreshold time.Duration
		expected         *metav1.Condition
	}{
		{
			name: "First detection",
			key:  "test1",
			curr: nil,
			observed: &metav1.Condition{
				Type:   "TestType",
				Status: metav1.ConditionTrue,
			},
			existingCond:     nil,
			successThreshold: 5 * time.Minute,
			failureThreshold: 3 * time.Minute,
			expected: &metav1.Condition{
				Type:   "TestType",
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "Status changed, within threshold",
			key:  "test2",
			curr: &metav1.Condition{
				Type:   "TestType",
				Status: metav1.ConditionTrue,
			},
			observed: &metav1.Condition{
				Type:   "TestType",
				Status: metav1.ConditionFalse,
			},
			existingCond: &detectCondition{
				condition: metav1.Condition{
					Type:   "TestType",
					Status: metav1.ConditionTrue,
				},
				thresholdStartTime: time.Now().Add(-1 * time.Minute),
			},
			successThreshold: 5 * time.Minute,
			failureThreshold: 3 * time.Minute,
			expected: &metav1.Condition{
				Type:   "TestType",
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "Status changed, threshold exceeded",
			key:  "test3",
			curr: &metav1.Condition{
				Type:   "TestType",
				Status: metav1.ConditionTrue,
			},
			observed: &metav1.Condition{
				Type:   "TestType",
				Status: metav1.ConditionFalse,
			},
			existingCond: &detectCondition{
				condition: metav1.Condition{
					Type:   "TestType",
					Status: metav1.ConditionFalse,
				},
				thresholdStartTime: time.Now().Add(-4 * time.Minute),
			},
			successThreshold: 5 * time.Minute,
			failureThreshold: 3 * time.Minute,
			expected: &metav1.Condition{
				Type:   "TestType",
				Status: metav1.ConditionFalse,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cache := NewConditionCache(tc.successThreshold, tc.failureThreshold)
			if tc.existingCond != nil {
				cache.update(tc.key, tc.existingCond)
			}

			result := cache.ThresholdAdjustedCondition(tc.key, tc.curr, tc.observed)

			assert.Equal(t, tc.expected.Type, result.Type)
			assert.Equal(t, tc.expected.Status, result.Status)
		})
	}
}

func TestConditionCache_GetAndUpdate(t *testing.T) {
	testCases := []struct {
		name           string
		key            string
		initialValue   *detectCondition
		updatedValue   *detectCondition
		expectedBefore *detectCondition
		expectedAfter  *detectCondition
	}{
		{
			name:           "Get non-existent key",
			key:            "testKey1",
			initialValue:   nil,
			updatedValue:   nil,
			expectedBefore: nil,
			expectedAfter:  nil,
		},
		{
			name: "Update and get existing key",
			key:  "testKey2",
			initialValue: &detectCondition{
				condition: metav1.Condition{
					Type:   "TestType",
					Status: metav1.ConditionTrue,
				},
				thresholdStartTime: time.Now(),
			},
			updatedValue: &detectCondition{
				condition: metav1.Condition{
					Type:   "TestType",
					Status: metav1.ConditionFalse,
				},
				thresholdStartTime: time.Now(),
			},
			expectedBefore: &detectCondition{
				condition: metav1.Condition{
					Type:   "TestType",
					Status: metav1.ConditionTrue,
				},
			},
			expectedAfter: &detectCondition{
				condition: metav1.Condition{
					Type:   "TestType",
					Status: metav1.ConditionFalse,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cache := NewConditionCache(5*time.Minute, 3*time.Minute)

			if tc.initialValue != nil {
				cache.update(tc.key, tc.initialValue)
			}

			result := cache.get(tc.key)
			if tc.expectedBefore == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tc.expectedBefore.condition.Type, result.condition.Type)
				assert.Equal(t, tc.expectedBefore.condition.Status, result.condition.Status)
			}

			if tc.updatedValue != nil {
				cache.update(tc.key, tc.updatedValue)
			}

			result = cache.get(tc.key)
			if tc.expectedAfter == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tc.expectedAfter.condition.Type, result.condition.Type)
				assert.Equal(t, tc.expectedAfter.condition.Status, result.condition.Status)
			}
		})
	}
}
