/*
Copyright 2025 The Karmada Authors.

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

package queue

import (
	"testing"
	"time"
)

func TestBackOffQueue(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func() bool
	}{
		{
			name: "Add b1, b2 and Get the binding with earlier expiry",
			testFunc: func() bool {
				now := time.Now()
				bTimestamp1 := now.Add(-10 * time.Second)
				bTimestamp2 := now.Add(-8 * time.Second)
				b1 := &QueuedBindingInfo{NamespacedKey: "foo1", Priority: 2, Attempts: 3, Timestamp: bTimestamp1}
				b2 := &QueuedBindingInfo{NamespacedKey: "foo2", Priority: 3, Attempts: 0, Timestamp: bTimestamp2}
				bq := NewSchedulingQueue()
				bq.PushBackoffIfNotPresent(b1)
				bq.PushBackoffIfNotPresent(b2)
				bInfo, ok := bq.(*prioritySchedulingQueue).backoffQ.Peek()
				if !ok || bInfo == nil {
					return false
				}
				return bInfo.NamespacedKey == b2.NamespacedKey
			},
		},
		{
			name: "Add b1, b2 and Get the binding with higher priority if expiry is same",
			testFunc: func() bool {
				now := time.Now()
				b1 := &QueuedBindingInfo{NamespacedKey: "foo1", Priority: 4, Attempts: 2, Timestamp: now}
				b2 := &QueuedBindingInfo{NamespacedKey: "foo2", Priority: 3, Attempts: 2, Timestamp: now}
				bq := NewSchedulingQueue()
				bq.PushBackoffIfNotPresent(b1)
				bq.PushBackoffIfNotPresent(b2)
				bInfo, ok := bq.(*prioritySchedulingQueue).backoffQ.Peek()
				if !ok || bInfo == nil {
					return false
				}
				return bInfo.NamespacedKey == b1.NamespacedKey
			},
		},
	}
	for _, tt := range tests {
		success := tt.testFunc()
		if !success {
			t.Fatalf(`%q should be success, but get failed`, tt.name)
		}
	}
}
